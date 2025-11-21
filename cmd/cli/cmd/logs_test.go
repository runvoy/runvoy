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

// mockSleeper is a test implementation that records sleep calls without actually sleeping
type mockSleeper struct {
	sleepCalls []time.Duration
}

func (m *mockSleeper) Sleep(duration time.Duration) {
	m.sleepCalls = append(m.sleepCalls, duration)
	// Don't actually sleep to keep tests fast
}

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

func (m *mockClientInterfaceForLogs) FetchBackendLogs(_ context.Context, _ string) ([]api.LogEvent, error) {
	return nil, nil
}

func TestLogsService_DisplayLogs(t *testing.T) {
	tests := []struct {
		name         string
		executionID  string
		webURL       string
		setupMock    func(*mockClientInterfaceForLogs)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name:        "successfully displays logs",
			executionID: "exec-123",
			webURL:      "https://logs.example.com",
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
				// Should not have any Table calls when there's an error
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
					// Simulate the bug scenario: multiple events with the same timestamp
					commonTimestamp := int64(1762984282442)
					return &api.LogsResponse{
						ExecutionID: "exec-dup-ts",
						Events: []api.LogEvent{
							{Timestamp: 1762984282441, Message: "### runvoy runner execution started"},
							{Timestamp: commonTimestamp, Message: "### Docker image => alpine:latest"},
							{Timestamp: commonTimestamp, Message: "### runvoy command => echo test"},
							{Timestamp: commonTimestamp, Message: "KEY1=value1"},
							{Timestamp: commonTimestamp, Message: "KEY2=value2"},
							{Timestamp: commonTimestamp, Message: "GITHUB_TOKEN=ghp_example1234567890"},
							{Timestamp: commonTimestamp, Message: "all done"},
						},
						Status: "SUCCEEDED",
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
				// Should display all 7 log events, not just 2
				assert.Equal(t, 7, len(rows), "Should display all 7 log events even with duplicate timestamps")
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
			mockSleeper := &mockSleeper{}
			service := NewLogsService(mockClient, mockOutput, mockSleeper)

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

func TestLogsService_SmartPolling_StartingState(t *testing.T) {
	tests := []struct {
		name              string
		executionStatus   string
		expectedSleepTime time.Duration
		logsError         error
	}{
		{
			name:              "waits 30 seconds for STARTING state",
			executionStatus:   "STARTING",
			expectedSleepTime: 30 * time.Second,
			logsError:         nil,
		},
		{
			name:              "waits 10 seconds for TERMINATING state",
			executionStatus:   "TERMINATING",
			expectedSleepTime: 10 * time.Second,
			logsError:         nil,
		},
		{
			name:              "no wait for RUNNING state",
			executionStatus:   "RUNNING",
			expectedSleepTime: 0,
			logsError:         nil,
		},
		{
			name:              "no wait for SUCCEEDED state",
			executionStatus:   "SUCCEEDED",
			expectedSleepTime: 0,
			logsError:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientInterfaceForLogs{
				mockClientInterface: &mockClientInterface{},
			}

			// Setup GetExecutionStatus mock
			mockClient.getExecutionStatusFunc = func(_ context.Context, _ string) (*api.ExecutionStatusResponse, error) {
				return &api.ExecutionStatusResponse{
					ExecutionID: "exec-123",
					Status:      tt.executionStatus,
				}, nil
			}

			// Setup GetLogs mock
			mockClient.getLogsFunc = func(_ context.Context, _ string) (*api.LogsResponse, error) {
				if tt.logsError != nil {
					return nil, tt.logsError
				}
				return &api.LogsResponse{
					ExecutionID: "exec-123",
					Events:      []api.LogEvent{{Timestamp: 1000000, Message: "Test log"}},
					Status:      tt.executionStatus,
				}, nil
			}

			mockOutput := &mockOutputInterface{}
			mockSleeper := &mockSleeper{}
			service := NewLogsService(mockClient, mockOutput, mockSleeper)

			_ = service.DisplayLogs(context.Background(), "exec-123", "https://example.com")

			// Verify sleep behavior
			if tt.expectedSleepTime > 0 {
				require.Len(t, mockSleeper.sleepCalls, 1, "Expected one sleep call")
				assert.Equal(t, tt.expectedSleepTime, mockSleeper.sleepCalls[0], "Sleep duration should match expected value")
			} else {
				assert.Empty(t, mockSleeper.sleepCalls, "Should not sleep for non-STARTING/TERMINATING states")
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
