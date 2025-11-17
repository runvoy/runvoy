package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
	"runvoy/internal/backend/websocket"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	"runvoy/internal/testutil"
)

// mockUserRepository implements database.UserRepository for testing
type mockUserRepository struct {
	createUserFunc          func(ctx context.Context, user *api.User, apiKeyHash string, expiresAtUnix int64) error
	removeExpirationFunc    func(ctx context.Context, email string) error
	getUserByEmailFunc      func(ctx context.Context, email string) (*api.User, error)
	getUserByAPIKeyHashFunc func(ctx context.Context, apiKeyHash string) (*api.User, error)
	updateLastUsedFunc      func(ctx context.Context, email string) (*time.Time, error)
	revokeUserFunc          func(ctx context.Context, email string) error
	createPendingAPIKeyFunc func(ctx context.Context, pending *api.PendingAPIKey) error
	getPendingAPIKeyFunc    func(ctx context.Context, secretToken string) (*api.PendingAPIKey, error)
	markAsViewedFunc        func(ctx context.Context, secretToken string, ipAddress string) error
	deletePendingAPIKeyFunc func(ctx context.Context, secretToken string) error
	listUsersFunc           func(ctx context.Context) ([]*api.User, error)
}

func (m *mockUserRepository) CreateUser(
	ctx context.Context,
	user *api.User,
	apiKeyHash string,
	expiresAtUnix int64,
) error {
	if m.createUserFunc != nil {
		return m.createUserFunc(ctx, user, apiKeyHash, expiresAtUnix)
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

func (m *mockUserRepository) ListUsers(ctx context.Context) ([]*api.User, error) {
	if m.listUsersFunc != nil {
		return m.listUsersFunc(ctx)
	}
	return []*api.User{}, nil
}

// mockExecutionRepository implements database.ExecutionRepository for testing
type mockExecutionRepository struct {
	createExecutionFunc func(ctx context.Context, execution *api.Execution) error
	getExecutionFunc    func(ctx context.Context, executionID string) (*api.Execution, error)
	updateExecutionFunc func(ctx context.Context, execution *api.Execution) error
	listExecutionsFunc  func(ctx context.Context, limit int, statuses []string) ([]*api.Execution, error)
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

func (m *mockExecutionRepository) ListExecutions(
	ctx context.Context,
	limit int,
	statuses []string,
) ([]*api.Execution, error) {
	if m.listExecutionsFunc != nil {
		return m.listExecutionsFunc(ctx, limit, statuses)
	}
	return []*api.Execution{}, nil
}

// mockConnectionRepository implements database.ConnectionRepository for testing
type mockConnectionRepository struct {
	createConnectionFunc            func(ctx context.Context, conn *api.WebSocketConnection) error
	deleteConnectionsFunc           func(ctx context.Context, connIDs []string) (int, error)
	getConnectionsByExecutionIDFunc func(ctx context.Context, executionID string) ([]*api.WebSocketConnection, error)
}

func (m *mockConnectionRepository) CreateConnection(ctx context.Context, conn *api.WebSocketConnection) error {
	if m.createConnectionFunc != nil {
		return m.createConnectionFunc(ctx, conn)
	}
	return nil
}

func (m *mockConnectionRepository) DeleteConnections(ctx context.Context, connIDs []string) (int, error) {
	if m.deleteConnectionsFunc != nil {
		return m.deleteConnectionsFunc(ctx, connIDs)
	}
	return len(connIDs), nil
}

func (m *mockConnectionRepository) GetConnectionsByExecutionID(
	ctx context.Context, executionID string,
) ([]*api.WebSocketConnection, error) {
	if m.getConnectionsByExecutionIDFunc != nil {
		return m.getConnectionsByExecutionIDFunc(ctx, executionID)
	}
	return nil, nil
}

// mockTokenRepository implements database.TokenRepository for testing
type mockTokenRepository struct {
	createTokenFunc func(ctx context.Context, token *api.WebSocketToken) error
	getTokenFunc    func(ctx context.Context, tokenValue string) (*api.WebSocketToken, error)
	deleteTokenFunc func(ctx context.Context, tokenValue string) error
}

func (m *mockTokenRepository) CreateToken(ctx context.Context, token *api.WebSocketToken) error {
	if m.createTokenFunc != nil {
		return m.createTokenFunc(ctx, token)
	}
	return nil
}

func (m *mockTokenRepository) GetToken(ctx context.Context, tokenValue string) (*api.WebSocketToken, error) {
	if m.getTokenFunc != nil {
		return m.getTokenFunc(ctx, tokenValue)
	}
	return nil, nil
}

func (m *mockTokenRepository) DeleteToken(ctx context.Context, tokenValue string) error {
	if m.deleteTokenFunc != nil {
		return m.deleteTokenFunc(ctx, tokenValue)
	}
	return nil
}

// mockRunner implements Runner interface for testing
type mockRunner struct {
	startTaskFunc func(
		ctx context.Context,
		userEmail string,
		req *api.ExecutionRequest,
	) (string, *time.Time, error)
	killTaskFunc      func(ctx context.Context, executionID string) error
	registerImageFunc func(
		ctx context.Context,
		image string,
		isDefault *bool,
		taskRoleName, taskExecutionRoleName *string,
		cpu, memory *int,
		runtimePlatform *string,
	) error
	listImagesFunc             func(ctx context.Context) ([]api.ImageInfo, error)
	getImageFunc               func(ctx context.Context, image string) (*api.ImageInfo, error)
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

func (m *mockRunner) RegisterImage(
	ctx context.Context,
	image string,
	isDefault *bool,
	taskRoleName, taskExecutionRoleName *string,
	cpu, memory *int,
	runtimePlatform *string,
) error {
	if m.registerImageFunc != nil {
		return m.registerImageFunc(ctx, image, isDefault, taskRoleName, taskExecutionRoleName, cpu, memory, runtimePlatform)
	}
	return nil
}

func (m *mockRunner) ListImages(ctx context.Context) ([]api.ImageInfo, error) {
	if m.listImagesFunc != nil {
		return m.listImagesFunc(ctx)
	}
	return []api.ImageInfo{}, nil
}

func (m *mockRunner) GetImage(ctx context.Context, image string) (*api.ImageInfo, error) {
	if m.getImageFunc != nil {
		return m.getImageFunc(ctx, image)
	}
	return nil, nil
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
func newTestService(
	userRepo *mockUserRepository,
	execRepo *mockExecutionRepository,
	runner *mockRunner,
) *Service {
	return newTestServiceWithConnRepo(userRepo, execRepo, nil, runner)
}

// newTestServiceWithConnRepo creates a Service with connection repo mock for testing
func newTestServiceWithConnRepo(
	userRepo *mockUserRepository,
	execRepo *mockExecutionRepository,
	connRepo *mockConnectionRepository,
	runner *mockRunner,
) *Service {
	logger := testutil.SilentLogger()
	svc, err := NewService(context.Background(),
		userRepo, execRepo, connRepo, &mockTokenRepository{}, runner, logger, constants.AWS, nil, nil, nil, nil,
	)
	if err != nil {
		panic(err)
	}
	return svc
}

// newTestServiceWithSecretsRepo creates a Service with a secrets repository for testing.
func newTestServiceWithEnforcer(
	userRepo *mockUserRepository,
	execRepo *mockExecutionRepository,
	runner *mockRunner,
	secretsRepo database.SecretsRepository,
) (*Service, *authorization.Enforcer) {
	logger := testutil.SilentLogger()
	enforcer, err := authorization.NewEnforcer(logger)
	if err != nil {
		panic(err)
	}

	var userRepoIface database.UserRepository
	if userRepo != nil {
		userRepoIface = userRepo
	}

	var execRepoIface database.ExecutionRepository
	if execRepo != nil {
		execRepoIface = execRepo
	}

	var runnerIface Runner
	if runner != nil {
		runnerIface = runner
	}

	svc, err := NewService(
		context.Background(),
		userRepoIface,
		execRepoIface,
		nil,
		&mockTokenRepository{},
		runnerIface,
		logger,
		constants.AWS,
		nil,
		secretsRepo,
		nil,
		enforcer,
	)
	if err != nil {
		panic(err)
	}
	return svc, enforcer
}

func newTestServiceWithSecretsRepo(
	userRepo *mockUserRepository,
	execRepo *mockExecutionRepository,
	runner *mockRunner,
	secretsRepo database.SecretsRepository,
) *Service {
	logger := testutil.SilentLogger()

	var userRepoIface database.UserRepository
	if userRepo != nil {
		userRepoIface = userRepo
	}

	var execRepoIface database.ExecutionRepository
	if execRepo != nil {
		execRepoIface = execRepo
	}

	var runnerIface Runner
	if runner != nil {
		runnerIface = runner
	}

	svc, err := NewService(
		context.Background(),
		userRepoIface,
		execRepoIface,
		nil,
		&mockTokenRepository{},
		runnerIface,
		logger,
		constants.AWS,
		nil,
		secretsRepo,
		nil,
		nil,
	)
	if err != nil {
		panic(err)
	}
	return svc
}

// mockSecretsRepository implements database.SecretsRepository for testing.
type mockSecretsRepository struct {
	createSecretFunc func(ctx context.Context, secret *api.Secret) error
	getSecretFunc    func(ctx context.Context, name string, includeValue bool) (*api.Secret, error)
	listSecretsFunc  func(ctx context.Context, includeValue bool) ([]*api.Secret, error)
	updateSecretFunc func(ctx context.Context, secret *api.Secret) error
	deleteSecretFunc func(ctx context.Context, name string) error
}

func (m *mockSecretsRepository) CreateSecret(ctx context.Context, secret *api.Secret) error {
	if m.createSecretFunc != nil {
		return m.createSecretFunc(ctx, secret)
	}
	return nil
}

func (m *mockSecretsRepository) GetSecret(
	ctx context.Context,
	name string,
	includeValue bool,
) (*api.Secret, error) {
	if m.getSecretFunc != nil {
		return m.getSecretFunc(ctx, name, includeValue)
	}
	return nil, nil
}

func (m *mockSecretsRepository) ListSecrets(
	ctx context.Context,
	includeValue bool,
) ([]*api.Secret, error) {
	if m.listSecretsFunc != nil {
		return m.listSecretsFunc(ctx, includeValue)
	}
	return nil, nil
}

func (m *mockSecretsRepository) UpdateSecret(ctx context.Context, secret *api.Secret) error {
	if m.updateSecretFunc != nil {
		return m.updateSecretFunc(ctx, secret)
	}
	return nil
}

func (m *mockSecretsRepository) DeleteSecret(ctx context.Context, name string) error {
	if m.deleteSecretFunc != nil {
		return m.deleteSecretFunc(ctx, name)
	}
	return nil
}

// newTestServiceWithWebSocketManager creates a Service with websocket manager for testing
func newTestServiceWithWebSocketManager(
	userRepo *mockUserRepository,
	execRepo *mockExecutionRepository,
	runner *mockRunner,
	wsManager websocket.Manager,
) *Service {
	logger := testutil.SilentLogger()
	svc, err := NewService(context.Background(),
		userRepo, execRepo, nil, &mockTokenRepository{}, runner, logger, constants.AWS, wsManager, nil, nil, nil,
	)
	if err != nil {
		panic(err)
	}
	return svc
}

// mockWebSocketManager implements websocket.Manager for testing
type mockWebSocketManager struct {
	generateWebSocketURLFunc func(
		ctx context.Context,
		executionID string,
		userEmail *string,
		clientIPAtCreationTime *string,
	) string
	handleRequestFunc func(
		ctx context.Context,
		rawEvent *json.RawMessage,
		reqLogger *slog.Logger,
	) (bool, error)
	notifyExecutionCompletionFunc func(ctx context.Context, executionID *string) error
	sendLogsToExecutionFunc       func(
		ctx context.Context,
		executionID *string,
		logEvents []api.LogEvent,
	) error
}

func (m *mockWebSocketManager) GenerateWebSocketURL(
	ctx context.Context,
	executionID string,
	userEmail *string,
	clientIPAtCreationTime *string,
) string {
	if m.generateWebSocketURLFunc != nil {
		return m.generateWebSocketURLFunc(ctx, executionID, userEmail, clientIPAtCreationTime)
	}
	return ""
}

func (m *mockWebSocketManager) HandleRequest(
	ctx context.Context,
	rawEvent *json.RawMessage,
	reqLogger *slog.Logger,
) (bool, error) {
	if m.handleRequestFunc != nil {
		return m.handleRequestFunc(ctx, rawEvent, reqLogger)
	}
	return false, nil
}

func (m *mockWebSocketManager) NotifyExecutionCompletion(ctx context.Context, executionID *string) error {
	if m.notifyExecutionCompletionFunc != nil {
		return m.notifyExecutionCompletionFunc(ctx, executionID)
	}
	return nil
}

func (m *mockWebSocketManager) SendLogsToExecution(
	ctx context.Context,
	executionID *string,
	logEvents []api.LogEvent,
) error {
	if m.sendLogsToExecutionFunc != nil {
		return m.sendLogsToExecutionFunc(ctx, executionID, logEvents)
	}
	return nil
}
