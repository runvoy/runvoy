package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/backend/orchestrator"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/require"
)

// mockRunner implements TaskManager, ImageRegistry, LogManager, and ObservabilityManager interfaces for testing
type mockRunner struct{}

func (m *mockRunner) StartTask(_ context.Context, _ string, _ *api.ExecutionRequest) (string, *time.Time, error) {
	return "test-execution-id", nil, nil
}

func (m *mockRunner) KillTask(_ context.Context, _ string) error {
	return nil
}

func (m *mockRunner) RegisterImage(
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

func (m *mockRunner) ListImages(_ context.Context) ([]api.ImageInfo, error) {
	return []api.ImageInfo{
		{
			Image:     "alpine:latest",
			ImageID:   "alpine:latest",
			CreatedBy: "test@example.com",
			OwnedBy:   []string{"user@example.com"},
		},
	}, nil
}

func (m *mockRunner) GetImage(_ context.Context, _ string) (*api.ImageInfo, error) {
	return nil, nil
}

func (m *mockRunner) RemoveImage(_ context.Context, _ string) error {
	return nil
}

func (m *mockRunner) FetchLogsByExecutionID(_ context.Context, _ string) ([]api.LogEvent, error) {
	return []api.LogEvent{}, nil
}

func (m *mockRunner) FetchBackendLogs(_ context.Context, _ string) ([]api.LogEvent, error) {
	return []api.LogEvent{}, nil
}

func (m *mockRunner) GetImagesByRequestID(_ context.Context, _ string) ([]api.ImageInfo, error) {
	return []api.ImageInfo{}, nil
}

// Test that the status endpoint exists and requires authentication
func TestGetExecutionStatus_Unauthorized(t *testing.T) {
	// Build a minimal service with nil repos; we won't reach the handler due to auth
	repos := database.Repositories{
		User:       &testUserRepository{},
		Execution:  &testExecutionRepository{},
		Connection: nil,
		Token:      &testTokenRepository{},
		Image:      &testImageRepository{},
		Secrets:    &testSecretsRepository{},
	}
	svc, err := orchestrator.NewService(context.Background(),
		testRegion,
		&repos,
		&mockRunner{}, // TaskManager
		&mockRunner{}, // ImageRegistry
		&mockRunner{}, // LogManager
		&mockRunner{}, // ObservabilityManager
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		&noopHealthManager{},
		newPermissiveTestEnforcerForHandlers(t),
	)
	require.NoError(t, err)
	router := NewRouter(svc, 2*time.Second, constants.DefaultCORSAllowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/executions/exec-123/status", http.NoBody)
	// No X-API-Key header set
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", resp.Code)
	}
}
