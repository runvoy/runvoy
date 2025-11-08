package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/app"
	"runvoy/internal/constants"
	"runvoy/internal/testutil"
)

// mockRunner implements the app.Runner interface for testing
type mockRunner struct{}

type noopConnectionRepository struct{}

func (n *noopConnectionRepository) CreateConnection(_ context.Context, _ *api.WebSocketConnection) error {
	return nil
}

func (n *noopConnectionRepository) DeleteConnections(_ context.Context, connIDs []string) (int, error) {
	return len(connIDs), nil
}

func (n *noopConnectionRepository) GetConnectionsByExecutionID(_ context.Context, _ string) ([]*api.WebSocketConnection, error) {
	return nil, nil
}

func (m *mockRunner) StartTask(_ context.Context, _ string, _ *api.ExecutionRequest) (string, *time.Time, error) {
	return "test-execution-id", nil, nil
}

func (m *mockRunner) KillTask(_ context.Context, _ string) error {
	return nil
}

func (m *mockRunner) RegisterImage(_ context.Context, _ string, _ *bool) error {
	return nil
}

func (m *mockRunner) ListImages(_ context.Context) ([]api.ImageInfo, error) {
	return []api.ImageInfo{
		{
			Image: "alpine:latest",
		},
	}, nil
}

func (m *mockRunner) RemoveImage(_ context.Context, _ string) error {
	return nil
}

func (m *mockRunner) FetchLogsByExecutionID(_ context.Context, _ string, _ *int64) ([]api.LogEvent, error) {
	return []api.LogEvent{}, nil
}

// Test that the status endpoint exists and requires authentication
func TestGetExecutionStatus_Unauthorized(t *testing.T) {
	// Build a minimal service with nil repos; we won't reach the handler due to auth
	svc := app.NewService(nil, nil, &noopConnectionRepository{}, &mockRunner{}, testutil.SilentLogger(), constants.AWS, "")
	router := NewRouter(svc, 2*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-123/status", http.NoBody)
	// No X-API-Key header set
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", resp.Code)
	}
}
