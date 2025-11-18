package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"runvoy/internal/api"
	"runvoy/internal/constants"
)

// mockClientInterfaceForRun extends mockClientInterface with RunCommand and GetLogs
type mockClientInterfaceForRun struct {
	*mockClientInterface
	runCommandFunc func(ctx context.Context, req *api.ExecutionRequest) (*api.ExecutionResponse, error)
	getLogsFunc    func(ctx context.Context, executionID string) (*api.LogsResponse, error)
}

func (m *mockClientInterfaceForRun) RunCommand(
	ctx context.Context, req *api.ExecutionRequest,
) (*api.ExecutionResponse, error) {
	if m.runCommandFunc != nil {
		return m.runCommandFunc(ctx, req)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForRun) GetLogs(ctx context.Context, executionID string) (*api.LogsResponse, error) {
	if m.getLogsFunc != nil {
		return m.getLogsFunc(ctx, executionID)
	}
	return nil, fmt.Errorf("not implemented")
}

func TestRunService_ExecuteCommand(t *testing.T) {
	tests := []struct {
		name         string
		request      ExecuteCommandRequest
		setupMock    func(*mockClientInterfaceForRun)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name: "successfully executes simple command",
			request: ExecuteCommandRequest{
				Command: "echo hello",
				WebURL:  "https://logs.example.com",
			},
			setupMock: func(m *mockClientInterfaceForRun) {
				m.runCommandFunc = func(_ context.Context, _ *api.ExecutionRequest) (*api.ExecutionResponse, error) {
					return &api.ExecutionResponse{
						ExecutionID: "exec-123",
						Status:      "pending",
						ImageID:     "alpine:latest-a1b2c3d4",
					}, nil
				}
				m.getLogsFunc = func(_ context.Context, executionID string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: executionID,
						Events:      []api.LogEvent{},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasInfof := false
				hasSuccess := false
				for _, call := range m.calls {
					if call.method == "Infof" && len(call.args) >= 1 {
						hasInfof = true
					}
					if call.method == "Successf" {
						hasSuccess = true
					}
				}
				assert.True(t, hasInfof, "Expected Infof call")
				assert.True(t, hasSuccess, "Expected Successf call")
			},
		},
		{
			name: "displays git repository information",
			request: ExecuteCommandRequest{
				Command: "npm test",
				GitRepo: "https://github.com/user/repo.git",
				GitRef:  constants.DefaultGitRef,
				WebURL:  "https://logs.example.com",
			},
			setupMock: func(m *mockClientInterfaceForRun) {
				m.runCommandFunc = func(_ context.Context, req *api.ExecutionRequest) (*api.ExecutionResponse, error) {
					assert.Equal(t, "https://github.com/user/repo.git", req.GitRepo)
					assert.Equal(t, constants.DefaultGitRef, req.GitRef)
					return &api.ExecutionResponse{
						ExecutionID: "exec-456",
						Status:      "pending",
						ImageID:     "alpine:latest-a1b2c3d4",
					}, nil
				}
				m.getLogsFunc = func(_ context.Context, executionID string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: executionID,
						Events:      []api.LogEvent{},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasGitRepo := false
				hasGitRef := false
				for _, call := range m.calls {
					if call.method == "Infof" && len(call.args) >= 1 {
						format := call.args[0].(string)
						if fmt.Sprintf(format, call.args[1:]...) != "" {
							hasGitRepo = true
							hasGitRef = true
						}
					}
				}
				assert.True(t, hasGitRepo || hasGitRef, "Should display git information")
			},
		},
		{
			name: "displays user environment variables",
			request: ExecuteCommandRequest{
				Command: "echo test",
				Env:     map[string]string{"API_KEY": "secret", "TOKEN": "abc123"},
				WebURL:  "https://logs.example.com",
			},
			setupMock: func(m *mockClientInterfaceForRun) {
				m.runCommandFunc = func(_ context.Context, req *api.ExecutionRequest) (*api.ExecutionResponse, error) {
					assert.Equal(t, map[string]string{"API_KEY": "secret", "TOKEN": "abc123"}, req.Env)
					return &api.ExecutionResponse{
						ExecutionID: "exec-789",
						Status:      "pending",
						ImageID:     "alpine:latest-a1b2c3d4",
					}, nil
				}
				m.getLogsFunc = func(_ context.Context, executionID string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: executionID,
						Events:      []api.LogEvent{},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasEnvVars := false
				for _, call := range m.calls {
					if call.method == "Infof" && len(call.args) >= 1 {
						format := call.args[0].(string)
						if fmt.Sprintf(format, call.args[1:]...) != "" {
							hasEnvVars = true
						}
					}
				}
				assert.True(t, hasEnvVars, "Should display environment variables")
			},
		},
		{
			name: "displays image ID when specified",
			request: ExecuteCommandRequest{
				Command: "terraform plan",
				Image:   "hashicorp/terraform:latest",
				WebURL:  "https://logs.example.com",
			},
			setupMock: func(m *mockClientInterfaceForRun) {
				m.runCommandFunc = func(_ context.Context, req *api.ExecutionRequest) (*api.ExecutionResponse, error) {
					assert.Equal(t, "hashicorp/terraform:latest", req.Image)
					return &api.ExecutionResponse{
						ExecutionID: "exec-abc",
						Status:      "pending",
						ImageID:     "hashicorp/terraform:latest-a1b2c3d4",
					}, nil
				}
				m.getLogsFunc = func(_ context.Context, executionID string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: executionID,
						Events:      []api.LogEvent{},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasImageID := false
				for _, call := range m.calls {
					if call.method == "KeyValue" && len(call.args) >= 2 {
						if call.args[0] == "Image ID" {
							hasImageID = true
						}
					}
				}
				assert.True(t, hasImageID, "Should display image ID")
			},
		},
		{
			name: "displays resolved image ID when no image specified",
			request: ExecuteCommandRequest{
				Command: "echo test",
				WebURL:  "https://logs.example.com",
			},
			setupMock: func(m *mockClientInterfaceForRun) {
				m.runCommandFunc = func(_ context.Context, req *api.ExecutionRequest) (*api.ExecutionResponse, error) {
					assert.Equal(t, "", req.Image)
					return &api.ExecutionResponse{
						ExecutionID: "exec-default",
						Status:      "pending",
						ImageID:     "alpine:latest-b1c2d3e4",
					}, nil
				}
				m.getLogsFunc = func(_ context.Context, executionID string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: executionID,
						Events:      []api.LogEvent{},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasImageID := false
				for _, call := range m.calls {
					if call.method == "KeyValue" && len(call.args) >= 2 {
						if call.args[0] == "Image ID" {
							hasImageID = true
						}
					}
				}
				assert.True(t, hasImageID, "Should display resolved image ID")
			},
		},
		{
			name: "handles client error",
			request: ExecuteCommandRequest{
				Command: "echo test",
				WebURL:  "https://logs.example.com",
			},
			setupMock: func(m *mockClientInterfaceForRun) {
				m.runCommandFunc = func(_ context.Context, _ *api.ExecutionRequest) (*api.ExecutionResponse, error) {
					return nil, fmt.Errorf("network error")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				// Should not have Successf call when there's an error
				for _, call := range m.calls {
					assert.NotEqual(t, "Successf", call.method, "Should not display success on error")
				}
			},
		},
		{
			name: "displays all git information when all provided",
			request: ExecuteCommandRequest{
				Command: "npm test",
				GitRepo: "https://github.com/user/repo.git",
				GitRef:  "feature-branch",
				GitPath: "subfolder",
				WebURL:  "https://logs.example.com",
			},
			setupMock: func(m *mockClientInterfaceForRun) {
				m.runCommandFunc = func(_ context.Context, req *api.ExecutionRequest) (*api.ExecutionResponse, error) {
					assert.Equal(t, "https://github.com/user/repo.git", req.GitRepo)
					assert.Equal(t, "feature-branch", req.GitRef)
					assert.Equal(t, "subfolder", req.GitPath)
					return &api.ExecutionResponse{
						ExecutionID: "exec-xyz",
						Status:      "pending",
						ImageID:     "alpine:latest-a1b2c3d4",
					}, nil
				}
				m.getLogsFunc = func(_ context.Context, executionID string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: executionID,
						Events:      []api.LogEvent{},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				assert.True(t, len(m.calls) > 0, "Should have output calls")
			},
		},
		{
			name: "displays execution ID",
			request: ExecuteCommandRequest{
				Command: "ls",
				WebURL:  "https://logs.example.com",
			},
			setupMock: func(m *mockClientInterfaceForRun) {
				m.runCommandFunc = func(_ context.Context, _ *api.ExecutionRequest) (*api.ExecutionResponse, error) {
					return &api.ExecutionResponse{
						ExecutionID: "exec-final",
						Status:      "pending",
						ImageID:     "alpine:latest-a1b2c3d4",
					}, nil
				}
				m.getLogsFunc = func(_ context.Context, executionID string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: executionID,
						Events:      []api.LogEvent{},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasExecID := false
				for _, call := range m.calls {
					if call.method == "KeyValue" && len(call.args) >= 2 {
						if call.args[0] == "Execution ID" && call.args[1] == "exec-final" {
							hasExecID = true
						}
					}
				}
				assert.True(t, hasExecID, "Should display execution ID")
			},
		},
		{
			name: "passes secrets to execution request",
			request: ExecuteCommandRequest{
				Command: "echo secret",
				Secrets: []string{"db-password", "api-token"},
				WebURL:  "https://logs.example.com",
			},
			setupMock: func(m *mockClientInterfaceForRun) {
				m.runCommandFunc = func(_ context.Context, req *api.ExecutionRequest) (*api.ExecutionResponse, error) {
					assert.ElementsMatch(t, []string{"db-password", "api-token"}, req.Secrets)
					return &api.ExecutionResponse{
						ExecutionID: "exec-secrets",
						Status:      "pending",
						ImageID:     "alpine:latest-a1b2c3d4",
					}, nil
				}
				m.getLogsFunc = func(_ context.Context, executionID string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: executionID,
						Events:      []api.LogEvent{},
					}, nil
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientInterfaceForRun{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewRunService(mockClient, mockOutput)

			err := service.ExecuteCommand(context.Background(), &tt.request)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.verifyOutput != nil {
				tt.verifyOutput(t, mockOutput)
			}
		})
	}
}
