package cmd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"runvoy/internal/api"
)

// mockClientInterfaceForList extends mockClientInterface with ListExecutions
type mockClientInterfaceForList struct {
	*mockClientInterface
	listExecutionsFunc func(ctx context.Context) ([]api.Execution, error)
}

func (m *mockClientInterfaceForList) ListExecutions(ctx context.Context) ([]api.Execution, error) {
	if m.listExecutionsFunc != nil {
		return m.listExecutionsFunc(ctx)
	}
	return nil, fmt.Errorf("not implemented")
}

func TestListService_ListExecutions(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(*mockClientInterfaceForList)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name: "successfully lists executions",
			setupMock: func(m *mockClientInterfaceForList) {
				m.listExecutionsFunc = func(_ context.Context) ([]api.Execution, error) {
					now := time.Now()
					return []api.Execution{
						{
							ExecutionID:     "exec-1",
							Status:          "completed",
							Command:         "echo hello",
							UserEmail:       "user@example.com",
							StartedAt:       now,
							CompletedAt:     func() *time.Time { t := now.Add(5 * time.Second); return &t }(),
							DurationSeconds: 5,
						},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasInfof := false
				hasTable := false
				hasSuccess := false
				for _, call := range m.calls {
					if call.method == "Infof" {
						hasInfof = true
					}
					if call.method == "Table" {
						hasTable = true
						// Verify table headers
						if len(call.args) >= 1 {
							headers := call.args[0].([]string)
							assert.Contains(t, headers, "Execution ID")
							assert.Contains(t, headers, "Status")
							assert.Contains(t, headers, "Command")
						}
					}
					if call.method == "Successf" {
						hasSuccess = true
					}
				}
				assert.True(t, hasInfof, "Expected Infof call")
				assert.True(t, hasTable, "Expected Table call")
				assert.True(t, hasSuccess, "Expected Successf call")
			},
		},
		{
			name: "handles empty execution list",
			setupMock: func(m *mockClientInterfaceForList) {
				m.listExecutionsFunc = func(_ context.Context) ([]api.Execution, error) {
					return []api.Execution{}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasTable := false
				for _, call := range m.calls {
					if call.method == "Table" {
						hasTable = true
						// Verify table is called with empty rows
						if len(call.args) >= 2 {
							rows := call.args[1].([][]string)
							assert.Equal(t, 0, len(rows), "Should have empty rows")
						}
					}
				}
				assert.True(t, hasTable, "Expected Table call even with empty list")
			},
		},
		{
			name: "handles client error",
			setupMock: func(m *mockClientInterfaceForList) {
				m.listExecutionsFunc = func(_ context.Context) ([]api.Execution, error) {
					return nil, fmt.Errorf("network error")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				// Should have Infof call but not Table or Successf
				hasInfof := false
				hasTable := false
				for _, call := range m.calls {
					if call.method == "Infof" {
						hasInfof = true
					}
					if call.method == "Table" {
						hasTable = true
					}
				}
				assert.True(t, hasInfof, "Expected Infof call")
				assert.False(t, hasTable, "Should not call Table on error")
			},
		},
		{
			name: "formats long commands correctly",
			setupMock: func(m *mockClientInterfaceForList) {
				m.listExecutionsFunc = func(_ context.Context) ([]api.Execution, error) {
					longCommand := "this is a very long command that exceeds the maximum command length limit and should be truncated"
					return []api.Execution{
						{
							ExecutionID: "exec-long",
							Status:      "running",
							Command:     longCommand,
							UserEmail:   "user@example.com",
							StartedAt:   time.Now(),
						},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				for _, call := range m.calls {
					if call.method == "Table" && len(call.args) >= 2 {
						rows := call.args[1].([][]string)
						if len(rows) > 0 && len(rows[0]) > 2 {
							command := rows[0][2]                                                                  // Command column
							assert.LessOrEqual(t, len(command), maxCommandLength+3, "Command should be truncated") // +3 for "..."
							assert.Contains(t, command, "...", "Long command should end with ...")
						}
					}
				}
			},
		},
		{
			name: "formats executions with completed and duration",
			setupMock: func(m *mockClientInterfaceForList) {
				m.listExecutionsFunc = func(_ context.Context) ([]api.Execution, error) {
					started := time.Now().Add(-10 * time.Minute)
					completed := time.Now()
					return []api.Execution{
						{
							ExecutionID:     "exec-completed",
							Status:          "completed",
							Command:         "test command",
							UserEmail:       "user@example.com",
							StartedAt:       started,
							CompletedAt:     &completed,
							DurationSeconds: 600,
						},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				for _, call := range m.calls {
					if call.method == "Table" && len(call.args) >= 2 {
						rows := call.args[1].([][]string)
						if len(rows) > 0 && len(rows[0]) >= 7 {
							completed := rows[0][5] // Completed (UTC) column
							duration := rows[0][6]  // Duration column
							assert.NotEmpty(t, completed, "Completed time should be set")
							assert.NotEmpty(t, duration, "Duration should be set")
							assert.Contains(t, duration, "s", "Duration should include seconds")
						}
					}
				}
			},
		},
		{
			name: "formats executions without completed time",
			setupMock: func(m *mockClientInterfaceForList) {
				m.listExecutionsFunc = func(_ context.Context) ([]api.Execution, error) {
					return []api.Execution{
						{
							ExecutionID: "exec-running",
							Status:      "running",
							Command:     "running command",
							UserEmail:   "user@example.com",
							StartedAt:   time.Now(),
							CompletedAt: nil,
						},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				for _, call := range m.calls {
					if call.method == "Table" && len(call.args) >= 2 {
						rows := call.args[1].([][]string)
						if len(rows) > 0 && len(rows[0]) >= 6 {
							completed := rows[0][5] // Completed (UTC) column
							assert.Empty(t, completed, "Completed time should be empty for running execution")
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientInterfaceForList{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewListService(mockClient, mockOutput)

			err := service.ListExecutions(context.Background())

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
