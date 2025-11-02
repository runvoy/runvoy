package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/app"
	"runvoy/internal/constants"
	rlogger "runvoy/internal/logger"
)

// mockRunner implements the app.Runner interface for testing
type mockRunner struct{}

func (m *mockRunner) StartTask(ctx context.Context, userEmail string, req api.ExecutionRequest) (string, *time.Time, error) {
	return "test-execution-id", nil, nil
}

func (m *mockRunner) KillTask(ctx context.Context, executionID string) error {
	return nil
}

func (m *mockRunner) RegisterImage(ctx context.Context, image string, isDefault *bool) error {
	return nil
}

func (m *mockRunner) ListImages(ctx context.Context) ([]api.ImageInfo, error) {
	return []api.ImageInfo{
		{
			Image: "alpine:latest",
		},
	}, nil
}

func (m *mockRunner) RemoveImage(ctx context.Context, image string) error {
	return nil
}

// Test that the status endpoint exists and requires authentication
func TestGetExecutionStatus_Unauthorized(t *testing.T) {
	// Initialize a basic logger
	_ = rlogger.Initialize(constants.Development, slog.LevelInfo)

	// Build a minimal service with nil repos; we won't reach the handler due to auth
	svc := app.NewService(nil, nil, &mockRunner{}, slog.Default(), constants.AWS)
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-123/status", nil)
	// No X-API-Key header set
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", resp.Code)
	}
}
