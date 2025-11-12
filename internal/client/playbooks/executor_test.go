package playbooks

import (
	"runvoy/internal/api"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlaybookExecutor_ToExecutionRequest(t *testing.T) {
	t.Run("converts playbook to execution request", func(t *testing.T) {
		pb := &api.Playbook{
			Description: "Test playbook",
			Image:       "test/image:latest",
			GitRepo:     "https://github.com/test/repo.git",
			GitRef:      "main",
			GitPath:     "/path",
			Secrets:     []string{"secret1", "secret2"},
			Env: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			Commands: []string{"echo hello", "echo world", "echo test"},
		}

		userEnv := map[string]string{
			"KEY2": "user_value2",
			"KEY3": "value3",
		}
		userSecrets := []string{"secret3"}

		executor := NewPlaybookExecutor()
		req := executor.ToExecutionRequest(pb, userEnv, userSecrets)

		assert.Equal(t, "echo hello && echo world && echo test", req.Command)
		assert.Equal(t, "test/image:latest", req.Image)
		assert.Equal(t, "https://github.com/test/repo.git", req.GitRepo)
		assert.Equal(t, "main", req.GitRef)
		assert.Equal(t, "/path", req.GitPath)
		assert.Equal(t, []string{"secret1", "secret2", "secret3"}, req.Secrets)
		assert.Equal(t, map[string]string{
			"KEY1": "value1",
			"KEY2": "user_value2", // user env takes precedence
			"KEY3": "value3",
		}, req.Env)
	})

	t.Run("handles empty playbook fields", func(t *testing.T) {
		pb := &api.Playbook{
			Commands: []string{"echo hello"},
		}

		executor := NewPlaybookExecutor()
		req := executor.ToExecutionRequest(pb, nil, nil)

		assert.Equal(t, "echo hello", req.Command)
		assert.Empty(t, req.Image)
		assert.Empty(t, req.GitRepo)
		assert.Empty(t, req.GitRef)
		assert.Empty(t, req.GitPath)
		assert.Empty(t, req.Secrets)
		assert.Empty(t, req.Env)
	})

	t.Run("combines single command", func(t *testing.T) {
		pb := &api.Playbook{
			Commands: []string{"echo hello"},
		}

		executor := NewPlaybookExecutor()
		req := executor.ToExecutionRequest(pb, nil, nil)

		assert.Equal(t, "echo hello", req.Command)
	})
}
