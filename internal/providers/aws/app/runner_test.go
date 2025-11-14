package aws

import (
	"fmt"
	"strings"
	"testing"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	awsConstants "runvoy/internal/providers/aws/constants"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSidecarContainerCommandWithoutGitRepo(t *testing.T) {
	cmd := buildSidecarContainerCommand(false, map[string]string{})

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
	cmd := buildSidecarContainerCommand(true, map[string]string{})

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
		fmt.Sprintf("printf '### %s runner execution started by requestID => %%s\\n' \"request-123\"", constants.ProjectName),
	)

	assert.Contains(t, commandScript, "printf '### Docker image => %s\\n' \"ubuntu:22.04\"")
	assert.Contains(
		t,
		commandScript,
		fmt.Sprintf("printf '### %s command => %%s\\n' %q", constants.ProjectName, req.Command),
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
			"printf '### Checked out repo => %%s (ref: %%s) (path: %%s)\\n' %q %q %q",
			repoURL,
			repoRef,
			repoPath,
		),
	)
	assert.Contains(
		t,
		commandScript,
		fmt.Sprintf("printf '### Working directory => %%s\\n' %q", awsConstants.SharedVolumePath+"/repo/nested/path"),
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
