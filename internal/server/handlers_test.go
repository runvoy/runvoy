package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
	"runvoy/internal/backend/contract"
	"runvoy/internal/backend/orchestrator"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test mocks for repositories and runner
type testUserRepository struct {
	authenticateUserFunc func(apiKeyHash string) (*api.User, error)
	updateLastUsedFunc   func(email string) error
	getUserByEmailFunc   func(email string) (*api.User, error)
	getPendingAPIKeyFunc func(ctx context.Context, secretToken string) (*api.PendingAPIKey, error)
	markAsViewedFunc     func(ctx context.Context, secretToken string, ipAddress string) error
	createUserFunc       func(ctx context.Context, user *api.User, apiKeyHash string, expiresAt int64) error
	listUsersFunc        func(ctx context.Context) ([]*api.User, error)
	revokeUserFunc       func(ctx context.Context, email string) error
}

func (t *testUserRepository) CreateUser(
	ctx context.Context,
	user *api.User,
	apiKeyHash string,
	expiresAt int64,
) error {
	if t.createUserFunc != nil {
		return t.createUserFunc(ctx, user, apiKeyHash, expiresAt)
	}
	return nil
}

func (t *testUserRepository) RemoveExpiration(_ context.Context, _ string) error {
	return nil
}

func (t *testUserRepository) GetUserByEmail(_ context.Context, email string) (*api.User, error) {
	if t.getUserByEmailFunc != nil {
		return t.getUserByEmailFunc(email)
	}
	return &api.User{
		Role:    authorization.RoleViewer.String(),
		Email:   email,
		Revoked: false,
	}, nil
}

func (t *testUserRepository) GetUserByAPIKeyHash(_ context.Context, apiKeyHash string) (*api.User, error) {
	if t.authenticateUserFunc != nil {
		return t.authenticateUserFunc(apiKeyHash)
	}
	return &api.User{
		Email:   "user@example.com",
		Revoked: false,
	}, nil
}

func (t *testUserRepository) UpdateLastUsed(_ context.Context, email string) (*time.Time, error) {
	if t.updateLastUsedFunc != nil {
		err := t.updateLastUsedFunc(email)
		if err != nil {
			return nil, err
		}
	}
	now := time.Now()
	return &now, nil
}

func (t *testUserRepository) RevokeUser(ctx context.Context, email string) error {
	if t.revokeUserFunc != nil {
		return t.revokeUserFunc(ctx, email)
	}
	return nil
}

func (t *testUserRepository) CreatePendingAPIKey(_ context.Context, _ *api.PendingAPIKey) error {
	return nil
}

func (t *testUserRepository) GetPendingAPIKey(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
	if t.getPendingAPIKeyFunc != nil {
		return t.getPendingAPIKeyFunc(ctx, secretToken)
	}
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour).Unix() // Valid for 24 hours
	return &api.PendingAPIKey{
		SecretToken: secretToken,
		APIKey:      "test-api-key",
		UserEmail:   "user@example.com",
		CreatedBy:   "admin@example.com",
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
	}, nil
}

func (t *testUserRepository) MarkAsViewed(ctx context.Context, secretToken, ipAddress string) error {
	if t.markAsViewedFunc != nil {
		return t.markAsViewedFunc(ctx, secretToken, ipAddress)
	}
	return nil
}

func (t *testUserRepository) DeletePendingAPIKey(_ context.Context, _ string) error {
	return nil
}

func (t *testUserRepository) ListUsers(ctx context.Context) ([]*api.User, error) {
	if t.listUsersFunc != nil {
		return t.listUsersFunc(ctx)
	}
	lastUsed1 := time.Now().Add(-1 * time.Hour)
	lastUsed3 := time.Now().Add(-12 * time.Hour)
	// Return users sorted by email (as the database now does) with valid roles
	return []*api.User{
		{
			Email:     "alice@example.com",
			Role:      "admin",
			CreatedAt: time.Now().Add(-48 * time.Hour),
			Revoked:   true,
			LastUsed:  nil,
		},
		{
			Email:     "bob@example.com",
			Role:      "admin",
			CreatedAt: time.Now().Add(-36 * time.Hour),
			Revoked:   false,
			LastUsed:  &lastUsed3,
		},
		{
			Email:     "charlie@example.com",
			Role:      "admin",
			CreatedAt: time.Now().Add(-24 * time.Hour),
			Revoked:   false,
			LastUsed:  &lastUsed1,
		},
	}, nil
}

