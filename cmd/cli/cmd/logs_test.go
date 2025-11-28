package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"runvoy/internal/api"
	"runvoy/internal/constants"
)

// mockClientInterfaceForLogs extends mockClientInterface with GetLogs
type mockClientInterfaceForLogs struct {
	*mockClientInterface
	getLogsFunc            func(ctx context.Context, executionID string) (*api.LogsResponse, error)
	getExecutionStatusFunc func(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error)
}

func (m *mockClientInterfaceForLogs) GetLogs(ctx context.Context, executionID string) (*api.LogsResponse, error) {
	if m.getLogsFunc != nil {
		return m.getLogsFunc(ctx, executionID)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForLogs) GetExecutionStatus(
	ctx context.Context,
	executionID string,
) (*api.ExecutionStatusResponse, error) {
	if m.getExecutionStatusFunc != nil {
		return m.getExecutionStatusFunc(ctx, executionID)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForLogs) FetchBackendLogs(_ context.Context, _ string) (*api.TraceResponse, error) {
	return nil, nil
}

func TestLogsService_DisplayLogs(t *testing.T) {
	tests := []struct {
		name             string
		executionID      string
		webURL           string
		setupMock        func(*mockClientInterfaceForLogs)
		configureService func(*testing.T, *LogsService)
		wantErr          bool
		verifyOutput     func(*testing.T, *mockOutputInterface)
	}{
		{
			name:        "successfully displays logs",
			executionID: "exec-123",
			webURL:      "https://logs.example.com",
			setupMock: func(m *mockClientInterfaceForLogs) {
				m.getLogsFunc = func(_ context.Context, _ string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: "exec-123",
						Status:      string(constants.ExecutionSucceeded),
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
				for _, call := range m.calls {
					if call.method == "Table" {
						hasTable = true
					}
				}
				assert.True(t, hasTable, "Expected Table call to display logs")
			},
		},
		{
			name:        "displays empty logs",
			executionID: "exec-456",
			webURL:      "https://logs.example.com",
			setupMock: func(m *mockClientInterfaceForLogs) {
				m.getLogsFunc = func(_ context.Context, _ string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: "exec-456",
						Status:      string(constants.ExecutionFailed),
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
			name:        "handles client error",
			executionID: "exec-789",
			webURL:      "https://logs.example.com",
			setupMock: func(m *mockClientInterfaceForLogs) {
				m.getLogsFunc = func(_ context.Context, _ string) (*api.LogsResponse, error) {
					return nil, fmt.Errorf("network error")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				for _, call := range m.calls {
					assert.NotEqual(t, "Table", call.method, "Should not display Table on error")
				}
			},
		},
		{
			name:        "displays all logs with duplicate timestamps",
			executionID: "exec-dup-ts",
			webURL:      "https://logs.example.com",
			setupMock: func(m *mockClientInterfaceForLogs) {
				m.getLogsFunc = func(_ context.Context, _ string) (*api.LogsResponse, error) {
					commonTimestamp := int64(1762984282442)
					return &api.LogsResponse{
						ExecutionID: "exec-dup-ts",
						Status:      string(constants.ExecutionSucceeded),
						Events: []api.LogEvent{
							{Timestamp: 1762984282441, Message: "### runvoy runner execution started"},
							{Timestamp: commonTimestamp, Message: "### Docker image => alpine:latest"},
							{Timestamp: commonTimestamp, Message: "### runvoy command => echo test"},
							{Timestamp: commonTimestamp, Message: "KEY1=value1"},
							{Timestamp: commonTimestamp, Message: "KEY2=value2"},
							{Timestamp: commonTimestamp, Message: "GITHUB_TOKEN=ghp_example1234567890"},
							{Timestamp: commonTimestamp, Message: "all done"},
						},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				var tableCall *call
				for i := range m.calls {
					if m.calls[i].method == "Table" {
						tableCall = &m.calls[i]
						break
					}
				}
				require.NotNil(t, tableCall, "Expected Table call to display logs")
				require.GreaterOrEqual(t, len(tableCall.args), 2, "Table call should have at least 2 args")
				rows, ok := tableCall.args[1].([][]string)
				require.True(t, ok, "Second arg should be [][]string")
				assert.Equal(t, 7, len(rows), "Should display all 7 log events even with duplicate timestamps")
			},
		},
		{
			name:        "streams logs when execution is running",
			executionID: "exec-stream",
			webURL:      "https://logs.example.com",
			setupMock: func(m *mockClientInterfaceForLogs) {
				m.getLogsFunc = func(_ context.Context, _ string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID:  "exec-stream",
						Status:       string(constants.ExecutionRunning),
						WebSocketURL: "wss://example.com/logs/exec-stream",
					}, nil
				}
			},
			configureService: func(t *testing.T, s *LogsService) {
				s.stream = func(websocketURL string, startingLineNumber int, webURL, executionID string) error {
					assert.Equal(t, "wss://example.com/logs/exec-stream", websocketURL)
					assert.Equal(t, 0, startingLineNumber)
					assert.Equal(t, "https://logs.example.com", webURL)
					assert.Equal(t, "exec-stream", executionID)
					return nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				var hasInfo bool
				for _, call := range m.calls {
					if call.method == "Infof" {
						hasInfo = true
						break
					}
				}
				assert.True(t, hasInfo, "Expected informational output before streaming")
			},
		},
		{
			name:        "errors when running execution lacks websocket URL",
			executionID: "exec-missing-ws",
			webURL:      "https://logs.example.com",
			setupMock: func(m *mockClientInterfaceForLogs) {
				m.getLogsFunc = func(_ context.Context, _ string) (*api.LogsResponse, error) {
					return &api.LogsResponse{
						ExecutionID: "exec-missing-ws",
						Status:      string(constants.ExecutionRunning),
					}, nil
				}
			},
			wantErr: true,
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
			if tt.configureService != nil {
				tt.configureService(t, service)
			}

			err := service.DisplayLogs(context.Background(), tt.executionID, tt.webURL)

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

func TestIsTerminalStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		status       string
		wantTerminal bool
	}{
		{status: "SUCCEEDED", wantTerminal: true},
		{status: "FAILED", wantTerminal: true},
		{status: "STOPPED", wantTerminal: true},
		{status: "TERMINATING", wantTerminal: true},
		{status: "RUNNING", wantTerminal: false},
		{status: "STARTING", wantTerminal: false},
		{status: "STARTED", wantTerminal: false},
		{status: "", wantTerminal: false},
	}

	for _, tc := range testCases {
		t.Run(tc.status, func(t *testing.T) {
			assert.Equal(t, tc.wantTerminal, isTerminalStatus(tc.status))
		})
	}
}
