package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommand(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		userEmail     string
		req           api.ExecutionRequest
		executionID   string
		createdAt     *time.Time
		startTaskErr  error
		createExecErr error
		expectErr     bool
		expectedError string
	}{
		{
			name:      "successful execution",
			userEmail: "user@example.com",
			req: api.ExecutionRequest{
				Command: "echo hello",
			},
			executionID: "exec-123",
			createdAt:   timePtr(time.Now()),
			expectErr:   false,
		},
		{
			name:      "successful execution without createdAt",
			userEmail: "user@example.com",
			req: api.ExecutionRequest{
				Command: "echo hello",
			},
			executionID: "exec-456",
			createdAt:   nil,
			expectErr:   false,
		},
		{
			name:      "successful execution with lock",
			userEmail: "user@example.com",
			req: api.ExecutionRequest{
				Command: "echo hello",
				Lock:    "my-lock",
			},
			executionID: "exec-789",
			expectErr:   false,
		},
		{
			name:          "empty command",
			userEmail:     "user@example.com",
			req:           api.ExecutionRequest{Command: ""},
			expectErr:     true,
			expectedError: apperrors.ErrCodeInvalidRequest,
		},
		{
			name:      "runner error",
			userEmail: "user@example.com",
			req: api.ExecutionRequest{
				Command: "echo hello",
			},
			startTaskErr:  errors.New("failed to start task"),
			expectErr:     true,
			expectedError: "failed to start task",
		},
		{
			name:      "execution creation fails but task started",
			userEmail: "user@example.com",
			req: api.ExecutionRequest{
				Command: "echo hello",
			},
			executionID:   "exec-999",
			createExecErr: errors.New("database error"),
			expectErr:     false, // Task started successfully, DB error logged but not returned
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{
				startTaskFunc: func(_ context.Context, _ string, _ *api.ExecutionRequest) (string, *time.Time, error) {
					return tt.executionID, tt.createdAt, tt.startTaskErr
				},
			}

			execRepo := &mockExecutionRepository{
				createExecutionFunc: func(_ context.Context, _ *api.Execution) error {
					return tt.createExecErr
				},
			}

			svc := newTestService(nil, execRepo, runner)
			resp, err := svc.RunCommand(ctx, tt.userEmail, &tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.expectedError != "" {
					if tt.expectedError == apperrors.ErrCodeInvalidRequest {
						assert.Equal(t, apperrors.ErrCodeInvalidRequest, apperrors.GetErrorCode(err))
					} else {
						assert.Contains(t, err.Error(), tt.expectedError)
					}
				}
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tt.executionID, resp.ExecutionID)
				assert.Equal(t, string(constants.ExecutionRunning), resp.Status)
			}
		})
	}
}

