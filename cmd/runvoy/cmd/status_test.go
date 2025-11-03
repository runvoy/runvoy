package cmd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"runvoy/internal/api"
)

// mockClientInterface is a manual mock for testing
type mockClientInterface struct {
	getExecutionStatusFunc func(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error)
}

func (m *mockClientInterface) GetExecutionStatus(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error) {
	if m.getExecutionStatusFunc != nil {
		return m.getExecutionStatusFunc(ctx, executionID)
	}
	return nil, fmt.Errorf("not implemented")
}

// Implement other Interface methods (not used in StatusService, but needed to satisfy interface)
func (m *mockClientInterface) GetLogs(ctx context.Context, executionID string) (*api.LogsResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockClientInterface) RunCommand(ctx context.Context, req api.ExecutionRequest) (*api.ExecutionResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockClientInterface) KillExecution(ctx context.Context, executionID string) (*api.KillExecutionResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockClientInterface) ListExecutions(ctx context.Context) ([]api.Execution, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockClientInterface) ClaimAPIKey(ctx context.Context, token string) (*api.ClaimAPIKeyResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockClientInterface) CreateUser(ctx context.Context, req api.CreateUserRequest) (*api.CreateUserResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockClientInterface) RevokeUser(ctx context.Context, req api.RevokeUserRequest) (*api.RevokeUserResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockClientInterface) ListUsers(ctx context.Context) (*api.ListUsersResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockClientInterface) RegisterImage(ctx context.Context, image string, isDefault *bool) (*api.RegisterImageResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockClientInterface) ListImages(ctx context.Context) (*api.ListImagesResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockClientInterface) UnregisterImage(ctx context.Context, image string) (*api.RemoveImageResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// mockOutputInterface is a manual mock for testing
type mockOutputInterface struct {
	calls []call
}

type call struct {
	method string
	args   []interface{}
}

func (m *mockOutputInterface) Infof(format string, a ...interface{}) {
	m.calls = append(m.calls, call{method: "Infof", args: []interface{}{format, a}})
}
func (m *mockOutputInterface) Errorf(format string, a ...interface{}) {
	m.calls = append(m.calls, call{method: "Errorf", args: []interface{}{format, a}})
}
func (m *mockOutputInterface) Successf(format string, a ...interface{}) {
	m.calls = append(m.calls, call{method: "Successf", args: []interface{}{format, a}})
}
func (m *mockOutputInterface) Warningf(format string, a ...interface{}) {
	m.calls = append(m.calls, call{method: "Warningf", args: []interface{}{format, a}})
}
func (m *mockOutputInterface) Table(headers []string, rows [][]string) {
	m.calls = append(m.calls, call{method: "Table", args: []interface{}{headers, rows}})
}
func (m *mockOutputInterface) Blank() {
	m.calls = append(m.calls, call{method: "Blank", args: []interface{}{}})
}
func (m *mockOutputInterface) Bold(text string) string {
	return text
}
func (m *mockOutputInterface) Cyan(text string) string {
	return text
}
func (m *mockOutputInterface) KeyValue(key, value string) {
	m.calls = append(m.calls, call{method: "KeyValue", args: []interface{}{key, value}})
}
func (m *mockOutputInterface) Prompt(prompt string) string {
	m.calls = append(m.calls, call{method: "Prompt", args: []interface{}{prompt}})
	// Return empty string by default - tests can override by checking calls
	return ""
}

func TestStatusService_DisplayStatus(t *testing.T) {
	tests := []struct {
		name        string
		executionID string
		setupMock   func(*mockClientInterface)
		wantErr     bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name:        "successfully displays status",
			executionID: "exec-123",
			setupMock: func(m *mockClientInterface) {
				m.getExecutionStatusFunc = func(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error) {
					now := time.Now()
					return &api.ExecutionStatusResponse{
						ExecutionID: "exec-123",
						Status:      "running",
						StartedAt:   now,
						CompletedAt: nil,
						ExitCode:    nil,
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				require.Greater(t, len(m.calls), 0)
				hasKeyValue := false
				for _, call := range m.calls {
					if call.method == "KeyValue" && len(call.args) >= 2 {
						if call.args[0] == "Execution ID" && call.args[1] == "exec-123" {
							hasKeyValue = true
							break
						}
					}
				}
				assert.True(t, hasKeyValue, "Expected KeyValue call with Execution ID")
			},
		},
		{
			name:        "displays completed status with exit code",
			executionID: "exec-456",
			setupMock: func(m *mockClientInterface) {
				m.getExecutionStatusFunc = func(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error) {
					started := time.Now()
					completed := started.Add(5 * time.Minute)
					exitCode := 0
					return &api.ExecutionStatusResponse{
						ExecutionID: "exec-456",
						Status:      "completed",
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
				m.getExecutionStatusFunc = func(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error) {
					return nil, fmt.Errorf("network error")
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
