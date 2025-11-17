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
	"runvoy/internal/backend/orchestrator"
	"runvoy/internal/constants"
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
}

func (t *testUserRepository) CreateUser(_ context.Context, _ *api.User, _ string, _ int64) error {
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

func (t *testUserRepository) RevokeUser(_ context.Context, _ string) error {
	return nil
}

func (t *testUserRepository) CreatePendingAPIKey(_ context.Context, _ *api.PendingAPIKey) error {
	return nil
}

func (t *testUserRepository) GetPendingAPIKey(_ context.Context, secretToken string) (*api.PendingAPIKey, error) {
	now := time.Now()
	return &api.PendingAPIKey{
		SecretToken: secretToken,
		APIKey:      "test-api-key",
		UserEmail:   "user@example.com",
		CreatedBy:   "admin@example.com",
		CreatedAt:   now,
	}, nil
}

func (t *testUserRepository) MarkAsViewed(_ context.Context, _, _ string) error {
	return nil
}

func (t *testUserRepository) DeletePendingAPIKey(_ context.Context, _ string) error {
	return nil
}

func (t *testUserRepository) ListUsers(_ context.Context) ([]*api.User, error) {
	lastUsed1 := time.Now().Add(-1 * time.Hour)
	lastUsed3 := time.Now().Add(-12 * time.Hour)
	// Return users sorted by email (as the database now does)
	return []*api.User{
		{
			Email:     "alice@example.com",
			CreatedAt: time.Now().Add(-48 * time.Hour),
			Revoked:   true,
			LastUsed:  nil,
		},
		{
			Email:     "bob@example.com",
			CreatedAt: time.Now().Add(-36 * time.Hour),
			Revoked:   false,
			LastUsed:  &lastUsed3,
		},
		{
			Email:     "charlie@example.com",
			CreatedAt: time.Now().Add(-24 * time.Hour),
			Revoked:   false,
			LastUsed:  &lastUsed1,
		},
	}, nil
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
		UserEmail:   "user@example.com",
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

type testRunner struct {
	runCommandFunc  func(userEmail string, req *api.ExecutionRequest) (*time.Time, error)
	listImagesFunc  func() ([]api.ImageInfo, error)
	getImageFunc    func(image string) (*api.ImageInfo, error)
	removeImageFunc func(ctx context.Context, image string) error
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

func TestHandleHealth(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		nil,
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", http.NoBody)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "status")
	assert.Contains(t, resp.Body.String(), "ok")
}

func TestHandleRunCommand_Success(t *testing.T) {
	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	runner := &testRunner{}

	svc, err := orchestrator.NewService(context.Background(),
		userRepo,
		execRepo,
		nil,
		&testTokenRepository{},
		runner,
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	reqBody := api.ExecutionRequest{
		Command: "echo hello",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusAccepted, resp.Code)

	var execResp api.ExecutionResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&execResp)
	assert.NoError(t, decodeErr)
	assert.NotEmpty(t, execResp.ExecutionID)
	assert.Equal(t, string(constants.ExecutionStarting), execResp.Status)
}

func TestHandleRunCommand_InvalidJSON(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "invalid request body")
}

func testUnauthorizedRequest(t *testing.T, method, endpoint string, reqBody any) {
	userRepo := &testUserRepository{
		authenticateUserFunc: func(_ string) (*api.User, error) {
			return nil, apperrors.ErrInvalidAPIKey(nil)
		},
	}

	svc, err := orchestrator.NewService(context.Background(),
		userRepo,
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

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

func TestHandleListExecutions_Success(t *testing.T) {
	now := time.Now()
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(_ int, _ []string) ([]*api.Execution, error) {
			return []*api.Execution{
				{
					ExecutionID: "exec-1",
					UserEmail:   "user@example.com",
					Command:     "echo hello",
					Status:      string(constants.ExecutionRunning),
					StartedAt:   now,
				},
			}, nil
		},
	}

	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		execRepo,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var executions []*api.Execution
	decodeErr := json.NewDecoder(resp.Body).Decode(&executions)
	assert.NoError(t, decodeErr)
	assert.Len(t, executions, 1)
	assert.Equal(t, "exec-1", executions[0].ExecutionID)
}

func TestHandleListExecutions_Empty(t *testing.T) {
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(_ int, _ []string) ([]*api.Execution, error) {
			return []*api.Execution{}, nil
		},
	}

	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		execRepo,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

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

	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		execRepo,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions?limit=0", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, 0, capturedLimit)
}

func TestHandleListExecutions_DatabaseError(t *testing.T) {
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(_ int, _ []string) ([]*api.Execution, error) {
			return nil, errors.New("database connection failed")
		},
	}

	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		execRepo,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, resp.Body.String(), "failed to list executions")
}

func TestHandleRegisterImage_Success(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	reqBody := api.RegisterImageRequest{
		Image: "alpine:latest",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/register", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusCreated, resp.Code)
	assert.Contains(t, resp.Body.String(), "alpine:latest")
	assert.Contains(t, resp.Body.String(), "Image registered successfully")
}

func TestHandleRegisterImage_InvalidJSON(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/register", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "invalid request body")
}

func TestHandleListImages_Success(t *testing.T) {
	runner := &testRunner{
		listImagesFunc: func() ([]api.ImageInfo, error) {
			return []api.ImageInfo{
				{Image: "alpine:latest"},
				{Image: "ubuntu:22.04"},
			}, nil
		},
	}

	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		nil,
		nil,
		&testTokenRepository{},
		runner,
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "alpine:latest")
	assert.Contains(t, resp.Body.String(), "ubuntu:22.04")
}