func TestGetExecutionStatus(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	exitCode := 0

	tests := []struct {
		name            string
		executionID     string
		mockExecution   *api.Execution
		getExecErr      error
		expectErr       bool
		expectedErrCode string
		expectExitCode  bool
	}{
		{
			name:        "successful - running execution",
			executionID: "exec-123",
			mockExecution: &api.Execution{
				ExecutionID: "exec-123",
				UserEmail:   "user@example.com",
				Command:     "echo hello",
				Status:      string(constants.ExecutionRunning),
				StartedAt:   now,
				CompletedAt: nil,
			},
			expectErr:      false,
			expectExitCode: false,
		},
		{
			name:        "successful - completed execution",
			executionID: "exec-456",
			mockExecution: &api.Execution{
				ExecutionID: "exec-456",
				UserEmail:   "user@example.com",
				Command:     "echo hello",
				Status:      string(constants.ExecutionSucceeded),
				StartedAt:   now,
				CompletedAt: timePtr(now.Add(5 * time.Second)),
				ExitCode:    exitCode,
			},
			expectErr:      false,
			expectExitCode: true,
		},
		{
			name:            "empty execution ID",
			executionID:     "",
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeInvalidRequest,
		},
		{
			name:            "execution not found",
			executionID:     "non-existent",
			mockExecution:   nil,
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeNotFound,
		},
		{
			name:        "repository error",
			executionID: "exec-789",
			getExecErr:  errors.New("database error"),
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execRepo := &mockExecutionRepository{
				getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
					return tt.mockExecution, tt.getExecErr
				},
			}

			svc := newTestService(nil, execRepo, nil)
			resp, err := svc.GetExecutionStatus(ctx, tt.executionID)

			if tt.expectErr {
				require.Error(t, err)
				if tt.expectedErrCode != "" {
					assert.Equal(t, tt.expectedErrCode, apperrors.GetErrorCode(err))
				}
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tt.mockExecution.ExecutionID, resp.ExecutionID)
				assert.Equal(t, tt.mockExecution.Status, resp.Status)
				assert.Equal(t, tt.mockExecution.StartedAt, resp.StartedAt)
				assert.Equal(t, tt.mockExecution.CompletedAt, resp.CompletedAt)

				if tt.expectExitCode {
					require.NotNil(t, resp.ExitCode)
					assert.Equal(t, tt.mockExecution.ExitCode, *resp.ExitCode)
				} else {
					assert.Nil(t, resp.ExitCode)
				}
			}
		})
	}
}

func TestListExecutions(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		name           string
		mockExecutions []*api.Execution
		listErr        error
		expectErr      bool
	}{
		{
			name: "successful with multiple executions",
			mockExecutions: []*api.Execution{
				{
					ExecutionID: "exec-1",
					UserEmail:   "user1@example.com",
					Command:     "echo hello",
					Status:      string(constants.ExecutionRunning),
					StartedAt:   now,
				},
				{
					ExecutionID: "exec-2",
					UserEmail:   "user2@example.com",
					Command:     "echo world",
					Status:      string(constants.ExecutionSucceeded),
					StartedAt:   now,
					CompletedAt: timePtr(now.Add(5 * time.Second)),
					ExitCode:    0,
				},
			},
			expectErr: false,
		},
		{
			name:           "successful with empty list",
			mockExecutions: []*api.Execution{},
			expectErr:      false,
		},
		{
			name:      "repository error",
			listErr:   errors.New("database connection failed"),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execRepo := &mockExecutionRepository{
				listExecutionsFunc: func(_ context.Context) ([]*api.Execution, error) {
					return tt.mockExecutions, tt.listErr
				},
			}

			svc := newTestService(nil, execRepo, nil)
			executions, err := svc.ListExecutions(ctx)

			if tt.expectErr {
				require.Error(t, err)
				assert.Nil(t, executions)
			} else {
				require.NoError(t, err)
				require.NotNil(t, executions)
				assert.Equal(t, len(tt.mockExecutions), len(executions))
				if len(tt.mockExecutions) > 0 {
					assert.Equal(t, tt.mockExecutions[0].ExecutionID, executions[0].ExecutionID)
				}
			}
		})
	}
}