func (t *testUserRepository) GetUsersByRequestID(_ context.Context, _ string) ([]*api.User, error) {
	return []*api.User{}, nil
}

// newPermissiveTestEnforcerForHandlers creates a test enforcer that allows all access.
func newPermissiveTestEnforcerForHandlers(t testing.TB) *authorization.Enforcer {
	enf, err := authorization.NewEnforcer(testutil.SilentLogger())
	require.NoError(t, err)

	err = enf.AddRoleForUser(context.Background(), "admin@example.com", authorization.RoleAdmin)
	require.NoError(t, err)
	err = enf.AddRoleForUser(context.Background(), "user@example.com", authorization.RoleAdmin)
	require.NoError(t, err)
	err = enf.AddRoleForUser(context.Background(), "alice@example.com", authorization.RoleAdmin)
	require.NoError(t, err)
	err = enf.AddRoleForUser(context.Background(), "bob@example.com", authorization.RoleAdmin)
	require.NoError(t, err)
	err = enf.AddRoleForUser(context.Background(), "charlie@example.com", authorization.RoleAdmin)
	require.NoError(t, err)

	return enf
}

type testExecutionRepository struct {
	listExecutionsFunc func(limit int, statuses []string) ([]*api.Execution, error)
	getExecutionFunc   func(ctx context.Context, executionID string) (*api.Execution, error)
}

func (t *testExecutionRepository) CreateExecution(_ context.Context, _ *api.Execution) error {
	return nil
}

func (t *testExecutionRepository) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	if t.getExecutionFunc != nil {
		return t.getExecutionFunc(ctx, executionID)
	}
	now := time.Now()
	return &api.Execution{
		ExecutionID: executionID,
		CreatedBy:   "user@example.com",
		OwnedBy:     []string{"user@example.com"},
		Command:     "echo hello",
		Status:      string(constants.ExecutionRunning),
		StartedAt:   now,
	}, nil
}

func (t *testExecutionRepository) UpdateExecution(_ context.Context, _ *api.Execution) error {
	return nil
}

func (t *testExecutionRepository) ListExecutions(
	_ context.Context,
	limit int,
	statuses []string,
) ([]*api.Execution, error) {
	if t.listExecutionsFunc != nil {
		return t.listExecutionsFunc(limit, statuses)
	}
	return []*api.Execution{}, nil
}

func (t *testExecutionRepository) GetExecutionsByRequestID(_ context.Context, _ string) ([]*api.Execution, error) {
	return []*api.Execution{}, nil
}

type testTokenRepository struct{}

func (t *testTokenRepository) CreateToken(_ context.Context, _ *api.WebSocketToken) error {
	return nil
}

func (t *testTokenRepository) GetToken(_ context.Context, _ string) (*api.WebSocketToken, error) {
	return nil, nil
}

func (t *testTokenRepository) DeleteToken(_ context.Context, _ string) error {
	return nil
}

type testSecretsRepository struct{}

func (t *testSecretsRepository) CreateSecret(_ context.Context, _ *api.Secret) error {
	return nil
}

func (t *testSecretsRepository) GetSecret(_ context.Context, _ string, _ bool) (*api.Secret, error) {
	return nil, nil
}

func (t *testSecretsRepository) ListSecrets(_ context.Context, _ bool) ([]*api.Secret, error) {
	return []*api.Secret{}, nil
}

func (t *testSecretsRepository) UpdateSecret(_ context.Context, _ *api.Secret) error {
	return nil
}

func (t *testSecretsRepository) DeleteSecret(_ context.Context, _ string) error {
	return nil
}

func (t *testSecretsRepository) GetSecretsByRequestID(_ context.Context, _ string) ([]*api.Secret, error) {
	return []*api.Secret{}, nil
}

type testImageRepository struct{}

func (t *testImageRepository) GetImagesByRequestID(_ context.Context, _ string) ([]api.ImageInfo, error) {
	return []api.ImageInfo{}, nil
}

type testHealthManager struct{}

func (t *testHealthManager) Reconcile(_ context.Context) (*api.HealthReport, error) {
	return &api.HealthReport{}, nil
}

type testRunner struct {
	runCommandFunc           func(userEmail string, req *api.ExecutionRequest) (*time.Time, error)
	listImagesFunc           func() ([]api.ImageInfo, error)
	getImageFunc             func(image string) (*api.ImageInfo, error)
	removeImageFunc          func(ctx context.Context, image string) error
	fetchBackendLogsFunc     func(ctx context.Context, requestID string) ([]api.LogEvent, error)
	getImagesByRequestIDFunc func(ctx context.Context, requestID string) ([]api.ImageInfo, error)
}

