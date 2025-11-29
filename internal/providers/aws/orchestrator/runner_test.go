package orchestrator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/constants"
	awsConstants "github.com/runvoy/runvoy/internal/providers/aws/constants"

	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSidecarContainerCommandWithoutGitRepo(t *testing.T) {
	cmd := buildSidecarContainerCommand(false, map[string]string{}, []string{})

	require.Len(t, cmd, 3, "expected shell command with interpreter and script")
	assert.Equal(t, "/bin/sh", cmd[0])
	assert.Equal(t, "-c", cmd[1])

	script := cmd[2]

	assert.Contains(t, script, constants.ProjectName+" sidecar: No RUNVOY_USER_* variables found, skipping .env creation")
	assert.Contains(t, script, constants.ProjectName+" sidecar: No git repository specified, exiting")
	assert.NotContains(t, script, "git clone", "git repo commands must be skipped when not requested")
	assert.Contains(t, script, "set -e", "script should enable exit on error")
}

func TestBuildSidecarContainerCommandWithGitRepo(t *testing.T) {
	cmd := buildSidecarContainerCommand(true, map[string]string{}, []string{})

	require.Len(t, cmd, 3)
	script := cmd[2]

	assert.Contains(t, script, "apk add --no-cache git", "should install git when repo cloning is requested")
	assert.Contains(t, script, "git clone --depth 1 --branch \"${GIT_REF}\" \"${GIT_REPO}\" \"${CLONE_PATH}\"")
	assert.Contains(t, script, "cp \"${RUNVOY_SHARED_VOLUME_PATH}/.env\" \"${CLONE_PATH}/.env\"")
	assert.Contains(t, script, constants.ProjectName+" sidecar: .env file copied to repo directory")
}

func TestInjectGitHubTokenIfNeeded(t *testing.T) {
	tests := []struct {
		name     string
		gitRepo  string
		userEnv  map[string]string
		expected string
	}{
		{
			name:     "GitHub URL with token",
			gitRepo:  "https://github.com/owner/repo.git",
			userEnv:  map[string]string{"GITHUB_TOKEN": "ghp_token123"},
			expected: "https://ghp_token123@github.com/owner/repo.git",
		},
		{
			name:     "GitHub URL without token",
			gitRepo:  "https://github.com/owner/repo.git",
			userEnv:  map[string]string{},
			expected: "https://github.com/owner/repo.git",
		},
		{
			name:     "GitHub URL with empty token",
			gitRepo:  "https://github.com/owner/repo.git",
			userEnv:  map[string]string{"GITHUB_TOKEN": ""},
			expected: "https://github.com/owner/repo.git",
		},
		{
			name:     "Non-GitHub URL with token",
			gitRepo:  "https://gitlab.com/owner/repo.git",
			userEnv:  map[string]string{"GITHUB_TOKEN": "ghp_token123"},
			expected: "https://gitlab.com/owner/repo.git",
		},
		{
			name:     "GitHub URL with other env vars but no token",
			gitRepo:  "https://github.com/owner/repo.git",
			userEnv:  map[string]string{"OTHER_VAR": "value"},
			expected: "https://github.com/owner/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := injectGitHubTokenIfNeeded(tt.gitRepo, tt.userEnv)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildMainContainerCommandWithoutRepo(t *testing.T) {
	req := &api.ExecutionRequest{
		Command: "echo 'hello world'",
	}

	cmd := buildMainContainerCommand(req, "request-123", "ubuntu:22.04", nil)

	require.Len(t, cmd, 3)
	commandScript := cmd[2]

	assert.Contains(t,
		commandScript,
		fmt.Sprintf("printf '### %s runner: execution started by requestID => %%s\\n' \"request-123\"",
			constants.ProjectName),
	)

	assert.Contains(
		t,
		commandScript,
		fmt.Sprintf("printf '### %s runner: image ID => %%s\\n' \"ubuntu:22.04\"", constants.ProjectName),
	)
	assert.Contains(
		t,
		commandScript,
		fmt.Sprintf("printf '### %s runner: command => %%s\\n' %q", constants.ProjectName, req.Command),
	)
	assert.True(t, strings.HasSuffix(commandScript, req.Command), "shell command should end with the user command")
	assert.Contains(t, commandScript, "set -e", "script should enable exit on error")
}

func TestBuildMainContainerCommandWithRepo(t *testing.T) {
	repoURL := "https://example.com/repo.git"
	repoRef := "main"
	repoPath := "/nested/path"

	repo := &gitRepoInfo{
		RepoURL:  &repoURL,
		RepoRef:  &repoRef,
		RepoPath: &repoPath,
	}

	req := &api.ExecutionRequest{
		Command: "make test",
	}

	cmd := buildMainContainerCommand(req, "req-456", "golang:1.23", repo)

	require.Len(t, cmd, 3)
	commandScript := cmd[2]

	expectedCd := "cd " + awsConstants.SharedVolumePath + "/repo/nested/path"
	assert.Contains(
		t,
		commandScript,
		expectedCd,
		"should change into the requested git subdirectory",
	)
	assert.Contains(
		t,
		commandScript,
		fmt.Sprintf(
			"printf '### %s runner: checked out repo => %%s (ref: %%s) (path: %%s)\\n' %q %q %q",
			constants.ProjectName,
			repoURL,
			repoRef,
			repoPath,
		),
	)
	expectedWorkingDir := awsConstants.SharedVolumePath + "/repo/nested/path"
	assert.Contains(
		t,
		commandScript,
		fmt.Sprintf("printf '### %s runner: working directory => %%s\\n' %q", constants.ProjectName, expectedWorkingDir),
	)
	assert.True(t, strings.HasSuffix(commandScript, req.Command))
}

func TestExtractTaskARNFromList(t *testing.T) {
	executionID := "abc123"
	taskARNs := []string{
		"arn:aws:ecs:region:123456789012:task/cluster/other-id",
		"arn:aws:ecs:region:123456789012:task/cluster/" + executionID,
	}

	result := extractTaskARNFromList(taskARNs, executionID)

	assert.Equal(t, taskARNs[1], result)
}

func TestValidateTaskStatusForKill(t *testing.T) {
	t.Run("allows runnable statuses", func(t *testing.T) {
		assert.NoError(t, validateTaskStatusForKill(string(awsConstants.EcsStatusRunning)))
		assert.NoError(t, validateTaskStatusForKill(string(awsConstants.EcsStatusActivating)))
	})

	t.Run("rejects already terminated statuses", func(t *testing.T) {
		err := validateTaskStatusForKill(string(awsConstants.EcsStatusStopped))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already terminated")
	})

	t.Run("rejects unexpected status", func(t *testing.T) {
		err := validateTaskStatusForKill(string(awsConstants.EcsStatusPending))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task cannot be terminated in current state")
	})
}

func TestBuildSidecarEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		userEnv  map[string]string
		validate func(t *testing.T, env []ecsTypes.KeyValuePair)
	}{
		{
			name:    "empty user environment",
			userEnv: map[string]string{},
			validate: func(t *testing.T, env []ecsTypes.KeyValuePair) {
				// Should have at least RUNVOY_SHARED_VOLUME_PATH
				assert.Greater(t, len(env), 0)
			},
		},
		{
			name: "single environment variable",
			userEnv: map[string]string{
				"MY_VAR": "my_value",
			},
			validate: func(t *testing.T, env []ecsTypes.KeyValuePair) {
				// Should have RUNVOY_SHARED_VOLUME_PATH + both prefixed and unprefixed versions
				assert.GreaterOrEqual(t, len(env), 2)
			},
		},
		{
			name: "multiple environment variables",
			userEnv: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
				"VAR3": "value3",
			},
			validate: func(t *testing.T, env []ecsTypes.KeyValuePair) {
				// Should have: RUNVOY_SHARED_VOLUME_PATH + 3 prefixed + 3 unprefixed
				assert.GreaterOrEqual(t, len(env), 7)
			},
		},
		{
			name: "special characters in environment values",
			userEnv: map[string]string{
				"GITHUB_TOKEN": "ghp_token123!@#$%",
				"DB_URL":       "postgres://user:pass@host/db",
			},
			validate: func(t *testing.T, env []ecsTypes.KeyValuePair) {
				assert.GreaterOrEqual(t, len(env), 4)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := buildSidecarEnvironment(tt.userEnv)
			tt.validate(t, env)
		})
	}
}

