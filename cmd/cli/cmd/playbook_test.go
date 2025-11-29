package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/client/playbooks"
	"github.com/runvoy/runvoy/internal/constants"
)

func TestPlaybookService_ListPlaybooks(t *testing.T) {
	t.Run("lists playbooks successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		playbookDir := filepath.Join(tmpDir, ".runvoy")
		err := os.MkdirAll(playbookDir, 0o750)
		require.NoError(t, err)

		yamlContent1 := `description: First playbook
commands:
  - echo hello
`
		yamlContent2 := `description: Second playbook
commands:
  - echo world
`
		err = os.WriteFile(filepath.Join(playbookDir, "playbook1.yaml"), []byte(yamlContent1), 0o600)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(playbookDir, "playbook2.yaml"), []byte(yamlContent2), 0o600)
		require.NoError(t, err)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		loader := playbooks.NewPlaybookLoader()
		mockOutput := &mockOutputInterface{}
		service := NewPlaybookService(loader, nil, mockOutput)

		err = service.ListPlaybooks(context.Background())
		assert.NoError(t, err)

		hasTable := false
		for _, call := range mockOutput.calls {
			if call.method == "Table" {
				hasTable = true
				if len(call.args) >= 2 {
					headers := call.args[0].([]string)
					rows := call.args[1].([][]string)
					assert.Contains(t, headers, "Name")
					assert.Contains(t, headers, "Description")
					assert.Len(t, rows, 2)
				}
			}
		}
		assert.True(t, hasTable, "Expected Table call")
	})

	t.Run("handles empty playbook directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		loader := playbooks.NewPlaybookLoader()
		mockOutput := &mockOutputInterface{}
		service := NewPlaybookService(loader, nil, mockOutput)

		err = service.ListPlaybooks(context.Background())
		assert.NoError(t, err)

		hasWarning := false
		for _, call := range mockOutput.calls {
			if call.method == "Warningf" {
				hasWarning = true
			}
		}
		assert.True(t, hasWarning, "Expected warning for empty directory")
	})
}

func TestPlaybookService_ShowPlaybook(t *testing.T) {
	t.Run("shows playbook details", func(t *testing.T) {
		tmpDir := t.TempDir()
		playbookDir := filepath.Join(tmpDir, ".runvoy")
		err := os.MkdirAll(playbookDir, 0o750)
		require.NoError(t, err)

		yamlContent := `description: Test playbook
image: test/image:latest
git_repo: https://github.com/test/repo.git
git_ref: main
git_path: /path
secrets:
  - secret1
env:
  KEY1: value1
commands:
  - echo hello
  - echo world
`
		err = os.WriteFile(filepath.Join(playbookDir, "test.yaml"), []byte(yamlContent), 0o600)
		require.NoError(t, err)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		loader := playbooks.NewPlaybookLoader()
		mockOutput := &mockOutputInterface{}
		service := NewPlaybookService(loader, nil, mockOutput)

		err = service.ShowPlaybook(context.Background(), "test")
		assert.NoError(t, err)

		hasKeyValue := false
		hasCommands := false
		for _, call := range mockOutput.calls {
			if call.method == "KeyValue" && len(call.args) >= 2 {
				key := call.args[0].(string)
				if key == "Name" {
					hasKeyValue = true
				}
				if key == "Commands" {
					hasCommands = true
					value := call.args[1].(string)
					assert.Contains(t, value, "echo hello")
					assert.Contains(t, value, "echo world")
				}
			}
		}
		assert.True(t, hasKeyValue, "Expected KeyValue call for Name")
		assert.True(t, hasCommands, "Expected Commands to be displayed")
	})

	t.Run("handles missing playbook", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		loader := playbooks.NewPlaybookLoader()
		mockOutput := &mockOutputInterface{}
		service := NewPlaybookService(loader, nil, mockOutput)

		err = service.ShowPlaybook(context.Background(), "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "playbook not found")
	})
}

