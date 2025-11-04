package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"runvoy/internal/api"
)

// mockClientInterfaceForLogs extends mockClientInterface with GetLogs
type mockClientInterfaceForLogs struct {
	*mockClientInterface
	getLogsFunc func(ctx context.Context, executionID string) (*api.LogsResponse, error)
}

func (m *mockClientInterfaceForLogs) GetLogs(ctx context.Context, executionID string) (*api.LogsResponse, error) {
	if m.getLogsFunc != nil {
		return m.getLogsFunc(ctx, executionID)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForLogs) GetLogStreamURL(_ context.Context, _ string) (*api.LogStreamResponse, error) {
	return &api.LogStreamResponse{}, nil
}

func TestLogsService_DisplayLogs(t *testing.T) {
	tests := []struct {
		name         string
		executionID  string
		webviewerURL string
		setupMock    func(*mockClientInterfaceForLogs)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name:         "successfully displays logs",
			executionID:  "exec-123",
			webviewerURL: "https://logs.example.com",
			setupMock: func(m *mockClientInterfaceForLogs) {
				m.getLogsFunc = func(_ context.Context, _ string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: "exec-123",
						Events: []api.LogEvent{
							{Timestamp: 1000000, Message: "Starting process"},
							{Timestamp: 2000000, Message: "Process completed"},
						},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				require.Greater(t, len(m.calls), 0)
				hasTable := false
				hasWarning := false
				for _, call := range m.calls {
					if call.method == "Table" {
						hasTable = true
					}
					if call.method == "Warningf" {
						hasWarning = true
					}
				}
				assert.True(t, hasTable, "Expected Table call to display logs")
				// Since WebSocket URL is empty in mock, we expect a warning instead of success
				assert.True(t, hasWarning, "Expected Warningf call when WebSocket not configured")
			},
		},
		{
			name:         "displays empty logs",
			executionID:  "exec-456",
			webviewerURL: "https://logs.example.com",
			setupMock: func(m *mockClientInterfaceForLogs) {
				m.getLogsFunc = func(_ context.Context, _ string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: "exec-456",
						Events:      []api.LogEvent{},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				var hasTable bool
				for _, call := range m.calls {
					if call.method == "Table" {
						hasTable = true
						if len(call.args) >= 2 {
							rows, ok := call.args[1].([][]string)
							if ok {
								assert.Empty(t, rows, "Should have no rows for empty logs")
							}
						}
					}
				}
				assert.True(t, hasTable, "Should still call Table even with empty logs")
			},
		},
		{
			name:         "handles client error",
			executionID:  "exec-789",
			webviewerURL: "https://logs.example.com",
			setupMock: func(m *mockClientInterfaceForLogs) {
				m.getLogsFunc = func(_ context.Context, _ string) (*api.LogsResponse, error) {
					return nil, fmt.Errorf("network error")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				// Should not have any Table calls when there's an error
				for _, call := range m.calls {
					assert.NotEqual(t, "Table", call.method, "Should not display Table on error")
				}
			},
		},
		{
			name:         "displays webviewer URL",
			executionID:  "exec-abc",
			webviewerURL: "https://custom-viewer.com",
			setupMock: func(m *mockClientInterfaceForLogs) {
				m.getLogsFunc = func(_ context.Context, _ string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: "exec-abc",
						Events:      []api.LogEvent{{Timestamp: 1000000, Message: "test"}},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasInfof := false
				for _, call := range m.calls {
					if call.method == "Infof" && len(call.args) >= 1 {
						format, ok := call.args[0].(string)
						if ok {
							if fmt.Sprintf(format, call.args[1:]...) != "" {
								hasInfof = true
							}
						}
					}
				}
				assert.True(t, hasInfof, "Should display webviewer URL via Infof")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientInterfaceForLogs{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewLogsService(mockClient, mockOutput)

			err := service.DisplayLogs(context.Background(), tt.executionID, tt.webviewerURL)

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
