package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"runvoy/internal/api"
)

// mockClientInterfaceForKill extends mockClientInterface with KillExecution
type mockClientInterfaceForKill struct {
	*mockClientInterface
	killExecutionFunc func(ctx context.Context, executionID string) (*api.KillExecutionResponse, error)
}

func (m *mockClientInterfaceForKill) KillExecution(
	ctx context.Context, executionID string,
) (*api.KillExecutionResponse, error) {
	if m.killExecutionFunc != nil {
		return m.killExecutionFunc(ctx, executionID)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForKill) GetLogStreamURL(_ context.Context, _ string) (*api.LogStreamResponse, error) {
	return &api.LogStreamResponse{}, nil
}

func TestKillService_KillExecution(t *testing.T) {
	tests := []struct {
		name         string
		executionID  string
		setupMock    func(*mockClientInterfaceForKill)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name:        "successfully kills execution",
			executionID: "exec-123",
			setupMock: func(m *mockClientInterfaceForKill) {
				m.killExecutionFunc = func(_ context.Context, executionID string) (*api.KillExecutionResponse, error) {
					assert.Equal(t, "exec-123", executionID)
					return &api.KillExecutionResponse{
						ExecutionID: "exec-123",
						Message:     "Execution terminated successfully",
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasSuccess := false
				hasExecID := false
				hasMessage := false
				for _, call := range m.calls {
					if call.method == "Successf" {
						hasSuccess = true
					}
					if call.method == "KeyValue" && len(call.args) >= 2 {
						if call.args[0] == "Execution ID" && call.args[1] == "exec-123" {
							hasExecID = true
						}
						if call.args[0] == "Message" && call.args[1] == "Execution terminated successfully" {
							hasMessage = true
						}
					}
				}
				assert.True(t, hasSuccess, "Expected Successf call")
				assert.True(t, hasExecID, "Expected Execution ID to be displayed")
				assert.True(t, hasMessage, "Expected Message to be displayed")
			},
		},
		{
			name:        "handles execution not found error",
			executionID: "exec-nonexistent",
			setupMock: func(m *mockClientInterfaceForKill) {
				m.killExecutionFunc = func(_ context.Context, _ string) (*api.KillExecutionResponse, error) {
					return nil, fmt.Errorf("execution not found")
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
			name:        "handles network error",
			executionID: "exec-456",
			setupMock: func(m *mockClientInterfaceForKill) {
				m.killExecutionFunc = func(_ context.Context, _ string) (*api.KillExecutionResponse, error) {
					return nil, fmt.Errorf("network error: connection timeout")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				// Service returns error, output.Errorf is called in killRun handler
				assert.Equal(t, 0, len(m.calls), "Service should not call output on error")
			},
		},
		{
			name:        "handles already completed execution",
			executionID: "exec-completed",
			setupMock: func(m *mockClientInterfaceForKill) {
				m.killExecutionFunc = func(_ context.Context, _ string) (*api.KillExecutionResponse, error) {
					return nil, fmt.Errorf("execution already completed")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				assert.Equal(t, 0, len(m.calls), "Service should not call output on error")
			},
		},
		{
			name:        "displays execution ID and message",
			executionID: "exec-789",
			setupMock: func(m *mockClientInterfaceForKill) {
				m.killExecutionFunc = func(_ context.Context, _ string) (*api.KillExecutionResponse, error) {
					return &api.KillExecutionResponse{
						ExecutionID: "exec-789",
						Message:     "Kill signal sent",
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasExecID := false
				hasMessage := false
				for _, call := range m.calls {
					if call.method == "KeyValue" && len(call.args) >= 2 {
						if call.args[0] == "Execution ID" {
							hasExecID = true
						}
						if call.args[0] == "Message" {
							hasMessage = true
						}
					}
				}
				assert.True(t, hasExecID, "Should display Execution ID")
				assert.True(t, hasMessage, "Should display Message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientInterfaceForKill{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewKillService(mockClient, mockOutput)

			err := service.KillExecution(context.Background(), tt.executionID)

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
