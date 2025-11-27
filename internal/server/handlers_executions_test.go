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
	"runvoy/internal/backend/contract"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/testutil"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper to create a router with execution-related dependencies
func newExecutionHandlerRouter(
	t *testing.T,
	execRepo *testExecutionRepository,
	runner *testRunner,
) *Router {
	if execRepo == nil {
		execRepo = &testExecutionRepository{}
	}
	if runner == nil {
		runner = &testRunner{}
	}
	svc := newTestOrchestratorService(
		t,
		&testUserRepository{},
		execRepo,
		nil,
		runner,
		nil,
		nil,
		nil,
	)
	return &Router{svc: svc}
}

// ==================== handleRunCommand tests ====================

func TestHandleRunCommand_Success(t *testing.T) {
	runner := &testRunner{
		getImageFunc: func(image string) (*api.ImageInfo, error) {
			if image == "alpine:latest" {
				return &api.ImageInfo{Image: image, ImageID: "sha256:abc123"}, nil
			}
			return nil, nil
		},
		runCommandFunc: func(userEmail string, req *api.ExecutionRequest) (*time.Time, error) {
			assert.Equal(t, "user@example.com", userEmail)
			assert.Equal(t, "echo hello", req.Command)
			now := time.Now()
			return &now, nil
		},
	}
	router := newExecutionHandlerRouter(t, nil, runner)

	reqBody := api.ExecutionRequest{
		Command: "echo hello",
		Image:   "alpine:latest",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "admin",
	})

	w := httptest.NewRecorder()
	router.handleRunCommand(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response api.ExecutionResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.NotEmpty(t, response.ExecutionID)
}

