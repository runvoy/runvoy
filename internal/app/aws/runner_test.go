package aws

import (
	"strings"
	"testing"

	"runvoy/internal/api"
	"runvoy/internal/constants"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSidecarContainerCommandWithoutGitRepo(t *testing.T) {
	cmd := buildSidecarContainerCommand(false)

	require.Len(t, cmd, 3, "expected shell command with interpreter and script")
	assert.Equal(t, "/bin/sh", cmd[0])
	assert.Equal(t, "-c", cmd[1])

	script := cmd[2]

	assert.Contains(t, script, "grep '^RUNVOY_USER_'", "should create .env file when user env vars exist")
	assert.Contains(t, script, constants.ProjectName+" sidecar: No git repository specified, exiting")
	assert.NotContains(t, script, "git clone", "git repo commands must be skipped when not requested")
}

func TestBuildSidecarContainerCommandWithGitRepo(t *testing.T) {
	cmd := buildSidecarContainerCommand(true)

	require.Len(t, cmd, 3)
	script := cmd[2]

	assert.Contains(t, script, "apk add --no-cache git", "should install git when repo cloning is requested")
	assert.Contains(t, script, "git clone --depth 1 --branch \"${GIT_REF}\" \"${GIT_REPO}\" \"${CLONE_PATH}\"")
	assert.Contains(t, script, "cp \"${RUNVOY_SHARED_VOLUME_PATH}/.env\" \"${CLONE_PATH}/.env\"")
	assert.Contains(t, script, constants.ProjectName+" sidecar: .env file copied to repo directory")
}

func TestBuildMainContainerCommandWithoutRepo(t *testing.T) {
	req := &api.ExecutionRequest{
		Command: "echo 'hello world'",
	}

	cmd := buildMainContainerCommand(req, "request-123", "ubuntu:22.04", nil)

	require.Len(t, cmd, 3)
	commandScript := cmd[2]

	assert.Contains(t, commandScript, "printf '### "+constants.ProjectName+" runner execution started by requestID => request-123")
	assert.Contains(t, commandScript, "printf '### Docker image => ubuntu:22.04")
	assert.Contains(t, commandScript, constants.ProjectName+" command => "+req.Command)
	assert.True(t, strings.HasSuffix(commandScript, req.Command), "shell command should end with the user command")
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

	assert.Contains(t, commandScript, "cd "+constants.SharedVolumePath+"/repo/nested/path", "should change into the requested git subdirectory")
	assert.Contains(t, commandScript, "Checked out repo => "+repoURL+" (ref: "+repoRef+") (path: "+repoPath+")")
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
		assert.NoError(t, validateTaskStatusForKill(string(constants.EcsStatusRunning)))
		assert.NoError(t, validateTaskStatusForKill(string(constants.EcsStatusActivating)))
	})

	t.Run("rejects already terminated statuses", func(t *testing.T) {
		err := validateTaskStatusForKill(string(constants.EcsStatusStopped))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already terminated")
	})

	t.Run("rejects unexpected status", func(t *testing.T) {
		err := validateTaskStatusForKill(string(constants.EcsStatusPending))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task cannot be terminated in current state")
	})
}
