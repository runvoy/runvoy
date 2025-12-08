package cmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runvoy/runvoy/internal/api"
)

// mockClientInterface is a manual mock for testing
type mockClientInterface struct {
	getExecutionStatusFunc func(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error)
}

func (m *mockClientInterface) GetExecutionStatus(
	ctx context.Context, executionID string,
) (*api.ExecutionStatusResponse, error) {
	if m.getExecutionStatusFunc != nil {
		return m.getExecutionStatusFunc(ctx, executionID)
	}
	return nil, errors.New("not implemented")
}

// Implement other Interface methods (not used in StatusService, but needed to satisfy interface)
func (m *mockClientInterface) GetLogs(_ context.Context, _ string) (*api.LogsResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) RunCommand(_ context.Context, _ *api.ExecutionRequest) (*api.ExecutionResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) KillExecution(_ context.Context, _ string) (*api.KillExecutionResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) ListExecutions(_ context.Context, _ int, _ string) ([]api.Execution, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) ClaimAPIKey(_ context.Context, _ string) (*api.ClaimAPIKeyResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) CreateUser(_ context.Context, _ api.CreateUserRequest) (*api.CreateUserResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) RevokeUser(_ context.Context, _ api.RevokeUserRequest) (*api.RevokeUserResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) ListUsers(_ context.Context) (*api.ListUsersResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) RegisterImage(
	_ context.Context, _ string, _ *bool, _, _ *string, _, _ *int, _ *string,
) (*api.RegisterImageResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) ListImages(_ context.Context) (*api.ListImagesResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) UnregisterImage(_ context.Context, _ string) (*api.RemoveImageResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) GetImage(_ context.Context, _ string) (*api.ImageInfo, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) CreateSecret(
	_ context.Context,
	_ api.CreateSecretRequest,
) (*api.CreateSecretResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) GetSecret(_ context.Context, _ string) (*api.GetSecretResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) ListSecrets(_ context.Context) (*api.ListSecretsResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) UpdateSecret(
	_ context.Context,
	_ string,
	_ api.UpdateSecretRequest,
) (*api.UpdateSecretResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockClientInterface) DeleteSecret(_ context.Context, _ string) (*api.DeleteSecretResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockClientInterface) ReconcileHealth(_ context.Context) (*api.HealthReconcileResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockClientInterface) FetchBackendLogs(_ context.Context, _ string) (*api.TraceResponse, error) {
	return nil, nil
}

// mockOutputInterface is a manual mock for testing
type mockOutputInterface struct {
	calls []call
}

type call struct {
	method string
	args   []any
}

func (m *mockOutputInterface) Infof(format string, a ...any) {
	m.calls = append(m.calls, call{method: "Infof", args: []any{format, a}})
}
func (m *mockOutputInterface) Errorf(format string, a ...any) {
	m.calls = append(m.calls, call{method: "Errorf", args: []any{format, a}})
}
func (m *mockOutputInterface) Successf(format string, a ...any) {
	m.calls = append(m.calls, call{method: "Successf", args: []any{format, a}})
}
func (m *mockOutputInterface) Warningf(format string, a ...any) {
	m.calls = append(m.calls, call{method: "Warningf", args: []any{format, a}})
}
func (m *mockOutputInterface) Table(headers []string, rows [][]string) {
	m.calls = append(m.calls, call{method: "Table", args: []any{headers, rows}})
}
func (m *mockOutputInterface) Blank() {
	m.calls = append(m.calls, call{method: "Blank", args: []any{}})
}
func (m *mockOutputInterface) Bold(text string) string {
	return text
}
func (m *mockOutputInterface) Cyan(text string) string {
	return text
}
func (m *mockOutputInterface) KeyValue(key, value string) {
	m.calls = append(m.calls, call{method: "KeyValue", args: []any{key, value}})
}
func (m *mockOutputInterface) Prompt(prompt string) string {
	m.calls = append(m.calls, call{method: "Prompt", args: []any{prompt}})
	// Return empty string by default - tests can override by checking calls
	return ""
}

func TestStatusService_DisplayStatus(t *testing.T) {
	tests := []struct {
		name         string
		executionID  string
		setupMock    func(*mockClientInterface)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name:        "successfully displays status",
			executionID: "exec-123",
			setupMock: func(m *mockClientInterface) {
				m.getExecutionStatusFunc = func(_ context.Context, _ string) (*api.ExecutionStatusResponse, error) {
					now := time.Now()
					return &api.ExecutionStatusResponse{
						ExecutionID: "exec-123",
						Status:      "running",
						Command:     "echo hello",
						ImageID:     "alpine:latest-abc123",
						StartedAt:   now,
						CompletedAt: nil,
						ExitCode:    nil,
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				require.NotEmpty(t, m.calls)
				hasKeyValue := false
				var commandShown, imageShown bool
				for _, call := range m.calls {
					if call.method == "KeyValue" && len(call.args) >= 2 {
						if call.args[0] == "Execution ID" && call.args[1] == "exec-123" {
							hasKeyValue = true
						}
						if call.args[0] == "Command" && call.args[1] == "echo hello" {
							commandShown = true
						}
						if call.args[0] == "Image ID" && call.args[1] == "alpine:latest-abc123" {
							imageShown = true
						}
					}
				}
				assert.True(t, hasKeyValue, "Expected KeyValue call with Execution ID")
				assert.True(t, commandShown, "Expected Command to be displayed")
				assert.True(t, imageShown, "Expected Image ID to be displayed")
			},
		},
		{
			name:        "displays completed status with exit code",
			executionID: "exec-456",
			setupMock: func(m *mockClientInterface) {
				m.getExecutionStatusFunc = func(_ context.Context, _ string) (*api.ExecutionStatusResponse, error) {
					started := time.Now()
					completed := started.Add(5 * time.Minute)
					exitCode := 0
					return &api.ExecutionStatusResponse{
						ExecutionID: "exec-456",
						Status:      "completed",
						Command:     "sleep 10",
						ImageID:     "ubuntu:latest-123",
						StartedAt:   started,
						CompletedAt: &completed,
						ExitCode:    &exitCode,
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				var hasCompletedAt, hasExitCode bool
				for _, call := range m.calls {
					if call.method == "KeyValue" && len(call.args) >= 2 {
						key := call.args[0]
						if key == "Completed At" {
							hasCompletedAt = true
						}
						if key == "Exit Code" {
							hasExitCode = true
						}
					}
				}
				assert.True(t, hasCompletedAt, "Expected Completed At to be displayed")
				assert.True(t, hasExitCode, "Expected Exit Code to be displayed")
			},
		},
		{
			name:        "handles client error",
			executionID: "exec-789",
			setupMock: func(m *mockClientInterface) {
				m.getExecutionStatusFunc = func(_ context.Context, _ string) (*api.ExecutionStatusResponse, error) {
					return nil, errors.New("network error")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				// Should not have any KeyValue calls when there's an error
				for _, call := range m.calls {
					assert.NotEqual(t, "KeyValue", call.method, "Should not display KeyValue on error")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientInterface{}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewStatusService(mockClient, mockOutput)

			err := service.DisplayStatus(context.Background(), tt.executionID)

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