func TestBuildSidecarEnvironmentVariableNames(t *testing.T) {
	userEnv := map[string]string{
		"API_KEY":      "secret-key",
		"GITHUB_TOKEN": "ghp_token",
	}

	env := buildSidecarEnvironment(userEnv)

	// Convert to key-value pairs for easier testing
	envMap := make(map[string]string)
	for _, pair := range env {
		key := *pair.Name
		value := *pair.Value
		envMap[key] = value
	}

	// Check for RUNVOY_SHARED_VOLUME_PATH
	_, hasSharedPath := envMap["RUNVOY_SHARED_VOLUME_PATH"]
	assert.True(t, hasSharedPath, "should have RUNVOY_SHARED_VOLUME_PATH")

	// Check for prefixed versions
	_, hasPrefixedKey := envMap["RUNVOY_USER_API_KEY"]
	assert.True(t, hasPrefixedKey, "should have prefixed RUNVOY_USER_API_KEY")

	// Check for unprefixed versions
	_, hasUnprefixedKey := envMap["API_KEY"]
	assert.True(t, hasUnprefixedKey, "should have unprefixed API_KEY")
}

func TestSanitizeURLForLogging(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "URL with credentials in authority",
			url:      "https://user:pass@github.com/repo",
			expected: "https://***@github.com/repo",
		},
		{
			name:     "URL with token in authority",
			url:      "https://token@github.com/repo",
			expected: "https://***@github.com/repo",
		},
		{
			name:     "URL without credentials",
			url:      "https://github.com/owner/repo",
			expected: "https://github.com/owner/repo",
		},
		{
			name:     "empty URL",
			url:      "",
			expected: "",
		},
		{
			name:     "HTTP URL with credentials",
			url:      "http://admin:secret@example.com/path",
			expected: "http://***@example.com/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeURLForLogging(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}
