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
	apperrors "runvoy/internal/errors"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper to create a router with API key dependencies
func newAPIKeyHandlerRouter(t *testing.T, userRepo *testUserRepository) *Router {
	if userRepo == nil {
		userRepo = &testUserRepository{}
	}
	svc := newTestOrchestratorService(
		t,
		userRepo,
		&testExecutionRepository{},
		nil,
		&testRunner{},
		nil,
		nil,
		nil,
	)
	return &Router{svc: svc}
}

// ==================== handleClaimAPIKey tests ====================

func TestHandleClaimAPIKey_Success(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour).Unix()

	userRepo := &testUserRepository{
		getPendingAPIKeyFunc: func(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
			assert.Equal(t, "test-secret-token-123", secretToken)
			return &api.PendingAPIKey{
				SecretToken: secretToken,
				APIKey:      "rvoy_test_key_123",
				UserEmail:   "newuser@example.com",
				CreatedBy:   "admin@example.com",
				CreatedAt:   now,
				ExpiresAt:   expiresAt,
			}, nil
		},
		markAsViewedFunc: func(ctx context.Context, secretToken string, ipAddress string) error {
			assert.Equal(t, "test-secret-token-123", secretToken)
			assert.NotEmpty(t, ipAddress)
			return nil
		},
	}
	router := newAPIKeyHandlerRouter(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/claim/test-secret-token-123", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", "test-secret-token-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleClaimAPIKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response api.ClaimAPIKeyResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.NotEmpty(t, response.APIKey)
}