func TestPlaybookService_RunPlaybook(t *testing.T) {
	t.Run("executes playbook successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		playbookDir := filepath.Join(tmpDir, ".runvoy")
		err := os.MkdirAll(playbookDir, 0o750)
		require.NoError(t, err)

		yamlContent := `description: Test playbook
commands:
  - echo hello
`
		err = os.WriteFile(filepath.Join(playbookDir, "test.yaml"), []byte(yamlContent), 0o600)
		require.NoError(t, err)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		loader := playbooks.NewPlaybookLoader()
		executor := playbooks.NewPlaybookExecutor()
		mockOutput := &mockOutputInterface{}
		mockClient := &mockClientInterfaceForRun{
			mockClientInterface: &mockClientInterface{},
		}
		mockClient.runCommandFunc = func(_ context.Context, req *api.ExecutionRequest) (*api.ExecutionResponse, error) {
			assert.Equal(t, "echo hello", req.Command)
			return &api.ExecutionResponse{
				ExecutionID:  "exec-123",
				Status:       "STARTING",
				WebSocketURL: "wss://example.com/logs/exec-123",
			}, nil
		}
		mockClient.getLogsFunc = func(_ context.Context, executionID string) (*api.LogsResponse, error) {
			return &api.LogsResponse{
				ExecutionID:  executionID,
				Status:       string(constants.ExecutionRunning),
				WebSocketURL: "wss://example.com/logs/" + executionID,
				Events:       []api.LogEvent{},
			}, nil
		}

		runService := NewRunService(mockClient, mockOutput)
		runService.streamLogs = func(_ *LogsService, websocketURL, _ string, _ string) error {
			assert.NotEmpty(t, websocketURL)
			return nil
		}
		service := NewPlaybookService(loader, executor, mockOutput)

		overrides := &PlaybookOverrides{}
		err = service.RunPlaybook(context.Background(), "test", nil, overrides, "", runService)
		assert.NoError(t, err)

		hasInfof := false
		for _, call := range mockOutput.calls {
			if call.method == "Infof" {
				hasInfof = true
			}
		}
		assert.True(t, hasInfof, "Expected Infof call")
	})

	t.Run("applies overrides correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		playbookDir := filepath.Join(tmpDir, ".runvoy")
		err := os.MkdirAll(playbookDir, 0o750)
		require.NoError(t, err)

		yamlContent := `description: Test playbook
image: original/image:latest
commands:
  - echo hello
`
		err = os.WriteFile(filepath.Join(playbookDir, "test.yaml"), []byte(yamlContent), 0o600)
		require.NoError(t, err)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		loader := playbooks.NewPlaybookLoader()
		executor := playbooks.NewPlaybookExecutor()
		mockOutput := &mockOutputInterface{}
		mockClient := &mockClientInterfaceForRun{
			mockClientInterface: &mockClientInterface{},
		}
		mockClient.runCommandFunc = func(_ context.Context, req *api.ExecutionRequest) (*api.ExecutionResponse, error) {
			assert.Equal(t, "override/image:latest", req.Image)
			return &api.ExecutionResponse{
				ExecutionID:  "exec-123",
				Status:       "STARTING",
				WebSocketURL: "wss://example.com/logs/exec-123",
			}, nil
		}
		mockClient.getLogsFunc = func(_ context.Context, executionID string) (*api.LogsResponse, error) {
			return &api.LogsResponse{
				ExecutionID:  executionID,
				Status:       string(constants.ExecutionRunning),
				WebSocketURL: "wss://example.com/logs/" + executionID,
				Events:       []api.LogEvent{},
			}, nil
		}

		runService := NewRunService(mockClient, mockOutput)
		runService.streamLogs = func(_ *LogsService, websocketURL, _ string, _ string) error {
			assert.NotEmpty(t, websocketURL)
			return nil
		}
		service := NewPlaybookService(loader, executor, mockOutput)

		overrides := &PlaybookOverrides{
			Image: "override/image:latest",
		}
		err = service.RunPlaybook(context.Background(), "test", nil, overrides, "", runService)
		assert.NoError(t, err)
	})

	t.Run("merges user env and secrets", func(t *testing.T) {
		tmpDir := t.TempDir()
		playbookDir := filepath.Join(tmpDir, ".runvoy")
		err := os.MkdirAll(playbookDir, 0o750)
		require.NoError(t, err)

		yamlContent := `secrets:
  - playbook-secret
env:
  PLAYBOOK_KEY: playbook-value
commands:
  - echo hello
`
		err = os.WriteFile(filepath.Join(playbookDir, "test.yaml"), []byte(yamlContent), 0o600)
		require.NoError(t, err)

		oldWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldWd) }()

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		loader := playbooks.NewPlaybookLoader()
		executor := playbooks.NewPlaybookExecutor()
		mockOutput := &mockOutputInterface{}
		mockClient := &mockClientInterfaceForRun{
			mockClientInterface: &mockClientInterface{},
		}
		mockClient.runCommandFunc = func(_ context.Context, req *api.ExecutionRequest) (*api.ExecutionResponse, error) {
			assert.Contains(t, req.Secrets, "playbook-secret")
			assert.Contains(t, req.Secrets, "user-secret")
			assert.Equal(t, "playbook-value", req.Env["PLAYBOOK_KEY"])
			assert.Equal(t, "user-value", req.Env["USER_KEY"])
			return &api.ExecutionResponse{
				ExecutionID:  "exec-123",
				Status:       "STARTING",
				WebSocketURL: "wss://example.com/logs/exec-123",
			}, nil
		}
		mockClient.getLogsFunc = func(_ context.Context, executionID string) (*api.LogsResponse, error) {
			return &api.LogsResponse{
				ExecutionID:  executionID,
				Status:       string(constants.ExecutionRunning),
				WebSocketURL: "wss://example.com/logs/" + executionID,
				Events:       []api.LogEvent{},
			}, nil
		}

		runService := NewRunService(mockClient, mockOutput)
		runService.streamLogs = func(_ *LogsService, websocketURL, _ string, _ string) error {
			assert.NotEmpty(t, websocketURL)
			return nil
		}
		service := NewPlaybookService(loader, executor, mockOutput)

		userEnv := map[string]string{
			"USER_KEY": "user-value",
		}
		overrides := &PlaybookOverrides{
			Secrets: []string{"user-secret"},
		}
		err = service.RunPlaybook(context.Background(), "test", userEnv, overrides, "", runService)
		assert.NoError(t, err)
	})
}