func TestHandleRunCommand_NoAuthentication(t *testing.T) {
	router := newExecutionHandlerRouter(t, nil, nil)

	reqBody := api.ExecutionRequest{
		Command: "echo hello",
		Image:   "alpine:latest",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.handleRunCommand(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandleRunCommand_InvalidJSON(t *testing.T) {
	router := newExecutionHandlerRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "admin",
	})

	w := httptest.NewRecorder()
	router.handleRunCommand(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleRunCommand_WithEnvironmentVariables(t *testing.T) {
	runner := &testRunner{
		getImageFunc: func(image string) (*api.ImageInfo, error) {
			if image == "python:3.9" {
				return &api.ImageInfo{Image: image, ImageID: "sha256:xyz789"}, nil
			}
			return nil, nil
		},
		runCommandFunc: func(userEmail string, req *api.ExecutionRequest) (*time.Time, error) {
			assert.Equal(t, "user@example.com", userEmail)
			assert.Equal(t, "python script.py", req.Command)
			assert.Equal(t, "production", req.Env["ENVIRONMENT"])
			assert.Equal(t, "8080", req.Env["PORT"])
			now := time.Now()
			return &now, nil
		},
	}
	router := newExecutionHandlerRouter(t, nil, runner)

	reqBody := api.ExecutionRequest{
		Command: "python script.py",
		Image:   "python:3.9",
		Env: map[string]string{
			"ENVIRONMENT": "production",
			"PORT":        "8080",
		},
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "developer",
	})

	w := httptest.NewRecorder()
	router.handleRunCommand(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestHandleRunCommand_ReturnsWebSocketURL(t *testing.T) {
	expectedWebSocketURL := "wss://example.com/logs/exec-123?token=abc"
	wsManager := &stubWebSocketManager{
		generateURL: func(
			_ context.Context,
			executionID string,
			userEmail *string,
			clientIPAtCreationTime *string,
		) string {
			assert.Equal(t, "exec-123", executionID)
			require.NotNil(t, userEmail)
			assert.Equal(t, "user@example.com", *userEmail)
			require.NotNil(t, clientIPAtCreationTime)
			assert.Equal(t, "203.0.113.10", *clientIPAtCreationTime)
			return expectedWebSocketURL
		},
	}
	runner := &testRunner{
		getImageFunc: func(image string) (*api.ImageInfo, error) {
			return &api.ImageInfo{Image: image, ImageID: "sha256:default"}, nil
		},
	}
	svc := newTestOrchestratorService(
		t,
		&testUserRepository{},
		&testExecutionRepository{},
		nil,
		runner,
		wsManager,
		nil,
		nil,
	)
	router := &Router{svc: svc}

	reqBody := api.ExecutionRequest{
		Command: "echo hello",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "admin",
	})

	w := httptest.NewRecorder()
	router.handleRunCommand(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response api.ExecutionResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, expectedWebSocketURL, response.WebSocketURL)
}

func TestHandleRunCommand_WithTimeout(t *testing.T) {
	runner := &testRunner{
		getImageFunc: func(image string) (*api.ImageInfo, error) {
			if image == "alpine:latest" {
				return &api.ImageInfo{Image: image, ImageID: "sha256:def456"}, nil
			}
			return nil, nil
		},
		runCommandFunc: func(_ string, req *api.ExecutionRequest) (*time.Time, error) {
			assert.Equal(t, 300, req.Timeout)
			now := time.Now()
			return &now, nil
		},
	}
	router := newExecutionHandlerRouter(t, nil, runner)

	reqBody := api.ExecutionRequest{
		Command: "long-running-command",
		Image:   "alpine:latest",
		Timeout: 300,
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "developer",
	})

	w := httptest.NewRecorder()
	router.handleRunCommand(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

type stubWebSocketManager struct {
	generateURL func(
		ctx context.Context,
		executionID string,
		userEmail *string,
		clientIPAtCreationTime *string,
	) string
}

var _ contract.WebSocketManager = (*stubWebSocketManager)(nil)

func (s *stubWebSocketManager) HandleRequest(
	_ context.Context,
	_ *json.RawMessage,
	_ *slog.Logger,
) (bool, error) {
	return false, nil
}

func (s *stubWebSocketManager) NotifyExecutionCompletion(_ context.Context, _ *string) error {
	return nil
}

func (s *stubWebSocketManager) SendLogsToExecution(
	_ context.Context,
	_ *string,
	_ []api.LogEvent,
) error {
	return nil
}

func (s *stubWebSocketManager) GenerateWebSocketURL(
	ctx context.Context,
	executionID string,
	userEmail *string,
	clientIPAtCreationTime *string,
) string {
	if s.generateURL != nil {
		return s.generateURL(ctx, executionID, userEmail, clientIPAtCreationTime)
	}
	return ""
}

// ==================== handleGetExecutionLogs tests ====================

func TestHandleGetExecutionLogs_Success(t *testing.T) {
	runner := &testRunner{}
	router := newExecutionHandlerRouter(t, nil, runner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-123/logs", http.NoBody)
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "developer",
	})

	// Set up chi route context with execution ID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("executionID", "exec-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetExecutionLogs(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleGetExecutionLogs_NoAuthentication(t *testing.T) {
	router := newExecutionHandlerRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-123/logs", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("executionID", "exec-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetExecutionLogs(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandleGetExecutionLogs_MissingExecutionID(t *testing.T) {
	router := newExecutionHandlerRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions//logs", http.NoBody)
	req = addAuthenticatedUser(req, &api.User{
		Email: "user@example.com",
		Role:  "developer",
	})

	// Set up chi route context with empty execution ID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("executionID", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetExecutionLogs(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ==================== handleGetBackendLogsTrace tests ====================

func TestHandleGetBackendLogsTrace_Success(t *testing.T) {
	runner := &testRunner{
		fetchBackendLogsFunc: func(_ context.Context, requestID string) ([]api.LogEvent, error) {
			assert.Equal(t, "req-123", requestID)
			return []api.LogEvent{
				{
					Message:   "test log",
					Timestamp: time.Now().UnixMilli(),
				},
			}, nil
		},
	}
	router := newExecutionHandlerRouter(t, nil, runner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trace/req-123", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("requestID", "req-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetBackendLogsTrace(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response api.TraceResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.NotNil(t, response.Logs)
}

func TestHandleGetBackendLogsTrace_MissingRequestID(t *testing.T) {
	router := newExecutionHandlerRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trace/", http.NoBody)

	// Set up chi route context with empty request ID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("requestID", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetBackendLogsTrace(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleGetBackendLogsTrace_ServiceError(t *testing.T) {
	runner := &testRunner{
		fetchBackendLogsFunc: func(_ context.Context, _ string) ([]api.LogEvent, error) {
			return nil, errors.New("service unavailable")
		},
	}
	router := newExecutionHandlerRouter(t, nil, runner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trace/req-123", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("requestID", "req-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetBackendLogsTrace(w, req)

	// Service error is wrapped in ErrDatabaseError which returns 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// ==================== handleGetExecutionStatus tests ====================

func TestHandleGetExecutionStatus_Success(t *testing.T) {
	execRepo := &testExecutionRepository{
		getExecutionFunc: func(_ context.Context, executionID string) (*api.Execution, error) {
			assert.Equal(t, "exec-123", executionID)
			return &api.Execution{
				ExecutionID: executionID,
				Status:      string(constants.ExecutionRunning),
				CreatedBy:   "user@example.com",
			}, nil
		},
	}
	router := newExecutionHandlerRouter(t, execRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-123/status", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("executionID", "exec-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetExecutionStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response api.ExecutionStatusResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, string(constants.ExecutionRunning), response.Status)
}

func TestHandleGetExecutionStatus_NotFound(t *testing.T) {
	execRepo := &testExecutionRepository{
		getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
			return nil, apperrors.ErrNotFound("execution not found", nil)
		},
	}
	router := newExecutionHandlerRouter(t, execRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/nonexistent/status", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("executionID", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetExecutionStatus(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleGetExecutionStatus_MissingExecutionID(t *testing.T) {
	router := newExecutionHandlerRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions//status", http.NoBody)

	// Set up chi route context with empty execution ID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("executionID", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleGetExecutionStatus(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ==================== handleKillExecution tests ====================

func TestHandleKillExecution_Success(t *testing.T) {
	execRepo := &testExecutionRepository{
		getExecutionFunc: func(_ context.Context, executionID string) (*api.Execution, error) {
			return &api.Execution{
				ExecutionID: executionID,
				Status:      string(constants.ExecutionRunning),
				CreatedBy:   "user@example.com",
			}, nil
		},
	}
	runner := &testRunner{}
	router := newExecutionHandlerRouter(t, execRepo, runner)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/executions/exec-123", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("executionID", "exec-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleKillExecution(w, req)

	// Response can be either 200 or 204 depending on implementation
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNoContent)
}

func TestHandleKillExecution_NotFound(t *testing.T) {
	execRepo := &testExecutionRepository{
		getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
			return nil, apperrors.ErrNotFound("execution not found", nil)
		},
	}
	router := newExecutionHandlerRouter(t, execRepo, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/executions/nonexistent", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("executionID", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleKillExecution(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleKillExecution_MissingExecutionID(t *testing.T) {
	router := newExecutionHandlerRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/executions/", http.NoBody)

	// Set up chi route context with empty execution ID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("executionID", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleKillExecution(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleKillExecution_NoContent(t *testing.T) {
	execRepo := &testExecutionRepository{
		getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
			return nil, apperrors.ErrNotFound("execution not found", nil)
		},
	}
	router := newExecutionHandlerRouter(t, execRepo, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/executions/exec-finished", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("executionID", "exec-finished")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleKillExecution(w, req)

	// Expect 404 since execution not found
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ==================== handleListExecutions tests ====================

func TestHandleListExecutions_Success(t *testing.T) {
	now := time.Now()
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(limit int, _ []string) ([]*api.Execution, error) {
			// Called both during enforcer initialization (limit=0) and actual list (limit=10)
			if limit == 0 {
				// Return empty list for initialization
				return []*api.Execution{}, nil
			}
			// Return data for actual list operation
			return []*api.Execution{
				{
					ExecutionID: "exec-1",
					Status:      string(constants.ExecutionRunning),
					CreatedBy:   "user@example.com",
					StartedAt:   now,
				},
				{
					ExecutionID: "exec-2",
					Status:      string(constants.ExecutionSucceeded),
					CreatedBy:   "user@example.com",
					StartedAt:   now.Add(-1 * time.Hour),
				},
			}, nil
		},
	}
	router := newExecutionHandlerRouter(t, execRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions", http.NoBody)

	w := httptest.NewRecorder()
	router.handleListExecutions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []*api.Execution
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Equal(t, "exec-1", response[0].ExecutionID)
	assert.Equal(t, "exec-2", response[1].ExecutionID)
}

func TestHandleListExecutions_WithLimit(t *testing.T) {
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(limit int, _ []string) ([]*api.Execution, error) {
			// Called both during enforcer initialization (limit=0) and actual list (limit=20)
			if limit == 0 {
				return []*api.Execution{}, nil
			}
			assert.Equal(t, 20, limit)
			return []*api.Execution{}, nil
		},
	}
	router := newExecutionHandlerRouter(t, execRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions?limit=20", http.NoBody)

	w := httptest.NewRecorder()
	router.handleListExecutions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleListExecutions_WithStatusFilter(t *testing.T) {
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(limit int, statuses []string) ([]*api.Execution, error) {
			// Called both during enforcer initialization (limit=0, statuses=nil) and actual list
			if limit == 0 {
				return []*api.Execution{}, nil
			}
			assert.Equal(t, 2, len(statuses))
			assert.Contains(t, statuses, "RUNNING")
			assert.Contains(t, statuses, "TERMINATING")
			return []*api.Execution{}, nil
		},
	}
	router := newExecutionHandlerRouter(t, execRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions?status=RUNNING,TERMINATING", http.NoBody)

	w := httptest.NewRecorder()
	router.handleListExecutions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleListExecutions_WithStatusFilterAndLimit(t *testing.T) {
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(limit int, statuses []string) ([]*api.Execution, error) {
			// Called both during enforcer initialization (limit=0, statuses=nil) and actual list
			if limit == 0 {
				return []*api.Execution{}, nil
			}
			assert.Equal(t, 50, limit)
			assert.Equal(t, 1, len(statuses))
			assert.Contains(t, statuses, "SUCCEEDED")
			return []*api.Execution{}, nil
		},
	}
	router := newExecutionHandlerRouter(t, execRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions?limit=50&status=SUCCEEDED", http.NoBody)

	w := httptest.NewRecorder()
	router.handleListExecutions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleListExecutions_InvalidLimit(t *testing.T) {
	router := newExecutionHandlerRouter(t, nil, nil)

	tests := []struct {
		name  string
		limit string
	}{
		{
			name:  "non-numeric limit",
			limit: "abc",
		},
		{
			name:  "negative limit",
			limit: "-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/executions?limit="+tt.limit, http.NoBody)

			w := httptest.NewRecorder()
			router.handleListExecutions(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestHandleListExecutions_ZeroLimit(t *testing.T) {
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(limit int, _ []string) ([]*api.Execution, error) {
			assert.Equal(t, 0, limit) // 0 means return all
			return []*api.Execution{}, nil
		},
	}
	router := newExecutionHandlerRouter(t, execRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions?limit=0", http.NoBody)

	w := httptest.NewRecorder()
	router.handleListExecutions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleListExecutions_EmptyResult(t *testing.T) {
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(_ int, _ []string) ([]*api.Execution, error) {
			return []*api.Execution{}, nil
		},
	}
	router := newExecutionHandlerRouter(t, execRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions", http.NoBody)

	w := httptest.NewRecorder()
	router.handleListExecutions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []*api.Execution
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response, 0)
}

func TestHandleListExecutions_ServiceError(t *testing.T) {
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(limit int, _ []string) ([]*api.Execution, error) {
			// For initialization call, return empty list
			if limit == 0 {
				return []*api.Execution{}, nil
			}
			// For actual call, return error
			return nil, apperrors.ErrDatabaseError("database connection failed", nil)
		},
	}
	router := newExecutionHandlerRouter(t, execRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions", http.NoBody)

	w := httptest.NewRecorder()
	router.handleListExecutions(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleListExecutions_MultipleStatusesWithSpaces(t *testing.T) {
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(limit int, statuses []string) ([]*api.Execution, error) {
			// Called both during enforcer initialization (limit=0, statuses=nil) and actual list
			if limit == 0 {
				return []*api.Execution{}, nil
			}
			assert.Equal(t, 3, len(statuses))
			// Verify spaces are trimmed
			assert.Contains(t, statuses, "RUNNING")
			assert.Contains(t, statuses, "PENDING")
			assert.Contains(t, statuses, "SUCCEEDED")
			return []*api.Execution{}, nil
		},
	}
	router := newExecutionHandlerRouter(t, execRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions?status=RUNNING,%20PENDING%20,%20SUCCEEDED", http.NoBody)

	w := httptest.NewRecorder()
	router.handleListExecutions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== Benchmark tests ====================

func BenchmarkHandleRunCommand(b *testing.B) {
	runner := &testRunner{}
	router := newExecutionHandlerRouter(&testing.T{}, nil, runner)

	reqBody := api.ExecutionRequest{
		Command: "echo hello",
		Image:   "alpine:latest",
	}
	body, _ := json.Marshal(reqBody)

	user := testutil.NewUserBuilder().
		WithEmail("user@example.com").
		WithRole("developer").
		Build()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = addAuthenticatedUser(req, user)

		w := httptest.NewRecorder()
		router.handleRunCommand(w, req)
	}
}

func BenchmarkHandleListExecutions(b *testing.B) {
	execRepo := &testExecutionRepository{
		listExecutionsFunc: func(_ int, _ []string) ([]*api.Execution, error) {
			return []*api.Execution{}, nil
		},
	}
	router := newExecutionHandlerRouter(&testing.T{}, execRepo, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/executions", http.NoBody)
		w := httptest.NewRecorder()
		router.handleListExecutions(w, req)
	}
}
