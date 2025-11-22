package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"runvoy/internal/backend/orchestrator"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouter(t *testing.T) {
	tokenRepo := &testTokenRepository{}

	repos := database.Repositories{
		User:       &testUserRepository{},
		Execution:  &testExecutionRepository{},
		Connection: nil,
		Token:      tokenRepo,
		Image:      &testImageRepository{},
		Secrets:    &testSecretsRepository{},
	}
	svc, err := orchestrator.NewService(
		context.Background(),
		&repos,
		&testRunner{}, // TaskManager
		&testRunner{}, // ImageRegistry
		&testRunner{}, // LogManager
		&testRunner{}, // ObservabilityManager
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil,
		newPermissiveTestEnforcerForHandlers(t),
	)
	require.NoError(t, err)

	t.Run("creates router without timeout", func(t *testing.T) {
		router := NewRouter(svc, 0, constants.DefaultCORSAllowedOrigins)
		assert.NotNil(t, router)
		assert.NotNil(t, router.router)
		assert.Equal(t, svc, router.svc)
	})

	t.Run("creates router with timeout", func(t *testing.T) {
		router := NewRouter(svc, 5*time.Second, constants.DefaultCORSAllowedOrigins)
		assert.NotNil(t, router)
		assert.NotNil(t, router.router)
		assert.Equal(t, svc, router.svc)
	})
}

func TestRouter_ChiMux(t *testing.T) {
	tokenRepo := &testTokenRepository{}

	repos := database.Repositories{
		User:       &testUserRepository{},
		Execution:  &testExecutionRepository{},
		Connection: nil,
		Token:      tokenRepo,
		Image:      &testImageRepository{},
		Secrets:    &testSecretsRepository{},
	}
	svc, err := orchestrator.NewService(
		context.Background(),
		&repos,
		&testRunner{}, // TaskManager
		&testRunner{}, // ImageRegistry
		&testRunner{}, // LogManager
		&testRunner{}, // ObservabilityManager
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil,
		newPermissiveTestEnforcerForHandlers(t),
	)
	require.NoError(t, err)
	router := NewRouter(svc, 0, constants.DefaultCORSAllowedOrigins)

	chiMux := router.ChiMux()
	assert.NotNil(t, chiMux)
	assert.Equal(t, router.router, chiMux)
}

func TestRouter_Handler(t *testing.T) {
	tokenRepo := &testTokenRepository{}

	repos := database.Repositories{
		User:       &testUserRepository{},
		Execution:  &testExecutionRepository{},
		Connection: nil,
		Token:      tokenRepo,
		Image:      &testImageRepository{},
		Secrets:    &testSecretsRepository{},
	}
	svc, err := orchestrator.NewService(
		context.Background(),
		&repos,
		&testRunner{}, // TaskManager
		&testRunner{}, // ImageRegistry
		&testRunner{}, // LogManager
		&testRunner{}, // ObservabilityManager
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil,
		newPermissiveTestEnforcerForHandlers(t),
	)
	require.NoError(t, err)
	router := NewRouter(svc, 0, constants.DefaultCORSAllowedOrigins)

	handler := router.Handler()
	assert.NotNil(t, handler)
	assert.Equal(t, router.router, handler)
}

func TestRouter_WithContext(t *testing.T) {
	tokenRepo := &testTokenRepository{}

	repos := database.Repositories{
		User:       &testUserRepository{},
		Execution:  &testExecutionRepository{},
		Connection: nil,
		Token:      tokenRepo,
		Image:      &testImageRepository{},
		Secrets:    &testSecretsRepository{},
	}
	svc, err := orchestrator.NewService(
		context.Background(),
		&repos,
		&testRunner{}, // TaskManager
		&testRunner{}, // ImageRegistry
		&testRunner{}, // LogManager
		&testRunner{}, // ObservabilityManager
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil,
		newPermissiveTestEnforcerForHandlers(t),
	)
	require.NoError(t, err)
	tokenRepo2 := &testTokenRepository{}
	runner2 := &testRunner{}
	repos2 := database.Repositories{
		User:       &testUserRepository{},
		Execution:  &testExecutionRepository{},
		Connection: nil,
		Token:      tokenRepo2,
		Image:      &testImageRepository{},
		Secrets:    &testSecretsRepository{},
	}
	svc2, err := orchestrator.NewService(
		context.Background(),
		&repos2,
		runner2, // TaskManager
		runner2, // ImageRegistry
		runner2, // LogManager
		runner2, // ObservabilityManager
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil,
		newPermissiveTestEnforcerForHandlers(t),
	)
	require.NoError(t, err)
	router := NewRouter(svc, 0, constants.DefaultCORSAllowedOrigins)

	ctx := context.Background()
	newCtx := router.WithContext(ctx, svc2)

	assert.NotEqual(t, ctx, newCtx)
	assert.Equal(t, svc2, newCtx.Value(serviceContextKey))
}

func TestRouter_ServeHTTP(t *testing.T) {
	tokenRepo := &testTokenRepository{}

	repos := database.Repositories{
		User:       &testUserRepository{},
		Execution:  &testExecutionRepository{},
		Connection: nil,
		Token:      tokenRepo,
		Image:      &testImageRepository{},
		Secrets:    &testSecretsRepository{},
	}
	svc, err := orchestrator.NewService(
		context.Background(),
		&repos,
		&testRunner{}, // TaskManager
		&testRunner{}, // ImageRegistry
		&testRunner{}, // LogManager
		&testRunner{}, // ObservabilityManager
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil,
		newPermissiveTestEnforcerForHandlers(t),
	)
	require.NoError(t, err)
	router := NewRouter(svc, 0, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest("GET", "/api/v1/health", http.NoBody)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestWriteErrorResponse(t *testing.T) {
	rr := httptest.NewRecorder()
	writeErrorResponse(rr, http.StatusBadRequest, "test error", "test details")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "test error")
	assert.Contains(t, rr.Body.String(), "test details")
}

func TestWriteErrorResponseWithCode(t *testing.T) {
	rr := httptest.NewRecorder()
	writeErrorResponseWithCode(rr, http.StatusBadRequest, "ERR_CODE", "test error", "test details")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "test error")
	assert.Contains(t, rr.Body.String(), "test details")
	assert.Contains(t, rr.Body.String(), "ERR_CODE")
}

func TestWriteErrorResponseWithCode_EmptyCode(t *testing.T) {
	rr := httptest.NewRecorder()
	writeErrorResponseWithCode(rr, http.StatusBadRequest, "", "test error", "test details")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "test error")
	assert.Contains(t, rr.Body.String(), "test details")
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rr,
		statusCode:     http.StatusOK,
	}

	rw.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, rw.statusCode)
	assert.True(t, rw.written)

	// Subsequent calls should not change status code
	rw.WriteHeader(http.StatusInternalServerError)
	assert.Equal(t, http.StatusNotFound, rw.statusCode)
}

func TestResponseWriter_Write(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rr,
		statusCode:     0,
	}

	n, err := rw.Write([]byte("test data"))
	require.NoError(t, err)
	assert.Equal(t, 9, n)
	assert.Equal(t, http.StatusOK, rw.statusCode)
	assert.True(t, rw.written)
}
