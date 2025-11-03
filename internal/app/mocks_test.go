package app

import (
	"context"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	"runvoy/internal/testutil"
)

// mockUserRepository implements database.UserRepository for testing
type mockUserRepository struct {
	createUserFunc               func(ctx context.Context, user *api.User, apiKeyHash string) error
	createUserWithExpirationFunc func(ctx context.Context, user *api.User, apiKeyHash string, expiresAtUnix int64) error
	removeExpirationFunc         func(ctx context.Context, email string) error
	getUserByEmailFunc           func(ctx context.Context, email string) (*api.User, error)
	getUserByAPIKeyHashFunc      func(ctx context.Context, apiKeyHash string) (*api.User, error)
	updateLastUsedFunc           func(ctx context.Context, email string) (*time.Time, error)
	revokeUserFunc               func(ctx context.Context, email string) error
	createPendingAPIKeyFunc      func(ctx context.Context, pending *api.PendingAPIKey) error
	getPendingAPIKeyFunc         func(ctx context.Context, secretToken string) (*api.PendingAPIKey, error)
	markAsViewedFunc             func(ctx context.Context, secretToken string, ipAddress string) error
	deletePendingAPIKeyFunc      func(ctx context.Context, secretToken string) error
}

func (m *mockUserRepository) CreateUser(ctx context.Context, user *api.User, apiKeyHash string) error {
	if m.createUserFunc != nil {
		return m.createUserFunc(ctx, user, apiKeyHash)
	}
	return nil
}

func (m *mockUserRepository) CreateUserWithExpiration(
	ctx context.Context,
	user *api.User,
	apiKeyHash string,
	expiresAtUnix int64,
) error {
	if m.createUserWithExpirationFunc != nil {
		return m.createUserWithExpirationFunc(ctx, user, apiKeyHash, expiresAtUnix)
	}
	return nil
}

func (m *mockUserRepository) RemoveExpiration(ctx context.Context, email string) error {
	if m.removeExpirationFunc != nil {
		return m.removeExpirationFunc(ctx, email)
	}
	return nil
}

func (m *mockUserRepository) GetUserByEmail(ctx context.Context, email string) (*api.User, error) {
	if m.getUserByEmailFunc != nil {
		return m.getUserByEmailFunc(ctx, email)
	}
	return nil, nil
}

func (m *mockUserRepository) GetUserByAPIKeyHash(ctx context.Context, apiKeyHash string) (*api.User, error) {
	if m.getUserByAPIKeyHashFunc != nil {
		return m.getUserByAPIKeyHashFunc(ctx, apiKeyHash)
	}
	return nil, nil
}

func (m *mockUserRepository) UpdateLastUsed(ctx context.Context, email string) (*time.Time, error) {
	if m.updateLastUsedFunc != nil {
		return m.updateLastUsedFunc(ctx, email)
	}
	now := time.Now()
	return &now, nil
}

func (m *mockUserRepository) RevokeUser(ctx context.Context, email string) error {
	if m.revokeUserFunc != nil {
		return m.revokeUserFunc(ctx, email)
	}
	return nil
}

func (m *mockUserRepository) CreatePendingAPIKey(ctx context.Context, pending *api.PendingAPIKey) error {
	if m.createPendingAPIKeyFunc != nil {
		return m.createPendingAPIKeyFunc(ctx, pending)
	}
	return nil
}

func (m *mockUserRepository) GetPendingAPIKey(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
	if m.getPendingAPIKeyFunc != nil {
		return m.getPendingAPIKeyFunc(ctx, secretToken)
	}
	return nil, nil
}

func (m *mockUserRepository) MarkAsViewed(ctx context.Context, secretToken, ipAddress string) error {
	if m.markAsViewedFunc != nil {
		return m.markAsViewedFunc(ctx, secretToken, ipAddress)
	}
	return nil
}

