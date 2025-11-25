package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
	"runvoy/internal/backend/contract"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchTrace_EmptyRequestID(t *testing.T) {
	svc := newTraceTestService(t)
	trace, err := svc.FetchTrace(context.Background(), "")

	assert.Error(t, err)
	assert.Nil(t, trace)
}

func TestFetchTrace_SuccessWithEmptyResults(t *testing.T) {
	svc := newTraceTestService(t)
	requestID := "test-request-id"

	trace, err := svc.FetchTrace(context.Background(), requestID)

	require.NoError(t, err)
	require.NotNil(t, trace)
	assert.Empty(t, trace.Logs)
	assert.NotNil(t, trace.RelatedResources)
	assert.Nil(t, trace.RelatedResources.Executions)
	assert.Nil(t, trace.RelatedResources.Secrets)
	assert.Nil(t, trace.RelatedResources.Users)
	assert.Nil(t, trace.RelatedResources.Images)
}

func TestFetchTrace_WithBackendLogError(t *testing.T) {
	runner := &traceMinimalRunner{
		backendLogsErr: appErrors.ErrServiceUnavailable("backend unavailable", nil),
	}
	svc := newTraceTestServiceWithRunner(t, runner)

	trace, err := svc.FetchTrace(context.Background(), "test-request-id")

	assert.Error(t, err)
	assert.Nil(t, trace)
	assert.Equal(t, appErrors.ErrCodeServiceUnavailable, err.(*appErrors.AppError).Code)
}

func TestFetchTrace_ConcurrentFetching(t *testing.T) {
	requestID := "test-request-id"

	runner := &traceMinimalRunner{
		logs: []api.LogEvent{
			{
				Timestamp: time.Now().UnixMilli(),
				Message:   "test log message",
			},
		},
		delay: 10 * time.Millisecond,
	}

	execRepo := &minimalExecutionRepositoryWithDelay{
		delay: 10 * time.Millisecond,
		execs: []*api.Execution{
			{
				ExecutionID: "exec-1",
				Status:      string(constants.ExecutionSucceeded),
				Command:     "echo test",
			},
		},
	}

	svc := newTraceTestServiceWithRunner(t, runner, withExecutionRepo(execRepo))

	start := time.Now()
	trace, err := svc.FetchTrace(context.Background(), requestID)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, trace)

	assert.Len(t, trace.Logs, 1)
	assert.Len(t, trace.RelatedResources.Executions, 1)

	assert.Less(t, elapsed, 100*time.Millisecond, "concurrent fetching should be significantly faster than sequential")
}

func TestFetchTrace_ResourceFetchError(t *testing.T) {
	runner := &traceMinimalRunner{}
	execRepo := &minimalExecutionRepositoryWithError{
		err: appErrors.ErrInternalError("database error", errors.New("connection lost")),
	}

	svc := newTraceTestServiceWithRunner(t, runner, withExecutionRepo(execRepo))

	trace, err := svc.FetchTrace(context.Background(), "test-request-id")

	assert.Error(t, err)
	assert.Nil(t, trace)
	assert.Equal(t, appErrors.ErrCodeServiceUnavailable, err.(*appErrors.AppError).Code)
}

// Minimal test helpers

type traceMinimalRunner struct {
	logs           []api.LogEvent
	backendLogsErr error
	delay          time.Duration
}

func (m *traceMinimalRunner) StartTask(
	_ context.Context, _ string, _ *api.ExecutionRequest,
) (string, *time.Time, error) {
	return "test-id", nil, nil
}

func (m *traceMinimalRunner) KillTask(_ context.Context, _ string) error {
	return nil
}

func (m *traceMinimalRunner) RegisterImage(
	_ context.Context, _ string, _ *bool, _, _ *string, _, _ *int, _ *string, _ string,
) error {
	return nil
}

func (m *traceMinimalRunner) ListImages(_ context.Context) ([]api.ImageInfo, error) {
	return nil, nil
}

func (m *traceMinimalRunner) GetImage(_ context.Context, _ string) (*api.ImageInfo, error) {
	return nil, nil
}

func (m *traceMinimalRunner) RemoveImage(_ context.Context, _ string) error {
	return nil
}

func (m *traceMinimalRunner) FetchLogsByExecutionID(_ context.Context, _ string) ([]api.LogEvent, error) {
	return nil, nil
}

func (m *traceMinimalRunner) FetchBackendLogs(_ context.Context, _ string) ([]api.LogEvent, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	if m.backendLogsErr != nil {
		return nil, m.backendLogsErr
	}
	return m.logs, nil
}

