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

	tests := []struct {
		name            string
		executionID     string
		mockEvents      []api.LogEvent
		executionStatus string
		fetchLogsErr    error
		getExecutionErr error
		expectErr       bool
		expectedError   string
		shouldHaveWSURL bool
	}{
		{
			name:        "successful fetch with logs - running execution",
			executionID: "exec-123",
			mockEvents: []api.LogEvent{
				{
					Timestamp: now.Unix(),
					Message:   "Starting task",
				},
				{
					Timestamp: now.Add(1 * time.Second).Unix(),
					Message:   "Task completed",
				},
			},
			executionStatus: string(constants.ExecutionRunning),
			expectErr:       false,
			shouldHaveWSURL: true,
		},
		{
			name:            "successful fetch with logs - completed execution",
			executionID:     "exec-456",
			mockEvents:      []api.LogEvent{{Timestamp: now.Unix(), Message: "Task completed"}},
			executionStatus: string(constants.ExecutionSucceeded),
			expectErr:       false,
			shouldHaveWSURL: false,
		},
		{
			name:            "successful fetch with empty logs",
			executionID:     "exec-789",
			mockEvents:      []api.LogEvent{},
			executionStatus: string(constants.ExecutionRunning),
			expectErr:       false,
		},
		{
			name:          "empty execution ID",
			executionID:   "",
			expectErr:     true,
			expectedError: apperrors.ErrCodeInvalidRequest,
		},
		{
			name:          "execution not found",
			executionID:   "exec-not-found",
			expectErr:     true,
			expectedError: apperrors.ErrCodeNotFound,
		},
		{
			name:            "runner error",
			executionID:     "exec-111",
			executionStatus: string(constants.ExecutionRunning),
			fetchLogsErr:    errors.New("failed to fetch logs"),
			expectErr:       true,
			expectedError:   "failed to fetch logs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{
				fetchLogsByExecutionIDFunc: func(_ context.Context, _ string) ([]api.LogEvent, error) {
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
			resp, err := svc.GetLogsByExecutionID(ctx, tt.executionID, &email, &clientIP)

			if tt.expectErr {
				require.Error(t, err)
				if tt.expectedError == apperrors.ErrCodeInvalidRequest {
					assert.Equal(t, apperrors.ErrCodeInvalidRequest, apperrors.GetErrorCode(err))
				} else if tt.expectedError == apperrors.ErrCodeNotFound {
					assert.Equal(t, apperrors.ErrCodeNotFound, apperrors.GetErrorCode(err))
				} else if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tt.executionID, resp.ExecutionID)
				assert.Equal(t, tt.executionStatus, resp.Status)
				assert.Equal(t, len(tt.mockEvents), len(resp.Events))
				if len(tt.mockEvents) > 0 {
					assert.Equal(t, tt.mockEvents[0].Message, resp.Events[0].Message)
				}
				// WebSocket URL should only be present for RUNNING executions
				if tt.shouldHaveWSURL {
					// Note: Will be empty in test because websocketAPIBaseURL is ""
					assert.Equal(t, "", resp.WebSocketURL)
				}
			}
		})
	}
}

func TestGetLogsByExecutionID_WebSocketToken(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		executionID      string
		executionStatus  string
		websocketBaseURL string
		mockEvents       []api.LogEvent
		tokenCreateErr   error
		expectTokenInURL bool
		expectTokenRepo  bool
		expectErr        bool
	}{
		{
			name:             "running execution generates token",
			executionID:      "exec-123",
			executionStatus:  string(constants.ExecutionRunning),
			websocketBaseURL: "api.example.com/production",
			mockEvents:       []api.LogEvent{{Message: "test"}},
			expectTokenInURL: true,
			expectTokenRepo:  true,
			expectErr:        false,
		},
		{
			name:             "completed execution also generates token for late-joiners",
			executionID:      "exec-456",
			executionStatus:  string(constants.ExecutionSucceeded),
			websocketBaseURL: "api.example.com/production",
			mockEvents:       []api.LogEvent{{Message: "test"}},
			expectTokenInURL: true,
			expectTokenRepo:  true,
			expectErr:        false,
		},
		{
			name:             "execution without base URL does not generate token",
			executionID:      "exec-789",
			executionStatus:  string(constants.ExecutionRunning),
			websocketBaseURL: "",
			mockEvents:       []api.LogEvent{{Message: "test"}},
			expectTokenInURL: false,
			expectTokenRepo:  false,
			expectErr:        false,
		},
		{
			name:             "token creation failure is best-effort (logs still returned)",
			executionID:      "exec-999",
			executionStatus:  string(constants.ExecutionRunning),
			websocketBaseURL: "api.example.com/production",
			mockEvents:       []api.LogEvent{{Message: "test"}},
			tokenCreateErr:   errors.New("database error"),
			expectTokenInURL: false, // URL won't be in response due to error
			expectTokenRepo:  true,
			expectErr:        false,
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

			var capturedToken *api.WebSocketToken
			tokenRepo := &mockTokenRepository{
				createTokenFunc: func(_ context.Context, token *api.WebSocketToken) error {
					capturedToken = token
					return tt.tokenCreateErr
				},
			}

			svc := newTestService(nil, execRepo, runner)
			svc.tokenRepo = tokenRepo
			svc.websocketAPIBaseURL = tt.websocketBaseURL

			email := "test@example.com"
			clientIP := "192.168.1.1"
			resp, err := svc.GetLogsByExecutionID(ctx, tt.executionID, &email, &clientIP)

			if tt.expectErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)

			// Logs should always be returned
			assert.Equal(t, tt.mockEvents, resp.Events)

			if tt.expectTokenInURL {
				assert.NotEmpty(t, resp.WebSocketURL)
				assert.Contains(t, resp.WebSocketURL, "token=")
				assert.Contains(t, resp.WebSocketURL, "execution_id=")
				require.NotNil(t, capturedToken)
				assert.Equal(t, email, capturedToken.UserEmail)
				assert.Equal(t, clientIP, capturedToken.ClientIP)
				assert.Equal(t, tt.executionID, capturedToken.ExecutionID)
				assert.Contains(t, resp.WebSocketURL, capturedToken.Token)
			} else {
				// WebSocket URL may be empty (either due to no base URL or creation failure)
				// but logs should always be present
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
	tokenRepo := &mockTokenRepository{
		createTokenFunc: func(_ context.Context, token *api.WebSocketToken) error {
			// Track all tokens generated
			tokens[token.Token] = true
			return nil
		},
	}

	svc := newTestService(nil, execRepo, runner)
	svc.tokenRepo = tokenRepo
	svc.websocketAPIBaseURL = "api.example.com/production"

	// Call GetLogsByExecutionID multiple times and verify tokens are unique
	for range 3 {
		email := "test@example.com"
		clientIP := "10.0.0.1"
		resp, err := svc.GetLogsByExecutionID(ctx, execution.ExecutionID, &email, &clientIP)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.WebSocketURL)
	}

	// Should have 3 unique tokens
	assert.Equal(t, 3, len(tokens), "tokens should be unique across multiple calls")
}