func TestKillExecution(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		name            string
		executionID     string
		mockExecution   *api.Execution
		getExecErr      error
		killTaskErr     error
		expectErr       bool
		expectedErrCode string
	}{
		{
			name:        "successful kill",
			executionID: "exec-123",
			mockExecution: &api.Execution{
				ExecutionID: "exec-123",
				Status:      string(constants.ExecutionRunning),
				StartedAt:   now,
			},
			expectErr: false,
		},
		{
			name:            "empty execution ID",
			executionID:     "",
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeInvalidRequest,
		},
		{
			name:            "execution not found",
			executionID:     "non-existent",
			mockExecution:   nil,
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeNotFound,
		},
		{
			name:        "execution already succeeded",
			executionID: "exec-456",
			mockExecution: &api.Execution{
				ExecutionID: "exec-456",
				Status:      string(constants.ExecutionSucceeded),
				StartedAt:   now,
				CompletedAt: timePtr(now.Add(5 * time.Second)),
			},
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeInvalidRequest,
		},
		{
			name:        "execution already failed",
			executionID: "exec-789",
			mockExecution: &api.Execution{
				ExecutionID: "exec-789",
				Status:      string(constants.ExecutionFailed),
				StartedAt:   now,
				CompletedAt: timePtr(now.Add(3 * time.Second)),
			},
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeInvalidRequest,
		},
		{
			name:        "execution already stopped",
			executionID: "exec-999",
			mockExecution: &api.Execution{
				ExecutionID: "exec-999",
				Status:      string(constants.ExecutionStopped),
				StartedAt:   now,
				CompletedAt: timePtr(now.Add(2 * time.Second)),
			},
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeInvalidRequest,
		},
		{
			name:        "repository error on get",
			executionID: "exec-111",
			getExecErr:  errors.New("database error"),
			expectErr:   true,
		},
		{
			name:        "runner error on kill",
			executionID: "exec-222",
			mockExecution: &api.Execution{
				ExecutionID: "exec-222",
				Status:      string(constants.ExecutionRunning),
				StartedAt:   now,
			},
			killTaskErr: errors.New("failed to stop task"),
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execRepo := &mockExecutionRepository{
				getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
					return tt.mockExecution, tt.getExecErr
				},
			}

			runner := &mockRunner{
				killTaskFunc: func(_ context.Context, _ string) error {
					return tt.killTaskErr
				},
			}

			svc := newTestService(nil, execRepo, runner)
			err := svc.KillExecution(ctx, tt.executionID)

			if tt.expectErr {
				require.Error(t, err)
				if tt.expectedErrCode != "" {
					assert.Equal(t, tt.expectedErrCode, apperrors.GetErrorCode(err))
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetLogsByExecutionID(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	firstTimestamp := now.UnixMilli()
	secondTimestamp := firstTimestamp + 1000

	tests := []struct {
		name                  string
		executionID           string
		mockEvents            []api.LogEvent
		executionStatus       string
		fetchLogsErr          error
		getExecutionErr       error
		expectErr             bool
		expectedErrCode       string
		expectedErrorContains string
		lastSeenTimestamp     *int64
		expectedEvents        []api.LogEvent
		expectedWebSocketURL  string
		expectFetchCall       bool
	}{
		{
			name:                 "running execution returns websocket response only",
			executionID:          "exec-123",
			executionStatus:      string(constants.ExecutionRunning),
			expectedEvents:       nil,
			expectedWebSocketURL: "",
			expectFetchCall:      false,
		},
		{
			name:            "completed execution returns all logs",
			executionID:     "exec-456",
			executionStatus: string(constants.ExecutionSucceeded),
			mockEvents: []api.LogEvent{
				{Timestamp: firstTimestamp, Message: "Starting task"},
				{Timestamp: secondTimestamp, Message: "Task completed"},
			},
			expectedEvents: []api.LogEvent{
				{Timestamp: firstTimestamp, Message: "Starting task"},
				{Timestamp: secondTimestamp, Message: "Task completed"},
			},
			expectedWebSocketURL: "",
			expectFetchCall:      true,
		},
		{
			name:            "completed execution filters logs using last_seen_timestamp",
			executionID:     "exec-789",
			executionStatus: string(constants.ExecutionSucceeded),
			mockEvents: []api.LogEvent{
				{Timestamp: firstTimestamp, Message: "Starting task"},
				{Timestamp: secondTimestamp, Message: "Task completed"},
			},
			lastSeenTimestamp: &firstTimestamp,
			expectedEvents: []api.LogEvent{
				{Timestamp: secondTimestamp, Message: "Task completed"},
			},
			expectedWebSocketURL: "",
			expectFetchCall:      true,
		},
		{
			name:            "empty execution ID",
			executionID:     "",
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeInvalidRequest,
			expectFetchCall: false,
		},
		{
			name:            "execution not found",
			executionID:     "exec-not-found",
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeNotFound,
			expectFetchCall: false,
		},
		{
			name:                  "runner error",
			executionID:           "exec-111",
			executionStatus:       string(constants.ExecutionSucceeded),
			fetchLogsErr:          errors.New("failed to fetch logs"),
			expectErr:             true,
			expectedErrorContains: "failed to fetch logs",
			expectFetchCall:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fetchCalled bool
			runner := &mockRunner{
				fetchLogsByExecutionIDFunc: func(_ context.Context, _ string) ([]api.LogEvent, error) {
					fetchCalled = true
					return tt.mockEvents, tt.fetchLogsErr
				},
			}

			execRepo := &mockExecutionRepository{
				getExecutionFunc: func(_ context.Context, execID string) (*api.Execution, error) {
					if tt.getExecutionErr != nil {
						return nil, tt.getExecutionErr
					}
					// Only return execution if we have a status configured (i.e., it's a valid execution case)
					if tt.executionStatus != "" && execID == tt.executionID {
						return &api.Execution{
							ExecutionID: execID,
							Status:      tt.executionStatus,
							StartedAt:   now,
						}, nil
					}
					// Return nil to simulate execution not found
					return nil, nil
				},
			}

			svc := newTestService(nil, execRepo, runner)
			email := "test@example.com"
			clientIP := "127.0.0.1"
			resp, err := svc.GetLogsByExecutionID(ctx, tt.executionID, &email, &clientIP, tt.lastSeenTimestamp)

			if tt.expectErr {
				require.Error(t, err)
				if tt.expectedErrCode == apperrors.ErrCodeInvalidRequest {
					assert.Equal(t, apperrors.ErrCodeInvalidRequest, apperrors.GetErrorCode(err))
				} else if tt.expectedErrCode == apperrors.ErrCodeNotFound {
					assert.Equal(t, apperrors.ErrCodeNotFound, apperrors.GetErrorCode(err))
				} else if tt.expectedErrorContains != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorContains)
				}
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tt.executionID, resp.ExecutionID)
				assert.Equal(t, tt.executionStatus, resp.Status)
				assert.Equal(t, tt.expectedWebSocketURL, resp.WebSocketURL)
				if tt.expectedEvents == nil {
					assert.Len(t, resp.Events, 0)
				} else {
					require.Equal(t, len(tt.expectedEvents), len(resp.Events))
					assert.Equal(t, tt.expectedEvents, resp.Events)
				}
			}
			assert.Equal(t, tt.expectFetchCall, fetchCalled)
		})
	}
}