func (m *traceMinimalRunner) GetImagesByRequestID(_ context.Context, _ string) ([]api.ImageInfo, error) {
	return nil, nil
}

type minimalUserRepository struct{}

func (r *minimalUserRepository) CreateUser(_ context.Context, _ *api.User, _ string, _ int64) error {
	return nil
}

func (r *minimalUserRepository) RemoveExpiration(_ context.Context, _ string) error {
	return nil
}

func (r *minimalUserRepository) GetUserByEmail(_ context.Context, _ string) (*api.User, error) {
	return nil, nil
}

func (r *minimalUserRepository) GetUserByAPIKeyHash(_ context.Context, _ string) (*api.User, error) {
	return nil, nil
}

func (r *minimalUserRepository) UpdateLastUsed(_ context.Context, _ string) (*time.Time, error) {
	return nil, nil
}

func (r *minimalUserRepository) RevokeUser(_ context.Context, _ string) error {
	return nil
}

func (r *minimalUserRepository) CreatePendingAPIKey(_ context.Context, _ *api.PendingAPIKey) error {
	return nil
}

func (r *minimalUserRepository) GetPendingAPIKey(_ context.Context, _ string) (*api.PendingAPIKey, error) {
	return nil, nil
}

func (r *minimalUserRepository) ConfirmPendingAPIKey(_ context.Context, _, _ string) error {
	return nil
}

func (r *minimalUserRepository) DeletePendingAPIKey(_ context.Context, _ string) error {
	return nil
}

func (r *minimalUserRepository) GetUsersByRequestID(_ context.Context, _ string) ([]*api.User, error) {
	return nil, nil
}

func (r *minimalUserRepository) ListUsers(_ context.Context) ([]*api.User, error) {
	return nil, nil
}

func (r *minimalUserRepository) MarkAsViewed(_ context.Context, _, _ string) error {
	return nil
}

type minimalExecutionRepository struct{}

func (r *minimalExecutionRepository) CreateExecution(_ context.Context, _ *api.Execution) error {
	return nil
}

func (r *minimalExecutionRepository) GetExecution(_ context.Context, _ string) (*api.Execution, error) {
	return nil, nil
}

func (r *minimalExecutionRepository) UpdateExecution(_ context.Context, _ *api.Execution) error {
	return nil
}

func (r *minimalExecutionRepository) ListExecutions(_ context.Context, _ int, _ []string) ([]*api.Execution, error) {
	return nil, nil
}

func (r *minimalExecutionRepository) GetExecutionsByRequestID(_ context.Context, _ string) ([]*api.Execution, error) {
	return nil, nil
}

type minimalExecutionRepositoryWithDelay struct {
	minimalExecutionRepository
	delay time.Duration
	execs []*api.Execution
}

func (r *minimalExecutionRepositoryWithDelay) GetExecutionsByRequestID(
	_ context.Context, _ string,
) ([]*api.Execution, error) {
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	return r.execs, nil
}

type minimalExecutionRepositoryWithError struct {
	minimalExecutionRepository
	err error
}

func (r *minimalExecutionRepositoryWithError) GetExecutionsByRequestID(
	_ context.Context, _ string,
) ([]*api.Execution, error) {
	return nil, r.err
}

type minimalConnectionRepository struct{}

func (r *minimalConnectionRepository) CreateConnection(_ context.Context, _ *api.WebSocketConnection) error {
	return nil
}

func (r *minimalConnectionRepository) DeleteConnections(_ context.Context, _ []string) (int, error) {
	return 0, nil
}

func (r *minimalConnectionRepository) GetConnectionsByExecutionID(
	_ context.Context, _ string,
) ([]*api.WebSocketConnection, error) {
	return nil, nil
}

type minimalTokenRepository struct{}

func (r *minimalTokenRepository) CreateToken(_ context.Context, _ *api.WebSocketToken) error {
	return nil
}

func (r *minimalTokenRepository) GetToken(_ context.Context, _ string) (*api.WebSocketToken, error) {
	return nil, nil
}

func (r *minimalTokenRepository) DeleteToken(_ context.Context, _ string) error {
	return nil
}

type minimalImageRepository struct{}

func (r *minimalImageRepository) GetImagesByRequestID(_ context.Context, _ string) ([]api.ImageInfo, error) {
	return nil, nil
}

type minimalSecretsRepository struct{}

func (r *minimalSecretsRepository) CreateSecret(_ context.Context, _ *api.Secret) error {
	return nil
}

