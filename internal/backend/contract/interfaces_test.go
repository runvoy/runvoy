package contract

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/runvoy/runvoy/internal/api"

	"github.com/stretchr/testify/assert"
)

// TestTaskManager_Interface verifies that the TaskManager interface is properly defined.
func TestTaskManager_Interface(t *testing.T) {
	var _ TaskManager = (*testTaskManager)(nil)

	manager := &testTaskManager{}
	req := &api.ExecutionRequest{Command: "echo test"}

	executionID, createdAt, err := manager.StartTask(context.Background(), "user@example.com", req)
	assert.NoError(t, err)
	assert.Equal(t, "test-exec", executionID)
	assert.NotNil(t, createdAt)

	err = manager.KillTask(context.Background(), "exec-123")
	assert.NoError(t, err)
}

// TestImageRegistry_Interface verifies that the ImageRegistry interface is properly defined.
func TestImageRegistry_Interface(t *testing.T) {
	var _ ImageRegistry = (*testImageRegistry)(nil)

	registry := &testImageRegistry{}

	isDefault := true
	cpu := 512
	memory := 1024
	platform := "Linux/ARM64"

	err := registry.RegisterImage(
		context.Background(),
		"alpine:latest",
		&isDefault,
		nil, nil,
		&cpu, &memory,
		&platform,
		"user@example.com",
	)
	assert.NoError(t, err)

	images, err := registry.ListImages(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, images)

	image, err := registry.GetImage(context.Background(), "alpine:latest")
	assert.NoError(t, err)
	assert.NotNil(t, image)

	err = registry.RemoveImage(context.Background(), "alpine:latest")
	assert.NoError(t, err)
}

// TestLogManager_Interface verifies that the LogManager interface is properly defined.
func TestLogManager_Interface(t *testing.T) {
	var _ LogManager = (*testLogManager)(nil)

	manager := &testLogManager{}
	logs, err := manager.FetchLogsByExecutionID(context.Background(), "exec-123")
	assert.NoError(t, err)
	assert.NotNil(t, logs)
}

// TestObservabilityManager_Interface verifies that the ObservabilityManager interface is properly defined.
func TestObservabilityManager_Interface(t *testing.T) {
	var _ ObservabilityManager = (*testObservabilityManager)(nil)

	manager := &testObservabilityManager{}
	logs, err := manager.FetchBackendLogs(context.Background(), "req-123")
	assert.NoError(t, err)
	assert.NotNil(t, logs)
}

// TestWebSocketManager_Interface verifies that the WebSocketManager interface is properly defined.
func TestWebSocketManager_Interface(t *testing.T) {
	var _ WebSocketManager = (*testWebSocketManager)(nil)

	manager := &testWebSocketManager{}
	rawEvent := json.RawMessage(`{}`)
	logger := slog.Default()

	handled, err := manager.HandleRequest(context.Background(), &rawEvent, logger)
	assert.NoError(t, err)
	assert.False(t, handled)

	err = manager.NotifyExecutionCompletion(context.Background(), nil)
	assert.NoError(t, err)

	err = manager.SendLogsToExecution(context.Background(), nil)
	assert.NoError(t, err)

	url := manager.GenerateWebSocketURL(context.Background(), "exec-123", nil, nil)
	assert.Equal(t, "", url)
}

// TestHealthManager_Interface verifies that the HealthManager interface is properly defined.
func TestHealthManager_Interface(t *testing.T) {
	var _ HealthManager = (*testHealthManager)(nil)

	manager := &testHealthManager{}
	report, err := manager.Reconcile(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, report)
}

// Minimal implementations for testing interfaces
type testTaskManager struct{}

func (t *testTaskManager) StartTask(_ context.Context, _ string, _ *api.ExecutionRequest) (string, *time.Time, error) {
	now := time.Now()
	return "test-exec", &now, nil
}

func (t *testTaskManager) KillTask(_ context.Context, _ string) error {
	return nil
}

type testImageRegistry struct{}

func (t *testImageRegistry) RegisterImage(
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

func (t *testImageRegistry) ListImages(_ context.Context) ([]api.ImageInfo, error) {
	return []api.ImageInfo{}, nil
}

func (t *testImageRegistry) GetImage(_ context.Context, _ string) (*api.ImageInfo, error) {
	return &api.ImageInfo{}, nil
}

func (t *testImageRegistry) RemoveImage(_ context.Context, _ string) error {
	return nil
}

type testLogManager struct{}

func (t *testLogManager) FetchLogsByExecutionID(_ context.Context, _ string) ([]api.LogEvent, error) {
	return []api.LogEvent{}, nil
}

type testObservabilityManager struct{}

func (t *testObservabilityManager) FetchBackendLogs(_ context.Context, _ string) ([]api.LogEvent, error) {
	return []api.LogEvent{}, nil
}

type testWebSocketManager struct{}

func (t *testWebSocketManager) HandleRequest(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
	return false, nil
}

func (t *testWebSocketManager) NotifyExecutionCompletion(_ context.Context, _ *string) error {
	return nil
}

func (t *testWebSocketManager) SendLogsToExecution(_ context.Context, _ *string) error {
	return nil
}

func (t *testWebSocketManager) GenerateWebSocketURL(
	_ context.Context,
	_ string,
	_ *string,
	_ *string,
) string {
	return ""
}

type testHealthManager struct{}

func (t *testHealthManager) Reconcile(_ context.Context) (*api.HealthReport, error) {
	return &api.HealthReport{}, nil
}