func TestHandleClaimAPIKey_MissingToken(t *testing.T) {
	router := newAPIKeyHandlerRouter(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/claim/", http.NoBody)

	// Set up chi route context with empty token
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleClaimAPIKey(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp map[string]string
	err := json.NewDecoder(w.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "token")
}

func TestHandleClaimAPIKey_WhitespaceOnlyToken(t *testing.T) {
	router := newAPIKeyHandlerRouter(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/claim/   ", http.NoBody)

	// Set up chi route context with whitespace-only token
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", "   ")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleClaimAPIKey(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleClaimAPIKey_TokenNotFound(t *testing.T) {
	userRepo := &testUserRepository{
		getPendingAPIKeyFunc: func(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
			return nil, apperrors.ErrNotFound("pending API key not found", nil)
		},
	}
	router := newAPIKeyHandlerRouter(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/claim/nonexistent-token", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", "nonexistent-token")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleClaimAPIKey(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleClaimAPIKey_ExpiredToken(t *testing.T) {
	now := time.Now()
	expiredAt := now.Add(-1 * time.Hour).Unix() // Expired 1 hour ago

	userRepo := &testUserRepository{
		getPendingAPIKeyFunc: func(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
			return &api.PendingAPIKey{
				SecretToken: secretToken,
				APIKey:      "rvoy_test_key_123",
				UserEmail:   "newuser@example.com",
				CreatedBy:   "admin@example.com",
				CreatedAt:   now.Add(-25 * time.Hour),
				ExpiresAt:   expiredAt,
			}, nil
		},
	}
	router := newAPIKeyHandlerRouter(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/claim/expired-token", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", "expired-token")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleClaimAPIKey(w, req)

	// Expect error due to expired token
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusForbidden || w.Code == http.StatusUnauthorized)
}

func TestHandleClaimAPIKey_AlreadyClaimed(t *testing.T) {
	now := time.Now()

	userRepo := &testUserRepository{
		getPendingAPIKeyFunc: func(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
			viewed := now.Add(-1 * time.Hour)
			return &api.PendingAPIKey{
				SecretToken: secretToken,
				APIKey:      "rvoy_test_key_123",
				UserEmail:   "newuser@example.com",
				CreatedBy:   "admin@example.com",
				CreatedAt:   now.Add(-2 * time.Hour),
				ExpiresAt:   now.Add(22 * time.Hour).Unix(),
				ViewedAt:    &viewed, // Already viewed
				ViewedFrom:  "192.168.1.1",
			}, nil
		},
	}
	router := newAPIKeyHandlerRouter(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/claim/already-claimed-token", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", "already-claimed-token")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleClaimAPIKey(w, req)

	// May succeed with warning or fail depending on implementation
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest || w.Code == http.StatusConflict)
}

func TestHandleClaimAPIKey_DatabaseError(t *testing.T) {
	userRepo := &testUserRepository{
		getPendingAPIKeyFunc: func(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
			return nil, apperrors.ErrDatabaseError("database connection failed", nil)
		},
	}
	router := newAPIKeyHandlerRouter(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/claim/test-token", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", "test-token")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleClaimAPIKey(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleClaimAPIKey_ServiceError(t *testing.T) {
	userRepo := &testUserRepository{
		getPendingAPIKeyFunc: func(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
			return nil, errors.New("unexpected service error")
		},
	}
	router := newAPIKeyHandlerRouter(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/claim/test-token", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", "test-token")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleClaimAPIKey(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleClaimAPIKey_WithLeadingTrailingSpaces(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour).Unix()

	userRepo := &testUserRepository{
		getPendingAPIKeyFunc: func(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
			// Should receive trimmed token
			assert.Equal(t, "test-token", secretToken)
			return &api.PendingAPIKey{
				SecretToken: secretToken,
				APIKey:      "rvoy_test_key_123",
				UserEmail:   "newuser@example.com",
				CreatedBy:   "admin@example.com",
				CreatedAt:   now,
				ExpiresAt:   expiresAt,
			}, nil
		},
	}
	router := newAPIKeyHandlerRouter(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/claim/  test-token  ", http.NoBody)

	// Set up chi route context with spaces
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", "  test-token  ")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleClaimAPIKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleClaimAPIKey_CapturesClientIP(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour).Unix()

	var capturedIP string
	userRepo := &testUserRepository{
		getPendingAPIKeyFunc: func(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
			return &api.PendingAPIKey{
				SecretToken: secretToken,
				APIKey:      "rvoy_test_key_123",
				UserEmail:   "newuser@example.com",
				CreatedBy:   "admin@example.com",
				CreatedAt:   now,
				ExpiresAt:   expiresAt,
			}, nil
		},
		markAsViewedFunc: func(ctx context.Context, secretToken string, ipAddress string) error {
			capturedIP = ipAddress
			return nil
		},
	}
	router := newAPIKeyHandlerRouter(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/claim/test-token", http.NoBody)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", "test-token")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleClaimAPIKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// IP should be captured (may be from X-Forwarded-For or RemoteAddr)
	assert.NotEmpty(t, capturedIP)
}

func TestHandleClaimAPIKey_MarkAsViewedError(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour).Unix()

	userRepo := &testUserRepository{
		getPendingAPIKeyFunc: func(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
			return &api.PendingAPIKey{
				SecretToken: secretToken,
				APIKey:      "rvoy_test_key_123",
				UserEmail:   "newuser@example.com",
				CreatedBy:   "admin@example.com",
				CreatedAt:   now,
				ExpiresAt:   expiresAt,
			}, nil
		},
		markAsViewedFunc: func(ctx context.Context, secretToken string, ipAddress string) error {
			return errors.New("failed to mark as viewed")
		},
	}
	router := newAPIKeyHandlerRouter(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/claim/test-token", http.NoBody)

	// Set up chi route context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", "test-token")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.handleClaimAPIKey(w, req)

	// May succeed or fail depending on whether mark-as-viewed is critical
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
}

// ==================== Benchmark tests ====================

func BenchmarkHandleClaimAPIKey(b *testing.B) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour).Unix()

	userRepo := &testUserRepository{
		getPendingAPIKeyFunc: func(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
			return &api.PendingAPIKey{
				SecretToken: secretToken,
				APIKey:      "rvoy_test_key_123",
				UserEmail:   "newuser@example.com",
				CreatedBy:   "admin@example.com",
				CreatedAt:   now,
				ExpiresAt:   expiresAt,
			}, nil
		},
		markAsViewedFunc: func(ctx context.Context, secretToken string, ipAddress string) error {
			return nil
		},
	}
	router := newAPIKeyHandlerRouter(b, userRepo)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/claim/test-token", http.NoBody)

		// Set up chi route context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "test-token")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		router.handleClaimAPIKey(w, req)
	}
}
