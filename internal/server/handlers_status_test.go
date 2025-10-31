package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"runvoy/internal/app"
	"runvoy/internal/constants"
	rlogger "runvoy/internal/logger"
)

// Test that the status endpoint exists and requires authentication
func TestGetExecutionStatus_Unauthorized(t *testing.T) {
	// Initialize a basic logger
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)

	// Build a minimal service with nil repos; we won't reach the handler due to auth
	svc := app.NewService(nil, nil, nil, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-123/status", nil)
	// No X-API-Key header set
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", resp.Code)
	}
}