func (r *minimalSecretsRepository) GetSecret(_ context.Context, _ string, _ bool) (*api.Secret, error) {
	return nil, nil
}

func (r *minimalSecretsRepository) ListSecrets(_ context.Context, _ bool) ([]*api.Secret, error) {
	return nil, nil
}

func (r *minimalSecretsRepository) UpdateSecret(_ context.Context, _ *api.Secret) error {
	return nil
}

func (r *minimalSecretsRepository) DeleteSecret(_ context.Context, _ string) error {
	return nil
}

func (r *minimalSecretsRepository) GetSecretsByRequestID(_ context.Context, _ string) ([]*api.Secret, error) {
	return nil, nil
}

type minimalWebSocketManager struct{}

func (m *minimalWebSocketManager) GenerateWebSocketURL(
	_ context.Context, _ string, _ *string, _ *string,
) string {
	return ""
}

func (m *minimalWebSocketManager) HandleRequest(
	_ context.Context, _ *json.RawMessage, _ *slog.Logger,
) (bool, error) {
	return false, nil
}

func (m *minimalWebSocketManager) NotifyExecutionCompletion(_ context.Context, _ *string) error {
	return nil
}

func (m *minimalWebSocketManager) SendLogsToExecution(
	_ context.Context, _ *string, _ []api.LogEvent,
) error {
	return nil
}

type minimalHealthManager struct{}

func (m *minimalHealthManager) Reconcile(_ context.Context) (*api.HealthReport, error) {
	return &api.HealthReport{}, nil
}

// newTraceTestService creates a Service for trace testing with minimal mocks.
// The runner parameter implements all 4 interfaces (TaskManager, ImageRegistry, LogManager, ObservabilityManager).
func newTraceTestService(t *testing.T) *Service {
	return newTraceTestServiceWithRunner(t, &traceMinimalRunner{})
}

// newTraceTestServiceWithRunner creates a Service for trace testing with a custom runner.
// The runner parameter implements all 4 interfaces (TaskManager, ImageRegistry, LogManager, ObservabilityManager).
// Optional repositories can be provided to override defaults.
func newTraceTestServiceWithRunner(
	t *testing.T,
	runner interface {
		contract.TaskManager
		contract.ImageRegistry
		contract.LogManager
		contract.ObservabilityManager
	},
	opts ...traceTestServiceOption,
) *Service {
	taskManager, _ := runner.(contract.TaskManager)
	imageRegistry, _ := runner.(contract.ImageRegistry)
	logManager, _ := runner.(contract.LogManager)
	observabilityManager, _ := runner.(contract.ObservabilityManager)

	cfg := &traceTestServiceConfig{
		userRepo:      &minimalUserRepository{},
		execRepo:      &minimalExecutionRepository{},
		connRepo:      &minimalConnectionRepository{},
		tokenRepo:     &minimalTokenRepository{},
		imageRepo:     &minimalImageRepository{},
		secretsRepo:   &minimalSecretsRepository{},
		wsManager:     &minimalWebSocketManager{},
		healthManager: &minimalHealthManager{},
	}

	for _, opt := range opts {
		opt(cfg)
	}

	repos := database.Repositories{
		User:       cfg.userRepo,
		Execution:  cfg.execRepo,
		Connection: cfg.connRepo,
		Token:      cfg.tokenRepo,
		Image:      cfg.imageRepo,
		Secrets:    cfg.secretsRepo,
	}
	svc, err := NewService(
		context.Background(),
		testRegion,
		&repos,
		taskManager,
		imageRegistry,
		logManager,
		observabilityManager,
		testutil.SilentLogger(),
		constants.AWS,
		cfg.wsManager,
		cfg.healthManager,
		newTraceTestEnforcer(t),
	)
	require.NoError(t, err)
	return svc
}

type traceTestServiceConfig struct {
	userRepo      database.UserRepository
	execRepo      database.ExecutionRepository
	connRepo      database.ConnectionRepository
	tokenRepo     database.TokenRepository
	imageRepo     database.ImageRepository
	secretsRepo   database.SecretsRepository
	wsManager     contract.WebSocketManager
	healthManager contract.HealthManager
}

type traceTestServiceOption func(*traceTestServiceConfig)

func withExecutionRepo(repo database.ExecutionRepository) traceTestServiceOption {
	return func(cfg *traceTestServiceConfig) {
		cfg.execRepo = repo
	}
}

func newTraceTestEnforcer(t *testing.T) *authorization.Enforcer {
	enforcer, err := authorization.NewEnforcer(testutil.SilentLogger())
	require.NoError(t, err)
	return enforcer
}
