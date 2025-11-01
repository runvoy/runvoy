package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/testutil"
)

// MockRunner is a mock implementation of Runner interface
type MockRunner struct {
	mock.Mock
}

func (m *MockRunner) StartTask(ctx context.Context, userEmail string, req api.ExecutionRequest) (executionID string, taskARN string, createdAt *time.Time, err error) {
	args := m.Called(ctx, userEmail, req)
	return args.String(0), args.String(1), args.Get(2).(*time.Time), args.Error(3)
}

func (m *MockRunner) KillTask(ctx context.Context, executionID string) error {
	args := m.Called(ctx, executionID)
	return args.Error(0)
}

// MockUserRepository is a mock implementation of UserRepository interface
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) CreateUser(ctx context.Context, user *api.User, apiKeyHash string) error {
	args := m.Called(ctx, user, apiKeyHash)
	return args.Error(0)
}

func (m *MockUserRepository) GetUserByEmail(ctx context.Context, email string) (*api.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.User), args.Error(1)
}

func (m *MockUserRepository) GetUserByAPIKeyHash(ctx context.Context, apiKeyHash string) (*api.User, error) {
	args := m.Called(ctx, apiKeyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.User), args.Error(1)
}

func (m *MockUserRepository) UpdateLastUsed(ctx context.Context, email string) (*time.Time, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockUserRepository) RevokeUser(ctx context.Context, email string) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

// MockExecutionRepository is a mock implementation of ExecutionRepository interface
type MockExecutionRepository struct {
	mock.Mock
}

func (m *MockExecutionRepository) CreateExecution(ctx context.Context, execution *api.Execution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
}

func (m *MockExecutionRepository) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	args := m.Called(ctx, executionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.Execution), args.Error(1)
}

func (m *MockExecutionRepository) UpdateExecution(ctx context.Context, execution *api.Execution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
}

func (m *MockExecutionRepository) ListExecutions(ctx context.Context) ([]*api.Execution, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*api.Execution), args.Error(1)
}

func TestNewService(t *testing.T) {
	tests := []struct {
		name         string
		userRepo     *MockUserRepository
		executionRepo *MockExecutionRepository
		runner       *MockRunner
		logger       bool
		provider     constants.BackendProvider
	}{
		{
			name:         "creates service with all dependencies",
			userRepo:     new(MockUserRepository),
			executionRepo: new(MockExecutionRepository),
			runner:       new(MockRunner),
			logger:       true,
			provider:     constants.AWS,
		},
		{
			name:         "creates service with nil userRepo",
			userRepo:     nil,
			executionRepo: new(MockExecutionRepository),
			runner:       new(MockRunner),
			logger:       true,
			provider:     constants.AWS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := testutil.SilentLogger()
			if !tt.logger {
				logger = nil
			}

			svc := NewService(tt.userRepo, tt.executionRepo, tt.runner, logger, tt.provider)

			assert.NotNil(t, svc)
			assert.Equal(t, tt.provider, svc.Provider)
		})
	}
}

