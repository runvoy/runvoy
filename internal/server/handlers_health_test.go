package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServiceForHealth is a mock service for testing health handlers
type mockServiceForHealth struct {
	reconcileResourcesFunc func(ctx context.Context) (*api.HealthReport, error)
}

func (m *mockServiceForHealth) ReconcileResources(ctx context.Context) (*api.HealthReport, error) {
	if m.reconcileResourcesFunc != nil {
		return m.reconcileResourcesFunc(ctx)
	}
	return nil, nil
}

func TestHandleHealth(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:           "GET request returns 200 OK",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"status": "ok",
			},
		},
		{
			name:           "POST request returns 200 OK",
			method:         http.MethodPost,
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"status": "ok",
			},
		},
		{
			name:           "HEAD request returns 200 OK",
			method:         http.MethodHead,
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"status": "ok",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal router with mock service
			router := &Router{
				svc: &mockServiceForHealth{},
			}

			// Create test request
			req := httptest.NewRequest(tt.method, "/health", nil)
			w := httptest.NewRecorder()

			// Call handler
			router.handleHealth(w, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			// For non-HEAD requests, check response body
			if tt.method != http.MethodHead && w.Code == http.StatusOK {
				var response api.HealthResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)

				assert.Equal(t, "ok", response.Status)
				assert.NotNil(t, response.Version)
			}
		})
	}
}

func TestHandleHealth_VersionInResponse(t *testing.T) {
	router := &Router{
		svc: &mockServiceForHealth{},
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.handleHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response api.HealthResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response.Status)
	// Version should be populated from constants
	assert.NotEmpty(t, response.Version)
}