func TestGetLogsByExecutionID_WebSocketToken(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		executionID       string
		executionStatus   string
		websocketBaseURL  string
		mockEvents        []api.LogEvent
		connCreateErr     error
		expectedTokenLen  int
		expectTokenInURL  bool
		expectPendingConn bool
		expectErr         bool
	}{
		{
			name:              "running execution generates token and pending connection",
			executionID:       "exec-123",
			executionStatus:   string(constants.ExecutionRunning),
			websocketBaseURL:  "api.example.com/production",
			mockEvents:        []api.LogEvent{{Message: "test"}},
			expectedTokenLen:  43, // ~32 bytes base64 encoded
			expectTokenInURL:  true,
			expectPendingConn: true,
			expectErr:         false,
		},
		{
			name:             "completed execution does not generate token",
			executionID:      "exec-456",
			executionStatus:  string(constants.ExecutionSucceeded),
			websocketBaseURL: "api.example.com/production",
			mockEvents:       []api.LogEvent{{Message: "test"}},
			expectTokenInURL: false,
			expectErr:        false,
		},
		{
			name:             "running execution without base URL does not generate token",
			executionID:      "exec-789",
			executionStatus:  string(constants.ExecutionRunning),
			websocketBaseURL: "",
			mockEvents:       []api.LogEvent{{Message: "test"}},
			expectTokenInURL: false,
			expectErr:        false,
		},
		{
			name:              "pending connection creation failure",
			executionID:       "exec-999",
			executionStatus:   string(constants.ExecutionRunning),
			websocketBaseURL:  "api.example.com/production",
			mockEvents:        []api.LogEvent{{Message: "test"}},
			connCreateErr:     errors.New("database error"),
			expectPendingConn: true,
			expectErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execution := &api.Execution{
				ExecutionID: tt.executionID,
				Status:      tt.executionStatus,
			}

			execRepo := &mockExecutionRepository{
				getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
					return execution, nil
				},
			}

			runner := &mockRunner{
				fetchLogsByExecutionIDFunc: func(_ context.Context, _ string) ([]api.LogEvent, error) {
					return tt.mockEvents, nil
				},
			}

			var capturedPendingConn *api.WebSocketConnection
			connRepo := &mockConnectionRepository{
				createConnectionFunc: func(_ context.Context, conn *api.WebSocketConnection) error {
					if tt.expectPendingConn {
						capturedPendingConn = conn
					}
					return tt.connCreateErr
				},
			}

			svc := newTestServiceWithConnRepo(nil, execRepo, connRepo, runner)
			svc.websocketAPIBaseURL = tt.websocketBaseURL

			email := "test@example.com"
			clientIP := "192.168.1.1"
			resp, err := svc.GetLogsByExecutionID(ctx, tt.executionID, &email, &clientIP, nil)

			if tt.expectErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)

			if tt.expectTokenInURL {
				assert.NotEmpty(t, resp.WebSocketURL)
				assert.Contains(t, resp.WebSocketURL, "token=")
				assert.Contains(t, resp.WebSocketURL, "execution_id=")

				// Verify pending connection was created
				require.NotNil(t, capturedPendingConn)
				assert.Equal(t, tt.executionID, capturedPendingConn.ExecutionID)
				assert.Contains(t, capturedPendingConn.ConnectionID, "pending_")
				assert.NotEmpty(t, capturedPendingConn.Token)
				assert.Equal(t, constants.FunctionalityLogStreaming, capturedPendingConn.Functionality)

				// Verify token in URL matches pending connection
				assert.Contains(t, resp.WebSocketURL, capturedPendingConn.Token)
			} else {
				assert.Empty(t, resp.WebSocketURL)
			}
		})
	}
}