func TestService_CreateUser(t *testing.T) {
	tests := []struct {
		name        string
		req         api.CreateUserRequest
		setupMocks  func(*MockUserRepository)
		wantErr     bool
		checkResult func(*testing.T, *api.CreateUserResponse)
	}{
		{
			name: "successfully creates user with generated API key",
			req: api.CreateUserRequest{
				Email: "test@example.com",
			},
			setupMocks: func(m *MockUserRepository) {
				m.On("GetUserByEmail", mock.Anything, "test@example.com").Return(nil, nil)
				m.On("CreateUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
			checkResult: func(t *testing.T, resp *api.CreateUserResponse) {
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.User)
				assert.Equal(t, "test@example.com", resp.User.Email)
				assert.NotEmpty(t, resp.APIKey)
				assert.False(t, resp.User.Revoked)
			},
		},
		{
			name: "successfully creates user with provided API key",
			req: api.CreateUserRequest{
				Email:  "test@example.com",
				APIKey: "custom-key-123",
			},
			setupMocks: func(m *MockUserRepository) {
				m.On("GetUserByEmail", mock.Anything, "test@example.com").Return(nil, nil)
				m.On("CreateUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
			checkResult: func(t *testing.T, resp *api.CreateUserResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, "custom-key-123", resp.APIKey)
			},
		},
		{
			name: "returns error when userRepo is nil",
			req: api.CreateUserRequest{
				Email: "test@example.com",
			},
			setupMocks:  nil, // No mocks needed when repo is nil
			wantErr:     true,
			checkResult: nil,
		},
		{
			name: "returns error when email is empty",
			req: api.CreateUserRequest{
				Email: "",
			},
			setupMocks:  nil, // Returns early before calling repo
			wantErr:     true,
			checkResult: nil,
		},
		{
			name: "returns error when email is invalid",
			req: api.CreateUserRequest{
				Email: "invalid-email",
			},
			setupMocks:  nil, // Returns early before calling repo
			wantErr:     true,
			checkResult: nil,
		},
		{
			name: "returns error when user already exists",
			req: api.CreateUserRequest{
				Email: "existing@example.com",
			},
			setupMocks: func(m *MockUserRepository) {
				existingUser := testutil.NewUserBuilder().WithEmail("existing@example.com").Build()
				m.On("GetUserByEmail", mock.Anything, "existing@example.com").Return(existingUser, nil)
			},
			wantErr:     true,
			checkResult: nil,
		},
		{
			name: "returns error when GetUserByEmail fails",
			req: api.CreateUserRequest{
				Email: "test@example.com",
			},
			setupMocks: func(m *MockUserRepository) {
				m.On("GetUserByEmail", mock.Anything, "test@example.com").
					Return(nil, apperrors.ErrDatabaseError("database error", nil))
			},
			wantErr:     true,
			checkResult: nil,
		},
		{
			name: "returns error when CreateUser fails",
			req: api.CreateUserRequest{
				Email: "test@example.com",
			},
			setupMocks: func(m *MockUserRepository) {
				m.On("GetUserByEmail", mock.Anything, "test@example.com").Return(nil, nil)
				m.On("CreateUser", mock.Anything, mock.Anything, mock.Anything).
					Return(apperrors.ErrDatabaseError("failed to create user", nil))
			},
			wantErr:     true,
			checkResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockUserRepo *MockUserRepository
			if tt.setupMocks != nil {
				mockUserRepo = new(MockUserRepository)
				tt.setupMocks(mockUserRepo)
			}

			svc := NewService(mockUserRepo, nil, nil, testutil.SilentLogger(), constants.AWS)

			resp, err := svc.CreateUser(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.checkResult != nil {
					tt.checkResult(t, resp)
				}
			}

			if mockUserRepo != nil {
				mockUserRepo.AssertExpectations(t)
			}
		})
	}
}

func TestService_AuthenticateUser(t *testing.T) {
	tests := []struct {
		name        string
		apiKey      string
		setupMocks  func(*MockUserRepository)
		wantErr     bool
		checkResult func(*testing.T, *api.User)
	}{
		{
			name:   "successfully authenticates valid API key",
			apiKey: "valid-api-key",
			setupMocks: func(m *MockUserRepository) {
				user := testutil.NewUserBuilder().WithEmail("test@example.com").Build()
				m.On("GetUserByAPIKeyHash", mock.Anything, mock.Anything).Return(user, nil)
			},
			wantErr: false,
			checkResult: func(t *testing.T, user *api.User) {
				assert.NotNil(t, user)
				assert.Equal(t, "test@example.com", user.Email)
				assert.False(t, user.Revoked)
			},
		},
		{
			name:       "returns error when userRepo is nil",
			apiKey:     "test-key",
			setupMocks: nil, // No repo needed
			wantErr:    true,
		},
		{
			name:       "returns error when API key is empty",
			apiKey:     "",
			setupMocks: nil, // Returns early before calling repo
			wantErr:    true,
		},
		{
			name:   "returns error when API key is invalid",
			apiKey: "invalid-key",
			setupMocks: func(m *MockUserRepository) {
				m.On("GetUserByAPIKeyHash", mock.Anything, mock.Anything).Return(nil, nil)
			},
			wantErr: true,
		},
		{
			name:   "returns error when API key is revoked",
			apiKey: "revoked-key",
			setupMocks: func(m *MockUserRepository) {
				user := testutil.NewUserBuilder().WithEmail("test@example.com").Revoked().Build()
				m.On("GetUserByAPIKeyHash", mock.Anything, mock.Anything).Return(user, nil)
			},
			wantErr: true,
		},
		{
			name:   "returns error when GetUserByAPIKeyHash fails",
			apiKey: "test-key",
			setupMocks: func(m *MockUserRepository) {
				m.On("GetUserByAPIKeyHash", mock.Anything, mock.Anything).
					Return(nil, apperrors.ErrDatabaseError("database error", nil))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockUserRepo *MockUserRepository
			if tt.setupMocks != nil {
				mockUserRepo = new(MockUserRepository)
				tt.setupMocks(mockUserRepo)
			}

			svc := NewService(mockUserRepo, nil, nil, testutil.SilentLogger(), constants.AWS)

			user, err := svc.AuthenticateUser(context.Background(), tt.apiKey)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
				if tt.checkResult != nil {
					tt.checkResult(t, user)
				}
			}

			if mockUserRepo != nil {
				mockUserRepo.AssertExpectations(t)
			}
		})
	}
}

