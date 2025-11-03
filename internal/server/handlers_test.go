package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/app"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	rlogger "runvoy/internal/logger"

	"github.com/stretchr/testify/assert"
)

// Test mocks for repositories and runner
type testUserRepository struct {
	authenticateUserFunc func(apiKeyHash string) (*api.User, error)
	updateLastUsedFunc   func(email string) error
}

func (t *testUserRepository) CreateUser(ctx context.Context, user *api.User, apiKeyHash string) error {
	return nil
}

func (t *testUserRepository) CreateUserWithExpiration(ctx context.Context, user *api.User, apiKeyHash string, expiresAtUnix int64) error {
	return nil
}

func (t *testUserRepository) RemoveExpiration(ctx context.Context, email string) error {
	return nil
}

func (t *testUserRepository) GetUserByEmail(ctx context.Context, email string) (*api.User, error) {
	return nil, nil
}

func (t *testUserRepository) GetUserByAPIKeyHash(ctx context.Context, apiKeyHash string) (*api.User, error) {
	if t.authenticateUserFunc != nil {
		return t.authenticateUserFunc(apiKeyHash)
	}
	return &api.User{
		Email:   "user@example.com",
		Revoked: false,
	}, nil
}

func (t *testUserRepository) UpdateLastUsed(ctx context.Context, email string) (*time.Time, error) {
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
	return nil
}

func (t *testUserRepository) CreatePendingAPIKey(ctx context.Context, pending *api.PendingAPIKey) error {
	return nil
}

func (t *testUserRepository) GetPendingAPIKey(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
	return nil, nil
}

func (t *testUserRepository) MarkAsViewed(ctx context.Context, secretToken string, ipAddress string) error {
	return nil
}

func (t *testUserRepository) DeletePendingAPIKey(ctx context.Context, secretToken string) error {
	return nil
}

func (t *testUserRepository) ListUsers(ctx context.Context) ([]*api.User, error) {
	return []*api.User{
		{
			Email:     "charlie@example.com",
			CreatedAt: time.Now().Add(-24 * time.Hour),
			Revoked:   false,
			LastUsed:  time.Now().Add(-1 * time.Hour),
		},
		{
			Email:     "alice@example.com",
			CreatedAt: time.Now().Add(-48 * time.Hour),
			Revoked:   true,
			LastUsed:  time.Time{},
		},
		{
			Email:     "bob@example.com",
			CreatedAt: time.Now().Add(-36 * time.Hour),
			Revoked:   false,
			LastUsed:  time.Now().Add(-12 * time.Hour),
		},
	}, nil
}

type testExecutionRepository struct {
	listExecutionsFunc func() ([]*api.Execution, error)
}

func (t *testExecutionRepository) CreateExecution(ctx context.Context, execution *api.Execution) error {
	return nil
}

func (t *testExecutionRepository) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	return nil, nil
}

func (t *testExecutionRepository) UpdateExecution(ctx context.Context, execution *api.Execution) error {
	return nil
}

func (t *testExecutionRepository) ListExecutions(ctx context.Context) ([]*api.Execution, error) {
	if t.listExecutionsFunc != nil {
		return t.listExecutionsFunc()
	}
	return []*api.Execution{}, nil
}

type testRunner struct {
	runCommandFunc func(userEmail string, req api.ExecutionRequest) (*time.Time, error)
	listImagesFunc func() ([]api.ImageInfo, error)
}

func (t *testRunner) StartTask(ctx context.Context, userEmail string, req api.ExecutionRequest) (string, *time.Time, error) {
	if t.runCommandFunc != nil {
		createdAt, err := t.runCommandFunc(userEmail, req)
		if err != nil {
			return "", nil, err
		}
		return "exec-123", createdAt, nil
	}
	return "exec-123", nil, nil
}

func (t *testRunner) KillTask(ctx context.Context, executionID string) error {
	return nil
}

func (t *testRunner) RegisterImage(ctx context.Context, image string, isDefault *bool) error {
	return nil
}

func (t *testRunner) ListImages(ctx context.Context) ([]api.ImageInfo, error) {
	if t.listImagesFunc != nil {
		return t.listImagesFunc()
	}
	return []api.ImageInfo{}, nil
}

func (t *testRunner) RemoveImage(ctx context.Context, image string) error {
	return nil
}

func (t *testRunner) FetchLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error) {
	return []api.LogEvent{}, nil
}

func TestHandleHealth(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)
	svc := app.NewService(nil, nil, &testRunner{}, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "status")
	assert.Contains(t, resp.Body.String(), "ok")
}

func TestHandleRunCommand_Success(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)

	userRepo := &testUserRepository{}
	execRepo := &testExecutionRepository{}
	runner := &testRunner{}

	svc := app.NewService(userRepo, execRepo, runner, slog.Default(), constants.AWS)
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
	json.NewDecoder(resp.Body).Decode(&execResp)
	assert.NotEmpty(t, execResp.ExecutionID)
	assert.Equal(t, string(constants.ExecutionRunning), execResp.Status)
}