func (m *mockUserRepository) DeletePendingAPIKey(ctx context.Context, secretToken string) error {
	if m.deletePendingAPIKeyFunc != nil {
		return m.deletePendingAPIKeyFunc(ctx, secretToken)
	}
	return nil
}

func (m *mockUserRepository) ListUsers(_ context.Context) ([]*api.User, error) {
	return []*api.User{}, nil
}

// mockExecutionRepository implements database.ExecutionRepository for testing
type mockExecutionRepository struct {
	createExecutionFunc func(ctx context.Context, execution *api.Execution) error
	getExecutionFunc    func(ctx context.Context, executionID string) (*api.Execution, error)
	updateExecutionFunc func(ctx context.Context, execution *api.Execution) error
	listExecutionsFunc  func(ctx context.Context) ([]*api.Execution, error)
}

func (m *mockExecutionRepository) CreateExecution(ctx context.Context, execution *api.Execution) error {
	if m.createExecutionFunc != nil {
		return m.createExecutionFunc(ctx, execution)
	}
	return nil
}

func (m *mockExecutionRepository) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	if m.getExecutionFunc != nil {
		return m.getExecutionFunc(ctx, executionID)
	}
	return nil, nil
}

func (m *mockExecutionRepository) UpdateExecution(ctx context.Context, execution *api.Execution) error {
	if m.updateExecutionFunc != nil {
		return m.updateExecutionFunc(ctx, execution)
	}
	return nil
}

func (m *mockExecutionRepository) ListExecutions(ctx context.Context) ([]*api.Execution, error) {
	if m.listExecutionsFunc != nil {
		return m.listExecutionsFunc(ctx)
	}
	return []*api.Execution{}, nil
}

// mockRunner implements Runner interface for testing
type mockRunner struct {
	startTaskFunc func(
		ctx context.Context,
		userEmail string,
		req *api.ExecutionRequest,
	) (string, *time.Time, error)
	killTaskFunc               func(ctx context.Context, executionID string) error
	registerImageFunc          func(ctx context.Context, image string, isDefault *bool) error
	listImagesFunc             func(ctx context.Context) ([]api.ImageInfo, error)
	removeImageFunc            func(ctx context.Context, image string) error
	fetchLogsByExecutionIDFunc func(ctx context.Context, executionID string) ([]api.LogEvent, error)
}

func (m *mockRunner) StartTask(
	ctx context.Context,
	userEmail string,
	req *api.ExecutionRequest,
) (string, *time.Time, error) {
	if m.startTaskFunc != nil {
		return m.startTaskFunc(ctx, userEmail, req)
	}
	return "test-execution-id", nil, nil
}

func (m *mockRunner) KillTask(ctx context.Context, executionID string) error {
	if m.killTaskFunc != nil {
		return m.killTaskFunc(ctx, executionID)
	}
	return nil
}

func (m *mockRunner) RegisterImage(ctx context.Context, image string, isDefault *bool) error {
	if m.registerImageFunc != nil {
		return m.registerImageFunc(ctx, image, isDefault)
	}
	return nil
}

func (m *mockRunner) ListImages(ctx context.Context) ([]api.ImageInfo, error) {
	if m.listImagesFunc != nil {
		return m.listImagesFunc(ctx)
	}
	return []api.ImageInfo{}, nil
}

func (m *mockRunner) RemoveImage(ctx context.Context, image string) error {
	if m.removeImageFunc != nil {
		return m.removeImageFunc(ctx, image)
	}
	return nil
}

func (m *mockRunner) FetchLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error) {
	if m.fetchLogsByExecutionIDFunc != nil {
		return m.fetchLogsByExecutionIDFunc(ctx, executionID)
	}
	return []api.LogEvent{}, nil
}

// newTestService creates a Service with mocks for testing
func newTestService(userRepo *mockUserRepository, execRepo *mockExecutionRepository, runner *mockRunner) *Service {
	logger := testutil.SilentLogger()
	return NewService(userRepo, execRepo, runner, logger, constants.AWS)
}