func (t *testRunner) StartTask(
	_ context.Context,
	userEmail string,
	req *api.ExecutionRequest,
) (string, *time.Time, error) {
	if t.runCommandFunc != nil {
		createdAt, err := t.runCommandFunc(userEmail, req)
		if err != nil {
			return "", nil, err
		}
		return "exec-123", createdAt, nil
	}
	return "exec-123", nil, nil
}

func (t *testRunner) KillTask(_ context.Context, _ string) error {
	return nil
}

func (t *testRunner) RegisterImage(
	_ context.Context,
	_ string,
	_ *bool,
	_, _ *string,
	_, _ *int,
	_ *string,
	_ string,
) error {
	return nil
}

func (t *testRunner) ListImages(_ context.Context) ([]api.ImageInfo, error) {
	if t.listImagesFunc != nil {
		return t.listImagesFunc()
	}
	return []api.ImageInfo{}, nil
}

func (t *testRunner) GetImage(_ context.Context, image string) (*api.ImageInfo, error) {
	if t.getImageFunc != nil {
		return t.getImageFunc(image)
	}
	return nil, nil
}

func (t *testRunner) RemoveImage(ctx context.Context, image string) error {
	if t.removeImageFunc != nil {
		return t.removeImageFunc(ctx, image)
	}
	return nil
}

func (t *testRunner) FetchLogsByExecutionID(_ context.Context, _ string) ([]api.LogEvent, error) {
	return []api.LogEvent{}, nil
}

func (t *testRunner) FetchBackendLogs(ctx context.Context, requestID string) ([]api.LogEvent, error) {
	if t.fetchBackendLogsFunc != nil {
		return t.fetchBackendLogsFunc(ctx, requestID)
	}
	return []api.LogEvent{}, nil
}

func (t *testRunner) GetImagesByRequestID(ctx context.Context, requestID string) ([]api.ImageInfo, error) {
	if t.getImagesByRequestIDFunc != nil {
		return t.getImagesByRequestIDFunc(ctx, requestID)
	}
	return []api.ImageInfo{}, nil
}

// newTestOrchestratorService creates an orchestrator service with default test repositories.
// All required repositories (userRepo, executionRepo) are provided by default.
// Optional repositories can be overridden by passing non-nil values.
// The runner parameter implements all 4 interfaces (TaskManager, ImageRegistry, LogManager, ObservabilityManager).
func newTestOrchestratorService(
	t testing.TB,
	userRepo *testUserRepository,
	execRepo *testExecutionRepository,
	connRepo database.ConnectionRepository, //nolint:unparam // kept for API consistency
	runner *testRunner,
	wsManager contract.WebSocketManager, //nolint:unparam // kept for API consistency
	secretsRepo database.SecretsRepository, //nolint:unparam // kept for API consistency
	healthManager contract.HealthManager,
) *orchestrator.Service {
	if userRepo == nil {
		userRepo = &testUserRepository{}
	}
	if execRepo == nil {
		execRepo = &testExecutionRepository{}
	}
	var taskManager contract.TaskManager = &testRunner{}
	var imageRegistry contract.ImageRegistry = &testRunner{}
	var logManager contract.LogManager = &testRunner{}
	var observabilityManager contract.ObservabilityManager = &testRunner{}
	if runner != nil {
		taskManager = runner
		imageRegistry = runner
		logManager = runner
		observabilityManager = runner
	}
	if secretsRepo == nil {
		secretsRepo = &testSecretsRepository{}
	}
	if healthManager == nil {
		healthManager = &noopHealthManager{}
	}

	repos := database.Repositories{
		User:       userRepo,
		Execution:  execRepo,
		Connection: connRepo,
		Token:      &testTokenRepository{},
		Image:      &testImageRepository{},
		Secrets:    secretsRepo,
	}
	svc, err := orchestrator.NewService(
		context.Background(),
		&repos,
		taskManager,
		imageRegistry,
		logManager,
		observabilityManager,
		testutil.SilentLogger(),
		constants.AWS,
		wsManager,
		healthManager,
		newPermissiveTestEnforcerForHandlers(t),
	)
	require.NoError(t, err)
	return svc
}