func TestHandleReconcileHealth_Success(t *testing.T) {
	now := time.Now()
	expectedReport := &api.HealthReport{
		Timestamp: now,
		ComputeStatus: api.ComputeHealthStatus{
			TotalResources: 10,
			VerifiedCount:  9,
			RecreatedCount: 1,
		},
		SecretsStatus: api.SecretsHealthStatus{
			TotalSecrets:  5,
			VerifiedCount: 5,
		},
		IdentityStatus: api.IdentityHealthStatus{
			DefaultRolesVerified: true,
			CustomRolesVerified:  3,
			CustomRolesTotal:     3,
		},
		AuthorizerStatus: api.AuthorizerHealthStatus{
			TotalUsersChecked:     10,
			TotalResourcesChecked: 20,
		},
		ReconciledCount: 1,
		ErrorCount:      0,
	}

	router := &Router{
		svc: &mockServiceForHealth{
			reconcileResourcesFunc: func(ctx context.Context) (*api.HealthReport, error) {
				return expectedReport, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/reconcile", nil)
	w := httptest.NewRecorder()

	router.handleReconcileHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Status string            `json:"status"`
		Report *api.HealthReport `json:"report"`
	}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response.Status)
	assert.NotNil(t, response.Report)
	assert.Equal(t, expectedReport.Timestamp.Unix(), response.Report.Timestamp.Unix())
	assert.Equal(t, expectedReport.ComputeStatus.TotalResources, response.Report.ComputeStatus.TotalResources)
	assert.Equal(t, expectedReport.SecretsStatus.TotalSecrets, response.Report.SecretsStatus.TotalSecrets)
}

func TestHandleReconcileHealth_Error(t *testing.T) {
	tests := []struct {
		name           string
		reconcileError error
		expectedStatus int
	}{
		{
			name:           "generic error",
			reconcileError: errors.New("reconciliation failed"),
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "context canceled",
			reconcileError: context.Canceled,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "context deadline exceeded",
			reconcileError: context.DeadlineExceeded,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := &Router{
				svc: &mockServiceForHealth{
					reconcileResourcesFunc: func(ctx context.Context) (*api.HealthReport, error) {
						return nil, tt.reconcileError
					},
				},
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/health/reconcile", nil)
			w := httptest.NewRecorder()

			router.handleReconcileHealth(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.Equal(t, "error", response.Status)
			assert.NotEmpty(t, response.Message)
		})
	}
}

func TestHandleReconcileHealth_NilReport(t *testing.T) {
	router := &Router{
		svc: &mockServiceForHealth{
			reconcileResourcesFunc: func(ctx context.Context) (*api.HealthReport, error) {
				return nil, nil // No error, but nil report
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/reconcile", nil)
	w := httptest.NewRecorder()

	router.handleReconcileHealth(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Message, "health report is nil")
}

func TestHandleReconcileHealth_WithIssues(t *testing.T) {
	now := time.Now()
	reportWithIssues := &api.HealthReport{
		Timestamp: now,
		Issues: []api.HealthIssue{
			{
				ResourceType: "ecs_task_definition",
				ResourceID:   "task-123",
				Severity:     "error",
				Message:      "Task definition missing",
				Action:       "recreated",
			},
			{
				ResourceType: "iam_role",
				ResourceID:   "role-456",
				Severity:     "warning",
				Message:      "Role tags outdated",
				Action:       "tag_updated",
			},
		},
		ComputeStatus: api.ComputeHealthStatus{
			TotalResources: 10,
			VerifiedCount:  8,
			RecreatedCount: 1,
			OrphanedCount:  1,
		},
		ReconciledCount: 2,
		ErrorCount:      1,
	}

	router := &Router{
		svc: &mockServiceForHealth{
			reconcileResourcesFunc: func(ctx context.Context) (*api.HealthReport, error) {
				return reportWithIssues, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/reconcile", nil)
	w := httptest.NewRecorder()

	router.handleReconcileHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Status string            `json:"status"`
		Report *api.HealthReport `json:"report"`
	}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response.Status)
	assert.Len(t, response.Report.Issues, 2)
	assert.Equal(t, 2, response.Report.ReconciledCount)
	assert.Equal(t, 1, response.Report.ErrorCount)
}

func TestHandleReconcileHealth_ContextCancellation(t *testing.T) {
	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	router := &Router{
		svc: &mockServiceForHealth{
			reconcileResourcesFunc: func(ctx context.Context) (*api.HealthReport, error) {
				// Check if context is canceled
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
					return nil, nil
				}
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/reconcile", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	router.handleReconcileHealth(w, req)

	// Should return error due to canceled context
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleReconcileHealth_CompleteHealthReport(t *testing.T) {
	now := time.Now()
	completeReport := &api.HealthReport{
		Timestamp: now,
		ComputeStatus: api.ComputeHealthStatus{
			TotalResources:  20,
			VerifiedCount:   18,
			RecreatedCount:  1,
			TagUpdatedCount: 1,
			OrphanedCount:   0,
		},
		SecretsStatus: api.SecretsHealthStatus{
			TotalSecrets:    10,
			VerifiedCount:   9,
			TagUpdatedCount: 1,
			MissingCount:    0,
			OrphanedCount:   0,
		},
		IdentityStatus: api.IdentityHealthStatus{
			DefaultRolesVerified: true,
			CustomRolesVerified:  5,
			CustomRolesTotal:     5,
			MissingRoles:         []string{},
		},
		AuthorizerStatus: api.AuthorizerHealthStatus{
			UsersWithInvalidRoles:      []string{},
			UsersWithMissingRoles:      []string{},
			ResourcesWithMissingOwners: []string{},
			OrphanedOwnerships:         []string{},
			MissingOwnerships:          []string{},
			TotalUsersChecked:          25,
			TotalResourcesChecked:      50,
		},
		Issues:          []api.HealthIssue{},
		ReconciledCount: 2,
		ErrorCount:      0,
	}

	router := &Router{
		svc: &mockServiceForHealth{
			reconcileResourcesFunc: func(ctx context.Context) (*api.HealthReport, error) {
				return completeReport, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/reconcile", nil)
	w := httptest.NewRecorder()

	router.handleReconcileHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Status string            `json:"status"`
		Report *api.HealthReport `json:"report"`
	}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response.Status)
	assert.NotNil(t, response.Report)

	// Verify all status fields
	assert.Equal(t, 20, response.Report.ComputeStatus.TotalResources)
	assert.Equal(t, 10, response.Report.SecretsStatus.TotalSecrets)
	assert.True(t, response.Report.IdentityStatus.DefaultRolesVerified)
	assert.Equal(t, 25, response.Report.AuthorizerStatus.TotalUsersChecked)
}

func TestHandleHealth_ContentType(t *testing.T) {
	router := &Router{
		svc: &mockServiceForHealth{},
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.handleHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Content-Type should be application/json (set by json.Encoder)
	contentType := w.Header().Get("Content-Type")
	assert.Contains(t, contentType, "application/json")
}

func TestHandleReconcileHealth_ContentType(t *testing.T) {
	router := &Router{
		svc: &mockServiceForHealth{
			reconcileResourcesFunc: func(ctx context.Context) (*api.HealthReport, error) {
				return &api.HealthReport{
					Timestamp: time.Now(),
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/reconcile", nil)
	w := httptest.NewRecorder()

	router.handleReconcileHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	contentType := w.Header().Get("Content-Type")
	assert.Contains(t, contentType, "application/json")
}

// BenchmarkHandleHealth measures health endpoint performance
func BenchmarkHandleHealth(b *testing.B) {
	router := &Router{
		svc: &mockServiceForHealth{},
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.handleHealth(w, req)
	}
}

// BenchmarkHandleReconcileHealth measures reconcile endpoint performance
func BenchmarkHandleReconcileHealth(b *testing.B) {
	router := &Router{
		svc: &mockServiceForHealth{
			reconcileResourcesFunc: func(ctx context.Context) (*api.HealthReport, error) {
				return &api.HealthReport{
					Timestamp: time.Now(),
					ComputeStatus: api.ComputeHealthStatus{
						TotalResources: 10,
						VerifiedCount:  10,
					},
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/reconcile", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.handleReconcileHealth(w, req)
	}
}

func TestHandleHealth_VersionConstant(t *testing.T) {
	// Store original version
	origVersion := constants.GetVersion()
	defer func() {
		// Restore original (if needed)
		_ = origVersion
	}()

	router := &Router{
		svc: &mockServiceForHealth{},
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.handleHealth(w, req)

	var response api.HealthResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	// Version should match what constants.GetVersion() returns
	assert.Equal(t, *constants.GetVersion(), response.Version)
}
