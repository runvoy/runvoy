// Package api defines the API types and structures used across runvoy.
package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionRequestJSON(t *testing.T) {
	t.Run("marshal and unmarshal with all fields", func(t *testing.T) {
		req := ExecutionRequest{
			Command: "echo hello",
			Image:   "alpine:latest",
			Env:     map[string]string{"KEY": "value"},
			Timeout: 300,
			Secrets: []string{"secret1", "secret2"},
			GitRepo: "https://github.com/user/repo.git",
			GitRef:  "main",
			GitPath: ".",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled ExecutionRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Command, unmarshaled.Command)
		assert.Equal(t, req.Image, unmarshaled.Image)
		assert.Equal(t, req.Env, unmarshaled.Env)
		assert.Equal(t, req.Timeout, unmarshaled.Timeout)
		assert.Equal(t, req.Secrets, unmarshaled.Secrets)
		assert.Equal(t, req.GitRepo, unmarshaled.GitRepo)
		assert.Equal(t, req.GitRef, unmarshaled.GitRef)
		assert.Equal(t, req.GitPath, unmarshaled.GitPath)
	})

	t.Run("omit empty optional fields", func(t *testing.T) {
		req := ExecutionRequest{
			Command: "echo hello",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		// Optional fields should be omitted when empty
		assert.NotContains(t, string(data), "image")
		assert.NotContains(t, string(data), "env")
		assert.NotContains(t, string(data), "timeout")
	})
}

func TestExecutionResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		resp := ExecutionResponse{
			ExecutionID: "exec-123",
			LogURL:      "https://example.com/logs",
			Status:      "running",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled ExecutionResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.ExecutionID, unmarshaled.ExecutionID)
		assert.Equal(t, resp.LogURL, unmarshaled.LogURL)
		assert.Equal(t, resp.Status, unmarshaled.Status)
	})
}

func TestExecutionStatusResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal with completed execution", func(t *testing.T) {
		now := time.Now()
		exitCode := 0
		resp := ExecutionStatusResponse{
			ExecutionID: "exec-123",
			Status:      "completed",
			StartedAt:   now,
			ExitCode:    &exitCode,
			CompletedAt: &now,
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled ExecutionStatusResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.ExecutionID, unmarshaled.ExecutionID)
		assert.Equal(t, resp.Status, unmarshaled.Status)
		assert.NotNil(t, unmarshaled.ExitCode)
		assert.Equal(t, *resp.ExitCode, *unmarshaled.ExitCode)
		assert.NotNil(t, unmarshaled.CompletedAt)
	})

	t.Run("marshal and unmarshal running execution", func(t *testing.T) {
		now := time.Now()
		resp := ExecutionStatusResponse{
			ExecutionID: "exec-123",
			Status:      "running",
			StartedAt:   now,
			ExitCode:    nil,
			CompletedAt: nil,
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled ExecutionStatusResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.ExecutionID, unmarshaled.ExecutionID)
		assert.Equal(t, resp.Status, unmarshaled.Status)
		assert.Nil(t, unmarshaled.CompletedAt)
	})
}

func TestKillExecutionResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		resp := KillExecutionResponse{
			ExecutionID: "exec-123",
			Message:     "Execution killed successfully",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled KillExecutionResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.ExecutionID, unmarshaled.ExecutionID)
		assert.Equal(t, resp.Message, unmarshaled.Message)
	})
}

func TestExecutionJSON(t *testing.T) {
	t.Run("marshal and unmarshal completed execution", func(t *testing.T) {
		now := time.Now()
		exec := Execution{
			ExecutionID:         "exec-123",
			CreatedBy:           "user@example.com",
			OwnedBy:             []string{"user@example.com"},
			Command:             "echo hello",
			StartedAt:           now,
			CompletedAt:         &now,
			Status:              "completed",
			ExitCode:            0,
			DurationSeconds:     5,
			LogStreamName:       "stream-123",
			CreatedByRequestID:  "req-123",
			ComputePlatform:     "aws",
		}

		data, err := json.Marshal(exec)
		require.NoError(t, err)

		var unmarshaled Execution
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, exec.ExecutionID, unmarshaled.ExecutionID)
		assert.Equal(t, exec.CreatedBy, unmarshaled.CreatedBy)
		assert.Equal(t, exec.Command, unmarshaled.Command)
		assert.Equal(t, exec.Status, unmarshaled.Status)
		assert.Equal(t, exec.ExitCode, unmarshaled.ExitCode)
		assert.NotNil(t, unmarshaled.CompletedAt)
	})

	t.Run("marshal and unmarshal running execution", func(t *testing.T) {
		now := time.Now()
		exec := Execution{
			ExecutionID: "exec-123",
			CreatedBy:   "user@example.com",
			OwnedBy:     []string{"user@example.com"},
			Command:     "echo hello",
			StartedAt:   now,
			CompletedAt: nil,
			Status:      "running",
		}

		data, err := json.Marshal(exec)
		require.NoError(t, err)

		var unmarshaled Execution
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, exec.ExecutionID, unmarshaled.ExecutionID)
		assert.Equal(t, exec.Status, unmarshaled.Status)
		assert.Nil(t, unmarshaled.CompletedAt)
	})
}