func TestService_UpdateUserLastUsed(t *testing.T) {
	tests := []struct {
		name       string
		email      string
		setupMocks func(*MockUserRepository)
		wantErr    bool
	}{
		{
			name:  "successfully updates last used timestamp",
			email: "test@example.com",
			setupMocks: func(m *MockUserRepository) {
				now := time.Now()
				m.On("UpdateLastUsed", mock.Anything, "test@example.com").Return(&now, nil)
			},
			wantErr: false,
		},
		{
			name:       "returns error when userRepo is nil",
			email:      "test@example.com",
			setupMocks: nil, // No repo needed
			wantErr:    true,
		},
		{
			name:       "returns error when email is empty",
			email:      "",
			setupMocks: nil, // Returns early before calling repo
			wantErr:    true,
		},
		{
			name:  "returns error when UpdateLastUsed fails",
			email: "test@example.com",
			setupMocks: func(m *MockUserRepository) {
				m.On("UpdateLastUsed", mock.Anything, "test@example.com").
					Return(nil, apperrors.ErrDatabaseError("database error", nil))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockUserRepo *MockUserRepository
			if tt.setupMocks != nil {
				mockUserRepo = new(MockUserRepository)
				tt.setupMocks(mockUserRepo)
			}

			svc := NewService(mockUserRepo, nil, nil, testutil.SilentLogger(), constants.AWS)

			lastUsed, err := svc.UpdateUserLastUsed(context.Background(), tt.email)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, lastUsed)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, lastUsed)
			}

			if mockUserRepo != nil {
				mockUserRepo.AssertExpectations(t)
			}
		})
	}
}

func TestService_RevokeUser(t *testing.T) {
	tests := []struct {
		name       string
		email      string
		setupMocks func(*MockUserRepository)
		wantErr    bool
	}{
		{
			name:  "successfully revokes user",
			email: "test@example.com",
			setupMocks: func(m *MockUserRepository) {
				user := testutil.NewUserBuilder().WithEmail("test@example.com").Build()
				m.On("GetUserByEmail", mock.Anything, "test@example.com").Return(user, nil)
				m.On("RevokeUser", mock.Anything, "test@example.com").Return(nil)
			},
			wantErr: false,
		},
		{
			name:       "returns error when userRepo is nil",
			email:      "test@example.com",
			setupMocks: nil, // No repo needed
			wantErr:    true,
		},
		{
			name:       "returns error when email is empty",
			email:      "",
			setupMocks: nil, // Returns early before calling repo
			wantErr:    true,
		},
		{
			name:  "returns error when user not found",
			email: "notfound@example.com",
			setupMocks: func(m *MockUserRepository) {
				m.On("GetUserByEmail", mock.Anything, "notfound@example.com").Return(nil, nil)
			},
			wantErr: true,
		},
		{
			name:  "returns error when GetUserByEmail fails",
			email: "test@example.com",
			setupMocks: func(m *MockUserRepository) {
				m.On("GetUserByEmail", mock.Anything, "test@example.com").
					Return(nil, apperrors.ErrDatabaseError("database error", nil))
			},
			wantErr: true,
		},
		{
			name:  "returns error when RevokeUser fails",
			email: "test@example.com",
			setupMocks: func(m *MockUserRepository) {
				user := testutil.NewUserBuilder().WithEmail("test@example.com").Build()
				m.On("GetUserByEmail", mock.Anything, "test@example.com").Return(user, nil)
				m.On("RevokeUser", mock.Anything, "test@example.com").
					Return(apperrors.ErrDatabaseError("database error", nil))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockUserRepo *MockUserRepository
			if tt.setupMocks != nil {
				mockUserRepo = new(MockUserRepository)
				tt.setupMocks(mockUserRepo)
			}

			svc := NewService(mockUserRepo, nil, nil, testutil.SilentLogger(), constants.AWS)

			err := svc.RevokeUser(context.Background(), tt.email)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if mockUserRepo != nil {
				mockUserRepo.AssertExpectations(t)
			}
		})
	}
}

