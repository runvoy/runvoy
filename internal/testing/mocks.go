package testing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"runvoy/internal/api"
	"runvoy/internal/services"
)

// MockAuthService mocks the AuthService
type MockAuthService struct {
	mock.Mock
}

func NewMockAuthService(t *testing.T) *MockAuthService {
	return &MockAuthService{}
}

func (m *MockAuthService) ValidateAPIKey(ctx context.Context, apiKey string) (*api.User, error) {
	args := m.Called(ctx, apiKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.User), args.Error(1)
}

func (m *MockAuthService) GenerateAPIKey(ctx context.Context, email string) (string, error) {
	args := m.Called(ctx, email)
	return args.String(0), args.Error(1)
}

func (m *MockAuthService) RevokeAPIKey(ctx context.Context, email string) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

// MockStorageService mocks the StorageService
type MockStorageService struct {
	mock.Mock
}

func NewMockStorageService(t *testing.T) *MockStorageService {
	return &MockStorageService{}
}

func (m *MockStorageService) GetUserByAPIKey(ctx context.Context, apiKeyHash string) (*api.User, error) {
	args := m.Called(ctx, apiKeyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.User), args.Error(1)
}

func (m *MockStorageService) CreateUser(ctx context.Context, user *api.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockStorageService) UpdateUser(ctx context.Context, user *api.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockStorageService) DeleteUser(ctx context.Context, email string) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

func (m *MockStorageService) CreateExecution(ctx context.Context, execution *api.Execution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
}

func (m *MockStorageService) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	args := m.Called(ctx, executionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.Execution), args.Error(1)
}

func (m *MockStorageService) UpdateExecution(ctx context.Context, execution *api.Execution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
}

func (m *MockStorageService) ListExecutions(ctx context.Context, userEmail string, limit int) ([]*api.Execution, error) {
	args := m.Called(ctx, userEmail, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*api.Execution), args.Error(1)
}

func (m *MockStorageService) CreateLock(ctx context.Context, lock *api.Lock) error {
	args := m.Called(ctx, lock)
	return args.Error(0)
}

func (m *MockStorageService) GetLock(ctx context.Context, lockName string) (*api.Lock, error) {
	args := m.Called(ctx, lockName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.Lock), args.Error(1)
}

func (m *MockStorageService) DeleteLock(ctx context.Context, lockName string) error {
	args := m.Called(ctx, lockName)
	return args.Error(0)
}

// MockECSService mocks the ECSService
type MockECSService struct {
	mock.Mock
}

func NewMockECSService(t *testing.T) *MockECSService {
	return &MockECSService{}
}

func (m *MockECSService) StartTask(ctx context.Context, req *api.ExecutionRequest, executionID string, userEmail string) (string, error) {
	args := m.Called(ctx, req, executionID, userEmail)
	return args.String(0), args.Error(1)
}

func (m *MockECSService) GetTaskStatus(ctx context.Context, taskARN string) (string, error) {
	args := m.Called(ctx, taskARN)
	return args.String(0), args.Error(1)
}

func (m *MockECSService) StopTask(ctx context.Context, taskARN string) error {
	args := m.Called(ctx, taskARN)
	return args.Error(0)
}

// MockLockService mocks the LockService
type MockLockService struct {
	mock.Mock
}

func NewMockLockService(t *testing.T) *MockLockService {
	return &MockLockService{}
}

func (m *MockLockService) AcquireLock(ctx context.Context, lockName string, userEmail string, executionID string) error {
	args := m.Called(ctx, lockName, userEmail, executionID)
	return args.Error(0)
}

func (m *MockLockService) ReleaseLock(ctx context.Context, lockName string) error {
	args := m.Called(ctx, lockName)
	return args.Error(0)
}

func (m *MockLockService) GetLockHolder(ctx context.Context, lockName string) (*api.Lock, error) {
	args := m.Called(ctx, lockName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.Lock), args.Error(1)
}

// MockLogService mocks the LogService
type MockLogService struct {
	mock.Mock
}

func NewMockLogService(t *testing.T) *MockLogService {
	return &MockLogService{}
}

func (m *MockLogService) GetLogs(ctx context.Context, executionID string, since time.Time) (string, error) {
	args := m.Called(ctx, executionID, since)
	return args.String(0), args.Error(1)
}

func (m *MockLogService) GenerateLogURL(ctx context.Context, executionID string) (string, error) {
	args := m.Called(ctx, executionID)
	return args.String(0), args.Error(1)
}

// MockExecutionService mocks the ExecutionService
type MockExecutionService struct {
	mock.Mock
}

func NewMockExecutionService(t *testing.T) *MockExecutionService {
	return &MockExecutionService{}
}

func (m *MockExecutionService) StartExecution(ctx context.Context, req *api.ExecutionRequest, user *api.User) (*api.ExecutionResponse, error) {
	args := m.Called(ctx, req, user)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.ExecutionResponse), args.Error(1)
}

func (m *MockExecutionService) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	args := m.Called(ctx, executionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.Execution), args.Error(1)
}

func (m *MockExecutionService) ListExecutions(ctx context.Context, userEmail string, limit int) ([]*api.Execution, error) {
	args := m.Called(ctx, userEmail, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*api.Execution), args.Error(1)
}

func (m *MockExecutionService) UpdateExecutionStatus(ctx context.Context, executionID string, status string, exitCode int) error {
	args := m.Called(ctx, executionID, status, exitCode)
	return args.Error(0)
}