func TestHandleListImages_Empty(t *testing.T) {
	runner := &testRunner{
		listImagesFunc: func() ([]api.ImageInfo, error) {
			return []api.ImageInfo{}, nil
		},
	}

	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		nil,
		nil,
		&testTokenRepository{},
		runner,
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandleRemoveImage_Success(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	reqBody := api.RemoveImageRequest{
		Image: "alpine:latest",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/images/alpine:latest", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "Image removed successfully")
}

func TestHandleRemoveImage_NotFound(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{
			removeImageFunc: func(_ context.Context, _ string) error {
				return apperrors.ErrNotFound("image not found", nil)
			},
		},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/images/nonexistent:latest", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Contains(t, resp.Body.String(), "image not found")
}

func TestHandleRemoveImage_MissingImage(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	// DELETE request without image path parameter
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/images/", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "image parameter is required")
}

func TestHandleGetImage_Success(t *testing.T) {
	runner := &testRunner{
		getImageFunc: func(_ string) (*api.ImageInfo, error) {
			return &api.ImageInfo{
				Image:              "alpine:latest",
				TaskDefinitionName: "runvoy-alpine-latest",
			}, nil
		},
	}

	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		nil,
		nil,
		&testTokenRepository{},
		runner,
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images/alpine:latest", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "alpine:latest")
	assert.Contains(t, resp.Body.String(), "runvoy-alpine-latest")
}

func TestHandleGetImage_NotFound(t *testing.T) {
	runner := &testRunner{
		getImageFunc: func(_ string) (*api.ImageInfo, error) {
			return nil, errors.New("image not found")
		},
	}

	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		nil,
		nil,
		&testTokenRepository{},
		runner,
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images/nonexistent:latest", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Error should return 500
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, resp.Body.String(), "failed to get image")
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

func TestHandleListUsers_Success(t *testing.T) {
	userRepo := &testUserRepository{}
	svc, err := orchestrator.NewService(context.Background(),
		userRepo,
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var listResp api.ListUsersResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&listResp)
	assert.NoError(t, decodeErr)
	assert.Len(t, listResp.Users, 3)
	// Verify users are sorted by email in ascending order
	assert.Equal(t, "alice@example.com", listResp.Users[0].Email)
	assert.Equal(t, true, listResp.Users[0].Revoked)
	assert.Equal(t, "bob@example.com", listResp.Users[1].Email)
	assert.Equal(t, false, listResp.Users[1].Revoked)
	assert.Equal(t, "charlie@example.com", listResp.Users[2].Email)
	assert.Equal(t, false, listResp.Users[2].Revoked)
}

func TestHandleListUsers_Unauthorized(t *testing.T) {
	userRepo := &testUserRepository{
		authenticateUserFunc: func(_ string) (*api.User, error) {
			return nil, apperrors.ErrInvalidAPIKey(nil)
		},
	}
	svc, err := orchestrator.NewService(context.Background(),
		userRepo,
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

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
	svc, err := orchestrator.NewService(context.Background(),
		userRepo,
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Expect an error because authentication fails with database error (503 Service Unavailable)
	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

// TODO: Add TestHandleCreateUser_Success - requires complex mock setup for admin user and pending keys

func TestHandleCreateUser_InvalidJSON(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/create", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "invalid request body")
}

func TestHandleCreateUser_Unauthorized(t *testing.T) {
	testUnauthorizedRequest(
		t,
		http.MethodPost,
		"/api/v1/users/create",
		api.CreateUserRequest{Email: "newuser@example.com"},
	)
}

func TestHandleRevokeUser_Success(t *testing.T) {
	userRepo := &testUserRepository{}
	svc, err := orchestrator.NewService(context.Background(),
		userRepo,
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	reqBody := api.RevokeUserRequest{
		Email: "user@example.com",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/revoke", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "user@example.com")
	assert.Contains(t, resp.Body.String(), "revoked successfully")
}

func TestHandleRevokeUser_InvalidJSON(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		nil,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/revoke", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "Invalid request body")
}

func TestHandleGetExecutionLogs_Success(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		&testExecutionRepository{},
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-123/logs", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var logsResp api.LogsResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&logsResp)
	assert.NoError(t, decodeErr)
}

func TestHandleGetExecutionLogs_MissingExecutionID(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		&testExecutionRepository{},
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions//logs", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "executionID is required")
}

func TestHandleGetExecutionStatus_Success(t *testing.T) {
	execRepo := &testExecutionRepository{}
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		execRepo,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-123/status", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var statusResp api.ExecutionStatusResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&statusResp)
	assert.NoError(t, decodeErr)
}

func TestHandleGetExecutionStatus_MissingExecutionID(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		&testExecutionRepository{},
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions//status", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "executionID is required")
}

func TestHandleKillExecution_Success(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		&testExecutionRepository{},
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/executions/exec-123/kill", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var killResp api.KillExecutionResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&killResp)
	assert.NoError(t, decodeErr)
	assert.Equal(t, "exec-123", killResp.ExecutionID)
	assert.Contains(t, killResp.Message, "termination initiated")
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

	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		execRepo,
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/executions/exec-456/kill", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNoContent, resp.Code)
	assert.Equal(t, 0, resp.Body.Len())
}

func TestHandleKillExecution_MissingExecutionID(t *testing.T) {
	svc, err := orchestrator.NewService(context.Background(),
		&testUserRepository{},
		&testExecutionRepository{},
		nil,
		&testTokenRepository{},
		&testRunner{},
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil, // SecretsService
		nil, // healthManager
		nil,
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/executions//kill", http.NoBody)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "executionID is required")
}

// TODO: Add tests for handleClaimAPIKey - currently has routing issues in test environment
// The handler itself has logic at handlers.go:263 that needs test coverage