func TestService_RunCommand(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name        string
		userEmail   string
		req         api.ExecutionRequest
		setupMocks  func(*MockRunner, *MockExecutionRepository)
		wantErr     bool
		checkResult func(*testing.T, *api.ExecutionResponse)
	}{
		{
			name:      "successfully runs command",
			userEmail: "test@example.com",
			req: api.ExecutionRequest{
				Command: "echo hello",
			},
			setupMocks: func(r *MockRunner, e *MockExecutionRepository) {
				r.On("StartTask", mock.Anything, "test@example.com", mock.Anything).
					Return("exec-123", "arn:aws:ecs:us-east-1:123456789012:task/cluster/task-id", &now, nil)
				e.On("CreateExecution", mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
			checkResult: func(t *testing.T, resp *api.ExecutionResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, "exec-123", resp.ExecutionID)
				assert.Equal(t, string(constants.ExecutionRunning), resp.Status)
			},
		},
		{
			name:      "returns error when executionRepo is nil",
			userEmail: "test@example.com",
			req: api.ExecutionRequest{
				Command: "echo hello",
			},
			setupMocks:  func(r *MockRunner, e *MockExecutionRepository) {},
			wantErr:     true,
			checkResult: nil,
		},
		{
			name:      "returns error when command is empty",
			userEmail: "test@example.com",
			req: api.ExecutionRequest{
				Command: "",
			},
			setupMocks:  func(r *MockRunner, e *MockExecutionRepository) {},
			wantErr:     true,
			checkResult: nil,
		},
		{
			name:      "returns error when StartTask fails",
			userEmail: "test@example.com",
			req: api.ExecutionRequest{
				Command: "echo hello",
			},
			setupMocks: func(r *MockRunner, e *MockExecutionRepository) {
				r.On("StartTask", mock.Anything, "test@example.com", mock.Anything).
					Return("", "", nil, apperrors.ErrInternalError("runner failed", nil))
			},
			wantErr:     true,
			checkResult: nil,
		},
		{
			name:      "continues even if CreateExecution fails",
			userEmail: "test@example.com",
			req: api.ExecutionRequest{
				Command: "echo hello",
			},
			setupMocks: func(r *MockRunner, e *MockExecutionRepository) {
				r.On("StartTask", mock.Anything, "test@example.com", mock.Anything).
					Return("exec-123", "task-arn", &now, nil)
				e.On("CreateExecution", mock.Anything, mock.Anything).
					Return(apperrors.ErrDatabaseError("database error", nil))
			},
			wantErr: false,
			checkResult: func(t *testing.T, resp *api.ExecutionResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, "exec-123", resp.ExecutionID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := new(MockRunner)
			mockExecRepo := new(MockExecutionRepository)
			tt.setupMocks(mockRunner, mockExecRepo)

			svc := NewService(nil, mockExecRepo, mockRunner, testutil.SilentLogger(), constants.AWS)

			resp, err := svc.RunCommand(context.Background(), tt.userEmail, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.checkResult != nil {
					tt.checkResult(t, resp)
				}
			}

			mockRunner.AssertExpectations(t)
			mockExecRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetExecutionStatus(t *testing.T) {
	tests := []struct {
		name        string
		executionID string
		setupMocks  func(*MockExecutionRepository)
		wantErr     bool
		checkResult func(*testing.T, *api.ExecutionStatusResponse)
	}{
		{
			name:        "successfully gets execution status",
			executionID: "exec-123",
			setupMocks: func(m *MockExecutionRepository) {
				exec := testutil.NewExecutionBuilder().
					WithExecutionID("exec-123").
					WithStatus(string(constants.ExecutionRunning)).
					Build()
				m.On("GetExecution", mock.Anything, "exec-123").Return(exec, nil)
			},
			wantErr: false,
			checkResult: func(t *testing.T, resp *api.ExecutionStatusResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, "exec-123", resp.ExecutionID)
				assert.Equal(t, string(constants.ExecutionRunning), resp.Status)
				assert.Nil(t, resp.ExitCode)
				assert.Nil(t, resp.CompletedAt)
			},
		},
		{
			name:        "returns status with exit code for completed execution",
			executionID: "exec-123",
			setupMocks: func(m *MockExecutionRepository) {
				exec := testutil.NewExecutionBuilder().
					WithExecutionID("exec-123").
					Completed().
					Build()
				m.On("GetExecution", mock.Anything, "exec-123").Return(exec, nil)
			},
			wantErr: false,
			checkResult: func(t *testing.T, resp *api.ExecutionStatusResponse) {
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.ExitCode)
				assert.Equal(t, 0, *resp.ExitCode)
				assert.NotNil(t, resp.CompletedAt)
			},
		},
		{
			name:        "returns error when executionRepo is nil",
			executionID: "exec-123",
			setupMocks:  func(m *MockExecutionRepository) {},
			wantErr:     true,
		},
		{
			name:        "returns error when executionID is empty",
			executionID: "",
			setupMocks:  func(m *MockExecutionRepository) {},
			wantErr:     true,
		},
		{
			name:        "returns error when execution not found",
			executionID: "exec-123",
			setupMocks: func(m *MockExecutionRepository) {
				m.On("GetExecution", mock.Anything, "exec-123").Return(nil, nil)
			},
			wantErr: true,
		},
		{
			name:        "returns error when GetExecution fails",
			executionID: "exec-123",
			setupMocks: func(m *MockExecutionRepository) {
				m.On("GetExecution", mock.Anything, "exec-123").
					Return(nil, apperrors.ErrDatabaseError("database error", nil))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecRepo := new(MockExecutionRepository)
			tt.setupMocks(mockExecRepo)

			svc := NewService(nil, mockExecRepo, nil, testutil.SilentLogger(), constants.AWS)

			resp, err := svc.GetExecutionStatus(context.Background(), tt.executionID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.checkResult != nil {
					tt.checkResult(t, resp)
				}
			}

			mockExecRepo.AssertExpectations(t)
		})
	}
}

func TestService_KillExecution(t *testing.T) {
	tests := []struct {
		name        string
		executionID string
		setupMocks  func(*MockExecutionRepository, *MockRunner)
		wantErr     bool
	}{
		{
			name:        "successfully kills running execution",
			executionID: "exec-123",
			setupMocks: func(e *MockExecutionRepository, r *MockRunner) {
				exec := testutil.NewExecutionBuilder().
					WithExecutionID("exec-123").
					WithStatus(string(constants.ExecutionRunning)).
					Build()
				e.On("GetExecution", mock.Anything, "exec-123").Return(exec, nil)
				r.On("KillTask", mock.Anything, "exec-123").Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "returns error when executionRepo is nil",
			executionID: "exec-123",
			setupMocks:  func(e *MockExecutionRepository, r *MockRunner) {},
			wantErr:     true,
		},
		{
			name:        "returns error when executionID is empty",
			executionID: "",
			setupMocks:  func(e *MockExecutionRepository, r *MockRunner) {},
			wantErr:     true,
		},
		{
			name:        "returns error when execution not found",
			executionID: "exec-123",
			setupMocks: func(e *MockExecutionRepository, r *MockRunner) {
				e.On("GetExecution", mock.Anything, "exec-123").Return(nil, nil)
			},
			wantErr: true,
		},
		{
			name:        "returns error when execution is already terminated (succeeded)",
			executionID: "exec-123",
			setupMocks: func(e *MockExecutionRepository, r *MockRunner) {
				exec := testutil.NewExecutionBuilder().
					WithExecutionID("exec-123").
					WithStatus(string(constants.ExecutionSucceeded)).
					Build()
				e.On("GetExecution", mock.Anything, "exec-123").Return(exec, nil)
			},
			wantErr: true,
		},
		{
			name:        "returns error when execution is already terminated (failed)",
			executionID: "exec-123",
			setupMocks: func(e *MockExecutionRepository, r *MockRunner) {
				exec := testutil.NewExecutionBuilder().
					WithExecutionID("exec-123").
					Failed().
					Build()
				e.On("GetExecution", mock.Anything, "exec-123").Return(exec, nil)
			},
			wantErr: true,
		},
		{
			name:        "returns error when execution is already terminated (stopped)",
			executionID: "exec-123",
			setupMocks: func(e *MockExecutionRepository, r *MockRunner) {
				exec := testutil.NewExecutionBuilder().
					WithExecutionID("exec-123").
					WithStatus(string(constants.ExecutionStopped)).
					Build()
				e.On("GetExecution", mock.Anything, "exec-123").Return(exec, nil)
			},
			wantErr: true,
		},
		{
			name:        "returns error when KillTask fails",
			executionID: "exec-123",
			setupMocks: func(e *MockExecutionRepository, r *MockRunner) {
				exec := testutil.NewExecutionBuilder().
					WithExecutionID("exec-123").
					WithStatus(string(constants.ExecutionRunning)).
					Build()
				e.On("GetExecution", mock.Anything, "exec-123").Return(exec, nil)
				r.On("KillTask", mock.Anything, "exec-123").
					Return(apperrors.ErrInternalError("kill failed", nil))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecRepo := new(MockExecutionRepository)
			mockRunner := new(MockRunner)
			tt.setupMocks(mockExecRepo, mockRunner)

			svc := NewService(nil, mockExecRepo, mockRunner, testutil.SilentLogger(), constants.AWS)

			err := svc.KillExecution(context.Background(), tt.executionID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockExecRepo.AssertExpectations(t)
			mockRunner.AssertExpectations(t)
		})
	}
}

func TestService_ListExecutions(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(*MockExecutionRepository)
		wantErr     bool
		checkResult func(*testing.T, []*api.Execution)
	}{
		{
			name: "successfully lists executions",
			setupMocks: func(m *MockExecutionRepository) {
				executions := []*api.Execution{
					testutil.NewExecutionBuilder().WithExecutionID("exec-1").Build(),
					testutil.NewExecutionBuilder().WithExecutionID("exec-2").Build(),
				}
				m.On("ListExecutions", mock.Anything).Return(executions, nil)
			},
			wantErr: false,
			checkResult: func(t *testing.T, executions []*api.Execution) {
				assert.NotNil(t, executions)
				assert.Len(t, executions, 2)
			},
		},
		{
			name: "returns empty list when no executions exist",
			setupMocks: func(m *MockExecutionRepository) {
				m.On("ListExecutions", mock.Anything).Return([]*api.Execution{}, nil)
			},
			wantErr: false,
			checkResult: func(t *testing.T, executions []*api.Execution) {
				assert.NotNil(t, executions)
				assert.Len(t, executions, 0)
			},
		},
		{
			name:       "returns error when executionRepo is nil",
			setupMocks: func(m *MockExecutionRepository) {},
			wantErr:    true,
		},
		{
			name: "returns error when ListExecutions fails",
			setupMocks: func(m *MockExecutionRepository) {
				m.On("ListExecutions", mock.Anything).
					Return(nil, apperrors.ErrDatabaseError("database error", nil))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecRepo := new(MockExecutionRepository)
			tt.setupMocks(mockExecRepo)

			svc := NewService(nil, mockExecRepo, nil, testutil.SilentLogger(), constants.AWS)

			executions, err := svc.ListExecutions(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, executions)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, executions)
				if tt.checkResult != nil {
					tt.checkResult(t, executions)
				}
			}

			mockExecRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetLogsByExecutionID(t *testing.T) {
	tests := []struct {
		name        string
		executionID string
		provider    constants.BackendProvider
		setupMocks  func(*MockRunner)
		wantErr     bool
		checkResult func(*testing.T, *api.LogsResponse)
	}{
		{
			name:        "returns error when executionID is empty",
			executionID: "",
			provider:    constants.AWS,
			setupMocks:  func(r *MockRunner) {},
			wantErr:     true,
			checkResult: nil,
		},
		{
			name:        "returns error when provider is not AWS",
			executionID: "exec-123",
			provider:    "GCP", // Not supported
			setupMocks:  func(r *MockRunner) {},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := new(MockRunner)
			tt.setupMocks(mockRunner)

			svc := NewService(nil, nil, mockRunner, testutil.SilentLogger(), tt.provider)

			resp, err := svc.GetLogsByExecutionID(context.Background(), tt.executionID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.checkResult != nil {
					tt.checkResult(t, resp)
				}
			}

			mockRunner.AssertExpectations(t)
		})
	}
}