func TestApplyOverrides(t *testing.T) {
	t.Run("applies all overrides", func(t *testing.T) {
		pb := &api.Playbook{
			Image:    "original/image:latest",
			GitRepo:  "https://github.com/original/repo.git",
			GitRef:   "main",
			GitPath:  "/original",
			Commands: []string{"echo hello"},
		}

		overrides := &PlaybookOverrides{
			Image:   "override/image:latest",
			GitRepo: "https://github.com/override/repo.git",
			GitRef:  "develop",
			GitPath: "/override",
		}

		applyOverrides(pb, overrides)

		assert.Equal(t, "override/image:latest", pb.Image)
		assert.Equal(t, "https://github.com/override/repo.git", pb.GitRepo)
		assert.Equal(t, "develop", pb.GitRef)
		assert.Equal(t, "/override", pb.GitPath)
	})

	t.Run("preserves original values when override is empty", func(t *testing.T) {
		pb := &api.Playbook{
			Image:    "original/image:latest",
			GitRepo:  "https://github.com/original/repo.git",
			Commands: []string{"echo hello"},
		}

		overrides := &PlaybookOverrides{
			Image: "override/image:latest",
		}

		applyOverrides(pb, overrides)

		assert.Equal(t, "override/image:latest", pb.Image)
		assert.Equal(t, "https://github.com/original/repo.git", pb.GitRepo)
	})
}