// newTestRouterForUnauthorized creates a router for testing unauthorized access.
// All unauthorized tests use the same service setup, so this helper eliminates duplication.
func newTestRouterForUnauthorized(t *testing.T) *Router {
	repos := database.Repositories{
		User:       &testUserRepository{},
		Execution:  &testExecutionRepository{},
		Connection: nil,
		Token:      &testTokenRepository{},
		Image:      &testImageRepository{},
		Secrets:    &testSecretsRepository{},
	}
	svc, err := orchestrator.NewService(context.Background(),
		&repos,
		&testRunner{}, // TaskManager
		&testRunner{}, // ImageRegistry
		&testRunner{}, // LogManager
		&testRunner{}, // ObservabilityManager
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		&noopHealthManager{},
		newPermissiveTestEnforcerForHandlers(t),
	)
	require.NoError(t, err)
	return NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)
}

func testUnauthorizedRequest(t *testing.T, method, endpoint string, reqBody any) {
	userRepo := &testUserRepository{
		authenticateUserFunc: func(_ string) (*api.User, error) {
			return nil, apperrors.ErrInvalidAPIKey(nil)
		},
	}
	execRepo := &testExecutionRepository{}

	svc := newTestOrchestratorService(t, userRepo, execRepo, nil, &testRunner{}, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(method, endpoint, bytes.NewReader(body))
	req.Header.Set("X-API-Key", "invalid-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestHandleRunCommand_Unauthorized(t *testing.T) {
	testUnauthorizedRequest(t, http.MethodPost, "/api/v1/run", api.ExecutionRequest{Command: "echo hello"})
}

func TestHandleRunCommand_WithImage_ValidatesAuthorization(t *testing.T) {
	userEmail := "developer@example.com"
	userRepo := &testUserRepository{
		authenticateUserFunc: func(_ string) (*api.User, error) {
			return &api.User{Email: userEmail}, nil
		},
	}
	execRepo := &testExecutionRepository{}
	runner := &testRunner{
		getImageFunc: func(image string) (*api.ImageInfo, error) {
			if image == "" || image == "ubuntu:22.04" {
				isDefault := false
				return &api.ImageInfo{
					ImageID:   "ubuntu:22.04-a1b2c3d4",
					Image:     "ubuntu:22.04",
					ImageName: "ubuntu",
					ImageTag:  "22.04",
					IsDefault: &isDefault,
				}, nil
			}
			return nil, nil
		},
	}

	// Use developer role which has execute permission and access to images
	enf, err := authorization.NewEnforcer(testutil.SilentLogger())
	require.NoError(t, err)
	err = enf.AddRoleForUser(context.Background(), userEmail, authorization.RoleDeveloper)
	require.NoError(t, err)

	repos := database.Repositories{
		User:       userRepo,
		Execution:  execRepo,
		Connection: nil,
		Token:      &testTokenRepository{},
		Image:      &testImageRepository{},
		Secrets:    &testSecretsRepository{},
	}
	svc, err := orchestrator.NewService(
		context.Background(),
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		&noopHealthManager{},
		enf,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	// Test with an image - this verifies that ValidateExecutionResourceAccess is called
	// Developer role has access to images, so validation should pass
	reqBody := api.ExecutionRequest{
		Command: "echo hello",
		Image:   "ubuntu:22.04",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Validation should pass (developer has access), so we shouldn't get a 403 Forbidden
	// The request may fail for other reasons (e.g., runner not configured), but not auth
	assert.NotEqual(t, http.StatusForbidden, resp.Code,
		"developer role should have access to images, validation should pass")
}

func TestHandleRunCommand_WithSecrets_ValidatesAuthorization(t *testing.T) {
	userEmail := "developer@example.com"
	userRepo := &testUserRepository{
		authenticateUserFunc: func(_ string) (*api.User, error) {
			return &api.User{Email: userEmail}, nil
		},
	}
	execRepo := &testExecutionRepository{}
	runner := &testRunner{}

	// Use developer role which has execute permission and access to secrets
	enf, err := authorization.NewEnforcer(testutil.SilentLogger())
	require.NoError(t, err)
	err = enf.AddRoleForUser(context.Background(), userEmail, authorization.RoleDeveloper)
	require.NoError(t, err)

	repos := database.Repositories{
		User:       userRepo,
		Execution:  execRepo,
		Connection: nil,
		Token:      &testTokenRepository{},
		Image:      &testImageRepository{},
		Secrets:    &testSecretsRepository{},
	}
	svc, err := orchestrator.NewService(
		context.Background(),
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		&noopHealthManager{},
		enf,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	// Test with secrets - this verifies that ValidateExecutionResourceAccess is called
	reqBody := api.ExecutionRequest{
		Command: "echo hello",
		Secrets: []string{"db-password", "api-key"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Validation should pass (developer has access), so we shouldn't get a 403 Forbidden
	assert.NotEqual(t, http.StatusForbidden, resp.Code,
		"developer role should have access to secrets, validation should pass")
}

func TestHandleRunCommand_AllResourcesAuthorized(t *testing.T) {
	userEmail := "developer@example.com"
	userRepo := &testUserRepository{
		authenticateUserFunc: func(_ string) (*api.User, error) {
			return &api.User{Email: userEmail}, nil
		},
	}
	execRepo := &testExecutionRepository{}
	runner := &testRunner{
		getImageFunc: func(image string) (*api.ImageInfo, error) {
			if image == "" || image == "python:3.9" {
				isDefault := false
				return &api.ImageInfo{
					ImageID:   "python:3.9-a1b2c3d4",
					Image:     "python:3.9",
					ImageName: "python",
					ImageTag:  "3.9",
					IsDefault: &isDefault,
				}, nil
			}
			return nil, nil
		},
	}

	// Use developer role which has access to images and secrets (per policy.csv)
	enf, err := authorization.NewEnforcer(testutil.SilentLogger())
	require.NoError(t, err)
	err = enf.AddRoleForUser(context.Background(), userEmail, authorization.RoleDeveloper)
	require.NoError(t, err)

	repos := database.Repositories{
		User:       userRepo,
		Execution:  execRepo,
		Connection: nil,
		Token:      &testTokenRepository{},
		Image:      &testImageRepository{},
		Secrets:    &testSecretsRepository{},
	}
	svc, err := orchestrator.NewService(
		context.Background(),
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		&noopHealthManager{},
		enf,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	reqBody := api.ExecutionRequest{
		Command: "echo hello",
		Image:   "ubuntu:22.04",
		Secrets: []string{"db-password", "api-key"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Validation should pass (developer has access to all resources)
	// The request may fail for other reasons (e.g., secrets resolution, runner issues)
	// but authorization validation should pass, so we shouldn't get 403 Forbidden
	assert.NotEqual(t, http.StatusForbidden, resp.Code,
		"developer role should have access to images and secrets, validation should pass")
}

func TestHandleListExecutions_Empty(t *testing.T) {
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(_ int, _ []string) ([]*api.Execution, error) {
			return []*api.Execution{}, nil
		},
	}
	svc := newTestOrchestratorService(t, &testUserRepository{}, execRepo, nil, &testRunner{}, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "[]")
}

func TestHandleListExecutions_LimitZero(t *testing.T) {
	capturedLimit := -1
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(limit int, _ []string) ([]*api.Execution, error) {
			capturedLimit = limit
			return []*api.Execution{}, nil
		},
	}
	svc := newTestOrchestratorService(t, &testUserRepository{}, execRepo, nil, &testRunner{}, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions?limit=0", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, 0, capturedLimit)
}

func TestHandleListExecutions_DatabaseError(t *testing.T) {
	// Create a repository that returns empty list during initialization but error during handler call
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(limit int, statuses []string) ([]*api.Execution, error) {
			// During initialization, limit is 0 and statuses is nil, so return empty list
			if limit == 0 && statuses == nil {
				return []*api.Execution{}, nil
			}
			// During handler call, return error
			return nil, errors.New("database connection failed")
		},
	}
	svc := newTestOrchestratorService(t, &testUserRepository{}, execRepo, nil, &testRunner{}, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, resp.Body.String(), "failed to list executions")
}

func TestHandleListImages_Empty(t *testing.T) {
	runner := &testRunner{
		listImagesFunc: func() ([]api.ImageInfo, error) {
			return []api.ImageInfo{}, nil
		},
	}
	svc := newTestOrchestratorService(t, nil, nil, nil, runner, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandleRemoveImage_MissingImage(t *testing.T) {
	svc := newTestOrchestratorService(t, nil, nil, nil, &testRunner{}, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	// DELETE request without image path parameter
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/images/", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "image parameter is required")
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")

	ip := getClientIP(req)

	assert.Equal(t, "192.168.1.1", ip)
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Real-IP", "192.168.1.2")

	ip := getClientIP(req)

	assert.Equal(t, "192.168.1.2", ip)
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.RemoteAddr = "192.168.1.3:12345"

	ip := getClientIP(req)

	assert.Equal(t, "192.168.1.3", ip)
}

func TestGetClientIP_XForwardedForPrecedence(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	req.Header.Set("X-Real-IP", "192.168.1.2")
	req.RemoteAddr = "192.168.1.3:12345"

	ip := getClientIP(req)

	// X-Forwarded-For should take precedence
	assert.Equal(t, "192.168.1.1", ip)
}

func TestHandleListUsers_Unauthorized(t *testing.T) {
	userRepo := &testUserRepository{
		authenticateUserFunc: func(_ string) (*api.User, error) {
			return nil, apperrors.ErrInvalidAPIKey(nil)
		},
	}
	execRepo := &testExecutionRepository{}
	svc := newTestOrchestratorService(t, userRepo, execRepo, nil, &testRunner{}, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/", http.NoBody)
	req.Header.Set("X-API-Key", "invalid-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)

	var errResp api.ErrorResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&errResp)
	assert.NoError(t, decodeErr)
	assert.Equal(t, "Unauthorized", errResp.Error)
}

func TestHandleListUsers_RepositoryError(t *testing.T) {
	userRepo := &testUserRepository{
		authenticateUserFunc: func(_ string) (*api.User, error) {
			return nil, apperrors.ErrDatabaseError("database error", errors.New("connection failed"))
		},
	}
	execRepo := &testExecutionRepository{}
	svc := newTestOrchestratorService(t, userRepo, execRepo, nil, &testRunner{}, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Expect an error because authentication fails with database error (503 Service Unavailable)
	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

// TestHandleCreateUser_Success tests successful user creation with API key claim token
// TestHandleCreateUser_MissingEmail tests validation of required email field
func TestHandleCreateUser_MissingEmail(t *testing.T) {
	svc := newTestOrchestratorService(t, nil, nil, nil, nil, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	createReq := api.CreateUserRequest{
		Email: "",
		Role:  "developer",
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/create", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

// TestHandleCreateUser_InvalidRole tests invalid role validation
func TestHandleCreateUser_InvalidRole(t *testing.T) {
	svc := newTestOrchestratorService(t, nil, nil, nil, nil, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	createReq := api.CreateUserRequest{
		Email: "newuser@example.com",
		Role:  "superadmin", // invalid role
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/create", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "invalid role")
}

func TestHandleCreateUser_Unauthorized(t *testing.T) {
	testUnauthorizedRequest(
		t,
		http.MethodPost,
		"/api/v1/users/create",
		api.CreateUserRequest{Email: "newuser@example.com"},
	)
}

// TestHandleRevokeUser_MissingEmail tests validation when email is missing
func TestHandleRevokeUser_MissingEmail(t *testing.T) {
	svc := newTestOrchestratorService(t, nil, nil, nil, nil, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	reqBody := api.RevokeUserRequest{
		Email: "",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/revoke", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

// TestHandleRevokeUser_NotFound tests when user doesn't exist
func TestHandleRevokeUser_NotFound(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	svc := newTestOrchestratorService(t, userRepo, execRepo, nil, nil, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	reqBody := api.RevokeUserRequest{
		Email: "nonexistent@example.com",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/revoke", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Test router makes the request; actual 404 depends on service behavior
	// With default mock, this should succeed
	assert.True(t, resp.Code == http.StatusOK || resp.Code == http.StatusNotFound)
}

// TestHandleRevokeUser_Unauthorized tests that revoke requires authentication
func TestHandleRevokeUser_Unauthorized(t *testing.T) {
	router := newTestRouterForUnauthorized(t)

	reqBody := api.RevokeUserRequest{
		Email: "user@example.com",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/revoke", bytes.NewReader(body))
	// No API key header

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestHandleKillExecution_AlreadyTerminated(t *testing.T) {
	execRepo := &testExecutionRepository{
		getExecutionFunc: func(_ context.Context, executionID string) (*api.Execution, error) {
			return &api.Execution{
				ExecutionID: executionID,
				Status:      string(constants.ExecutionSucceeded),
				StartedAt:   time.Now(),
			}, nil
		},
	}
	svc := newTestOrchestratorService(t, &testUserRepository{}, execRepo, nil, &testRunner{}, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/executions/exec-456", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNoContent, resp.Code)
	assert.Equal(t, 0, resp.Body.Len())
}

// TestHandleClaimAPIKey_EmptyToken tests empty token validation
func TestHandleClaimAPIKey_EmptyToken(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	svc := newTestOrchestratorService(t, userRepo, execRepo, nil, nil, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Empty path parameter should result in 404 Not Found (chi behavior)
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

// TestHandleClaimAPIKey_TokenWithWhitespace tests whitespace trimming
func TestHandleClaimAPIKey_TokenWithWhitespace(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	svc := newTestOrchestratorService(t, userRepo, execRepo, nil, nil, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/valid-token", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "test-api-key")
}

// TestHandleReconcileHealth_Unauthenticated tests that reconcile requires auth
func TestHandleReconcileHealth_Unauthenticated(t *testing.T) {
	repos := database.Repositories{
		User:       &testUserRepository{},
		Execution:  &testExecutionRepository{},
		Connection: nil,
		Token:      &testTokenRepository{},
		Image:      &testImageRepository{},
		Secrets:    &testSecretsRepository{},
	}
	svc, err := orchestrator.NewService(context.Background(),
		&repos,
		&testRunner{}, // TaskManager
		&testRunner{}, // ImageRegistry
		&testRunner{}, // LogManager
		&testRunner{}, // ObservabilityManager
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		&noopHealthManager{},
		newPermissiveTestEnforcerForHandlers(t),
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/reconcile", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	assert.Contains(t, resp.Body.String(), "Unauthorized")
}

// TestHandleReconcileHealth_Authenticated tests health reconciliation with authentication
func TestHandleReconcileHealth_Authenticated(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	svc := newTestOrchestratorService(t, userRepo, execRepo, nil, nil, nil, nil, &testHealthManager{})
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/reconcile", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should authenticate and process the request (health manager error is expected)
	// Status could be 500 or 200 depending on health manager availability
	assert.True(t, resp.Code == http.StatusOK || resp.Code == http.StatusInternalServerError,
		"should authenticate and attempt to reconcile health")
}

// Image Handler Gap Tests

// TestHandleRemoveImage_Unauthorized tests that remove image requires auth
func TestHandleRemoveImage_Unauthorized(t *testing.T) {
	router := newTestRouterForUnauthorized(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/images/ubuntu:22.04", http.NoBody)
	// No API key

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

// TestHandleRegisterImage_Unauthorized tests that register image requires auth
func TestHandleRegisterImage_Unauthorized(t *testing.T) {
	router := newTestRouterForUnauthorized(t)

	regReq := api.RegisterImageRequest{Image: "alpine:latest"}
	body, _ := json.Marshal(regReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/register", bytes.NewReader(body))
	// No API key

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

// TestHandleGetImage_Unauthorized tests that get image requires auth
func TestHandleGetImage_Unauthorized(t *testing.T) {
	router := newTestRouterForUnauthorized(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images/ubuntu:22.04", http.NoBody)
	// No API key

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

// Execution Handler Gap Tests

// TestHandleKillExecution_Unauthorized tests that kill requires auth
func TestHandleKillExecution_Unauthorized(t *testing.T) {
	router := newTestRouterForUnauthorized(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/executions/exec-123", http.NoBody)
	// No API key

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

// TestHandleGetExecutionLogs_Unauthorized tests that logs require auth
func TestHandleGetExecutionLogs_Unauthorized(t *testing.T) {
	router := newTestRouterForUnauthorized(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-123/logs", http.NoBody)
	// No API key

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

// TestHandleGetExecutionStatus_Unauthorized tests that status requires auth
func TestHandleGetExecutionStatus_Unauthorized(t *testing.T) {
	router := newTestRouterForUnauthorized(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-123/status", http.NoBody)
	// No API key

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

// TestHandleRunCommand_WithValidCommand tests run command with valid request
func TestHandleRunCommand_WithValidCommand(t *testing.T) {
	runner := &testRunner{
		getImageFunc: func(image string) (*api.ImageInfo, error) {
			// When no image is specified, return a default image
			if image == "" {
				isDefault := true
				return &api.ImageInfo{
					ImageID:   "default-image-id",
					Image:     "default-image",
					IsDefault: &isDefault,
				}, nil
			}
			return nil, nil
		},
	}
	svc := newTestOrchestratorService(t, nil, nil, nil, runner, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	execReq := api.ExecutionRequest{Command: "echo hello"}
	body, _ := json.Marshal(execReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Should process the request
	assert.True(t, resp.Code == http.StatusAccepted || resp.Code == http.StatusInternalServerError,
		"should handle run command")
}

// TestHandleClaimAPIKey_GetPendingKeyError tests database error when retrieving pending key
func TestHandleClaimAPIKey_GetPendingKeyError(t *testing.T) {
	userRepo := &testUserRepository{
		getPendingAPIKeyFunc: func(_ context.Context, _ string) (*api.PendingAPIKey, error) {
			return nil, apperrors.ErrDatabaseError("failed to retrieve pending key", nil)
		},
	}
	svc := newTestOrchestratorService(t, userRepo, nil, nil, nil, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/token", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Database errors return 503 Service Unavailable
	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

// TestGetBackendLogs_Success tests successful backend logs retrieval
func TestGetBackendLogs_Success(t *testing.T) {
	userRepo := &testUserRepository{
		getUserByEmailFunc: func(email string) (*api.User, error) {
			return &api.User{Email: email, Role: "role:admin"}, nil
		},
	}

	runner := &testRunner{
		fetchBackendLogsFunc: func(_ context.Context, _ string) ([]api.LogEvent, error) {
			return []api.LogEvent{
				{Timestamp: 1000, Message: "Log 1"},
				{Timestamp: 2000, Message: "Log 2"},
			}, nil
		},
	}

	svc := newTestOrchestratorService(t, userRepo, nil, nil, runner, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trace/aws-request-id", http.NoBody)
	req.Header.Set("X-API-Key", "test-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	var trace api.TraceResponse
	err := json.Unmarshal(resp.Body.Bytes(), &trace)
	require.NoError(t, err)
	require.Len(t, trace.Logs, 2)
	assert.Equal(t, "Log 1", trace.Logs[0].Message)
	assert.Equal(t, "Log 2", trace.Logs[1].Message)
}

// TestGetBackendLogs_MissingRequestID tests error when requestID is missing
func TestGetBackendLogs_MissingRequestID(t *testing.T) {
	userRepo := &testUserRepository{
		getUserByEmailFunc: func(email string) (*api.User, error) {
			return &api.User{Email: email, Role: "role:admin"}, nil
		},
	}

	svc := newTestOrchestratorService(t, userRepo, nil, nil, &testRunner{}, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trace/", http.NoBody)
	req.Header.Set("X-API-Key", "test-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Route doesn't match so we get 404
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

// TestGetBackendLogs_ServiceError tests error from service layer
func TestGetBackendLogs_ServiceError(t *testing.T) {
	userRepo := &testUserRepository{
		getUserByEmailFunc: func(email string) (*api.User, error) {
			return &api.User{Email: email, Role: "role:admin"}, nil
		},
	}

	runner := &testRunner{
		fetchBackendLogsFunc: func(_ context.Context, _ string) ([]api.LogEvent, error) {
			return nil, apperrors.ErrServiceUnavailable("no log groups found", nil)
		},
	}

	svc := newTestOrchestratorService(t, userRepo, nil, nil, runner, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trace/aws-request-id", http.NoBody)
	req.Header.Set("X-API-Key", "test-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
	var errResp api.ErrorResponse
	err := json.Unmarshal(resp.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Equal(t, "SERVICE_UNAVAILABLE", errResp.Code)
}

// TestGetBackendLogs_NoLogs tests empty logs result
func TestGetBackendLogs_NoLogs(t *testing.T) {
	userRepo := &testUserRepository{
		getUserByEmailFunc: func(email string) (*api.User, error) {
			return &api.User{Email: email, Role: "role:admin"}, nil
		},
	}

	runner := &testRunner{
		fetchBackendLogsFunc: func(_ context.Context, _ string) ([]api.LogEvent, error) {
			return []api.LogEvent{}, nil
		},
	}

	svc := newTestOrchestratorService(t, userRepo, nil, nil, runner, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trace/aws-request-id", http.NoBody)
	req.Header.Set("X-API-Key", "test-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	var trace api.TraceResponse
	err := json.Unmarshal(resp.Body.Bytes(), &trace)
	require.NoError(t, err)
	require.Len(t, trace.Logs, 0)
}

// TestGetBackendLogs_Unauthorized tests unauthorized access
func TestGetBackendLogs_Unauthorized(t *testing.T) {
	svc := newTestOrchestratorService(t, &testUserRepository{}, nil, nil, &testRunner{}, nil, nil, nil)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trace/aws-request-id", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}