func TestHandleRunCommand_InvalidJSON(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)
	svc := app.NewService(&testUserRepository{}, nil, &testRunner{}, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "invalid request body")
}

func TestHandleRunCommand_Unauthorized(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)

	userRepo := &testUserRepository{
		authenticateUserFunc: func(apiKeyHash string) (*api.User, error) {
			return nil, apperrors.ErrInvalidAPIKey(nil)
		},
	}

	svc := app.NewService(userRepo, nil, &testRunner{}, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	reqBody := api.ExecutionRequest{Command: "echo hello"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "invalid-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestHandleListExecutions_Success(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)

	now := time.Now()
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func() ([]*api.Execution, error) {
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

	svc := app.NewService(&testUserRepository{}, execRepo, &testRunner{}, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions", nil)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var executions []*api.Execution
	json.NewDecoder(resp.Body).Decode(&executions)
	assert.Len(t, executions, 1)
	assert.Equal(t, "exec-1", executions[0].ExecutionID)
}

func TestHandleListExecutions_Empty(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)

	execRepo := &testExecutionRepository{
		listExecutionsFunc: func() ([]*api.Execution, error) {
			return []*api.Execution{}, nil
		},
	}

	svc := app.NewService(&testUserRepository{}, execRepo, &testRunner{}, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions", nil)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "[]")
}

func TestHandleListExecutions_DatabaseError(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)

	execRepo := &testExecutionRepository{
		listExecutionsFunc: func() ([]*api.Execution, error) {
			return nil, errors.New("database connection failed")
		},
	}

	svc := app.NewService(&testUserRepository{}, execRepo, &testRunner{}, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions", nil)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, resp.Body.String(), "failed to list executions")
}

func TestHandleRegisterImage_Success(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)
	svc := app.NewService(&testUserRepository{}, nil, &testRunner{}, slog.Default(), constants.AWS)
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
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)
	svc := app.NewService(&testUserRepository{}, nil, &testRunner{}, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/register", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "invalid request body")
}

func TestHandleListImages_Success(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)

	runner := &testRunner{
		listImagesFunc: func() ([]api.ImageInfo, error) {
			return []api.ImageInfo{
				{Image: "alpine:latest"},
				{Image: "ubuntu:22.04"},
			}, nil
		},
	}

	svc := app.NewService(&testUserRepository{}, nil, runner, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images", nil)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "alpine:latest")
	assert.Contains(t, resp.Body.String(), "ubuntu:22.04")
}

func TestHandleListImages_Empty(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)

	runner := &testRunner{
		listImagesFunc: func() ([]api.ImageInfo, error) {
			return []api.ImageInfo{}, nil
		},
	}

	svc := app.NewService(&testUserRepository{}, nil, runner, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/images", nil)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandleRemoveImage_Success(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)
	svc := app.NewService(&testUserRepository{}, nil, &testRunner{}, slog.Default(), constants.AWS)
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

func TestHandleRemoveImage_MissingImage(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)
	svc := app.NewService(&testUserRepository{}, nil, &testRunner{}, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	// DELETE request without image path parameter
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/images/", nil)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "image parameter is required")
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")

	ip := getClientIP(req)

	assert.Equal(t, "192.168.1.1", ip)
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.2")

	ip := getClientIP(req)

	assert.Equal(t, "192.168.1.2", ip)
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.3:12345"

	ip := getClientIP(req)

	assert.Equal(t, "192.168.1.3", ip)
}

func TestGetClientIP_XForwardedForPrecedence(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	req.Header.Set("X-Real-IP", "192.168.1.2")
	req.RemoteAddr = "192.168.1.3:12345"

	ip := getClientIP(req)

	// X-Forwarded-For should take precedence
	assert.Equal(t, "192.168.1.1", ip)
}

func TestHandleListUsers_Success(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)

	userRepo := &testUserRepository{}
	svc := app.NewService(userRepo, nil, &testRunner{}, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/", nil)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var listResp api.ListUsersResponse
	err := json.NewDecoder(resp.Body).Decode(&listResp)
	assert.NoError(t, err)
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
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)

	userRepo := &testUserRepository{
		authenticateUserFunc: func(apiKeyHash string) (*api.User, error) {
			return nil, apperrors.ErrInvalidAPIKey(nil)
		},
	}
	svc := app.NewService(userRepo, nil, &testRunner{}, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/", nil)
	req.Header.Set("X-API-Key", "invalid-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)

	var errResp api.ErrorResponse
	err := json.NewDecoder(resp.Body).Decode(&errResp)
	assert.NoError(t, err)
	assert.Equal(t, "Unauthorized", errResp.Error)
}

func TestHandleListUsers_RepositoryError(t *testing.T) {
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)

	userRepo := &testUserRepository{
		authenticateUserFunc: func(apiKeyHash string) (*api.User, error) {
			return nil, apperrors.ErrDatabaseError("database error", errors.New("connection failed"))
		},
	}
	svc := app.NewService(userRepo, nil, &testRunner{}, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/", nil)
	req.Header.Set("X-API-Key", "test-api-key")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	// Expect an error because authentication fails with database error (503 Service Unavailable)
	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
}