func TestGetLogsByExecutionID_TokenUniqueness(t *testing.T) {
	ctx := context.Background()

	execution := &api.Execution{
		ExecutionID: "exec-123",
		Status:      string(constants.ExecutionRunning),
	}

	execRepo := &mockExecutionRepository{
		getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
			return execution, nil
		},
	}

	runner := &mockRunner{
		fetchLogsByExecutionIDFunc: func(_ context.Context, _ string) ([]api.LogEvent, error) {
			return []api.LogEvent{{Message: "test"}}, nil
		},
	}

	tokens := make(map[string]bool)
	connRepo := &mockConnectionRepository{
		createConnectionFunc: func(_ context.Context, conn *api.WebSocketConnection) error {
			// Track all tokens generated
			tokens[conn.Token] = true
			return nil
		},
	}

	svc := newTestServiceWithConnRepo(nil, execRepo, connRepo, runner)
	svc.websocketAPIBaseURL = "api.example.com/production"

	// Call GetLogsByExecutionID multiple times and verify tokens are unique
	for range 3 {
		email := "test@example.com"
		clientIP := "10.0.0.1"
		resp, err := svc.GetLogsByExecutionID(ctx, execution.ExecutionID, &email, &clientIP, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.WebSocketURL)
	}

	// Should have 3 unique tokens
	assert.Equal(t, 3, len(tokens), "tokens should be unique across multiple calls")
}
