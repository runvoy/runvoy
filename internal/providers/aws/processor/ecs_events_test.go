package aws

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	awsConstants "runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
)

// mockWebSocketManager is a mock for websocket notifications
type mockWebSocketManager struct {
	notifyFunc func(ctx context.Context, executionID *string) error
}

func (m *mockWebSocketManager) HandleRequest(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
	return false, nil
}

func (m *mockWebSocketManager) NotifyExecutionCompletion(ctx context.Context, executionID *string) error {
	if m.notifyFunc != nil {
		return m.notifyFunc(ctx, executionID)
	}
	return nil
}

func (m *mockWebSocketManager) SendLogsToExecution(_ context.Context, _ *string, _ []api.LogEvent) error {
	return nil
}

func (m *mockWebSocketManager) GenerateWebSocketURL(_ context.Context, _ string, _, _ *string) string {
	return ""
}

func TestHandleECSTaskEvent_Running(t *testing.T) {
	ctx := context.Background()
	executionID := "test-exec-123"
	taskArn := "arn:aws:ecs:us-east-1:123456789012:task/cluster/" + executionID

	execution := &api.Execution{
		ExecutionID: executionID,
		Status:      string(constants.ExecutionStarting),
		StartedAt:   time.Now(),
	}

	updated := false
	execRepo := &mockExecutionRepo{
		getExecutionFunc: func(ctx context.Context, id string) (*api.Execution, error) {
			assert.Equal(t, executionID, id)
			return execution, nil
		},
		updateExecutionFunc: func(ctx context.Context, exec *api.Execution) error {
			assert.Equal(t, string(constants.ExecutionRunning), exec.Status)
			updated = true
			return nil
		},
	}

	p := &Processor{
		executionRepo: execRepo,
	}

	event := &events.CloudWatchEvent{
		Detail: mustMarshal(ECSTaskStateChangeEvent{
			TaskArn:    taskArn,
			LastStatus: "RUNNING",
			StartedAt:  time.Now().Format(time.RFC3339),
		}),
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := p.handleECSTaskEvent(ctx, event, logger)

	assert.NoError(t, err)
	assert.True(t, updated, "execution should have been updated")
}

func TestHandleECSTaskEvent_Stopped(t *testing.T) {
	ctx := context.Background()
	executionID := "test-exec-456"
	taskArn := "arn:aws:ecs:us-east-1:123456789012:task/cluster/" + executionID

	startedAt := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)
	stoppedAt := time.Now().Format(time.RFC3339)

	execution := &api.Execution{
		ExecutionID: executionID,
		Status:      string(constants.ExecutionRunning),
		StartedAt:   mustParseTime(startedAt),
	}

	updated := false
	notified := false

	execRepo := &mockExecutionRepo{
		getExecutionFunc: func(ctx context.Context, id string) (*api.Execution, error) {
			return execution, nil
		},
		updateExecutionFunc: func(ctx context.Context, exec *api.Execution) error {
			assert.Equal(t, string(constants.ExecutionSucceeded), exec.Status)
			assert.NotNil(t, exec.CompletedAt)
			assert.Equal(t, 0, exec.ExitCode)
			updated = true
			return nil
		},
	}

	wsManager := &mockWebSocketManager{
		notifyFunc: func(ctx context.Context, execID *string) error {
			assert.Equal(t, executionID, *execID)
			notified = true
			return nil
		},
	}

	p := &Processor{
		executionRepo:     execRepo,
		webSocketManager: wsManager,
	}

	event := &events.CloudWatchEvent{
		Detail: mustMarshal(ECSTaskStateChangeEvent{
			TaskArn:    taskArn,
			LastStatus: "STOPPED",
			StartedAt:  startedAt,
			StoppedAt:  stoppedAt,
			StopCode:   "EssentialContainerExited",
			Containers: []ContainerDetail{
				{
					Name:     awsConstants.RunnerContainerName,
					ExitCode: intPtr(0),
				},
			},
		}),
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := p.handleECSTaskEvent(ctx, event, logger)

	assert.NoError(t, err)
	assert.True(t, updated, "execution should have been updated")
	assert.True(t, notified, "websocket notification should have been sent")
}

func TestHandleECSTaskEvent_OrphanedTask(t *testing.T) {
	ctx := context.Background()
	executionID := "orphaned-exec"
	taskArn := "arn:aws:ecs:us-east-1:123456789012:task/cluster/" + executionID

	execRepo := &mockExecutionRepo{
		getExecutionFunc: func(ctx context.Context, id string) (*api.Execution, error) {
			return nil, nil // Orphaned task
		},
	}

	p := &Processor{
		executionRepo: execRepo,
	}

	event := &events.CloudWatchEvent{
		Detail: mustMarshal(ECSTaskStateChangeEvent{
			TaskArn:    taskArn,
			LastStatus: "RUNNING",
		}),
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := p.handleECSTaskEvent(ctx, event, logger)

	// Should not return error for orphaned tasks
	assert.NoError(t, err)
}

func TestHandleECSTaskEvent_InvalidJSON(t *testing.T) {
	ctx := context.Background()

	p := &Processor{
		executionRepo: &mockExecutionRepo{},
	}

	event := &events.CloudWatchEvent{
		Detail: json.RawMessage(`invalid json`),
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := p.handleECSTaskEvent(ctx, event, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestUpdateExecutionToRunning_AlreadyRunning(t *testing.T) {
	ctx := context.Background()
	executionID := "exec-123"

	execution := &api.Execution{
		ExecutionID: executionID,
		Status:      string(constants.ExecutionRunning),
	}

	updateCalled := false
	execRepo := &mockExecutionRepo{
		updateExecutionFunc: func(ctx context.Context, exec *api.Execution) error {
			updateCalled = true
			return nil
		},
	}

	p := &Processor{
		executionRepo: execRepo,
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := p.updateExecutionToRunning(ctx, executionID, execution, logger)

	assert.NoError(t, err)
	assert.False(t, updateCalled, "should not update if already running")
}

func TestUpdateExecutionToRunning_InvalidTransition(t *testing.T) {
	ctx := context.Background()
	executionID := "exec-456"

	execution := &api.Execution{
		ExecutionID: executionID,
		Status:      string(constants.ExecutionSucceeded), // Terminal state
	}

	updateCalled := false
	execRepo := &mockExecutionRepo{
		updateExecutionFunc: func(ctx context.Context, exec *api.Execution) error {
			updateCalled = true
			return nil
		},
	}

	p := &Processor{
		executionRepo: execRepo,
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := p.updateExecutionToRunning(ctx, executionID, execution, logger)

	assert.NoError(t, err)
	assert.False(t, updateCalled, "should not update on invalid transition")
}

func TestFinalizeExecutionFromTaskEvent_InvalidTransition(t *testing.T) {
	ctx := context.Background()
	executionID := "exec-789"

	execution := &api.Execution{
		ExecutionID: executionID,
		Status:      string(constants.ExecutionSucceeded), // Already completed
		StartedAt:   time.Now().Add(-1 * time.Hour),
	}

	updateCalled := false
	execRepo := &mockExecutionRepo{
		updateExecutionFunc: func(ctx context.Context, exec *api.Execution) error {
			updateCalled = true
			return nil
		},
	}

	p := &Processor{
		executionRepo: execRepo,
	}

	taskEvent := &ECSTaskStateChangeEvent{
		TaskArn:    "arn:aws:ecs:us-east-1:123456789012:task/cluster/" + executionID,
		LastStatus: "STOPPED",
		StartedAt:  time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		StoppedAt:  time.Now().Format(time.RFC3339),
		StopCode:   "UserInitiated",
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := p.finalizeExecutionFromTaskEvent(ctx, executionID, execution, taskEvent, logger)

	assert.NoError(t, err)
	assert.False(t, updateCalled, "should not update on invalid transition")
}

func TestHandleECSTaskEvent_UserInitiatedStop(t *testing.T) {
	ctx := context.Background()
	executionID := "user-stopped-exec"
	taskArn := "arn:aws:ecs:us-east-1:123456789012:task/cluster/" + executionID

	execution := &api.Execution{
		ExecutionID: executionID,
		Status:      string(constants.ExecutionRunning),
		StartedAt:   time.Now().Add(-5 * time.Minute),
	}

	updated := false
	execRepo := &mockExecutionRepo{
		getExecutionFunc: func(ctx context.Context, id string) (*api.Execution, error) {
			return execution, nil
		},
		updateExecutionFunc: func(ctx context.Context, exec *api.Execution) error {
			assert.Equal(t, string(constants.ExecutionStopped), exec.Status)
			assert.Equal(t, 130, exec.ExitCode)
			updated = true
			return nil
		},
	}

	p := &Processor{
		executionRepo:     execRepo,
		webSocketManager: &mockWebSocketManager{},
	}

	event := &events.CloudWatchEvent{
		Detail: mustMarshal(ECSTaskStateChangeEvent{
			TaskArn:    taskArn,
			LastStatus: "STOPPED",
			StartedAt:  time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
			StoppedAt:  time.Now().Format(time.RFC3339),
			StopCode:   "UserInitiated",
		}),
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := p.handleECSTaskEvent(ctx, event, logger)

	assert.NoError(t, err)
	assert.True(t, updated)
}

func TestHandleECSTaskEvent_IgnoredStatus(t *testing.T) {
	ctx := context.Background()
	executionID := "exec-ignored"
	taskArn := "arn:aws:ecs:us-east-1:123456789012:task/cluster/" + executionID

	execution := &api.Execution{
		ExecutionID: executionID,
		Status:      string(constants.ExecutionStarting),
	}

	updateCalled := false
	execRepo := &mockExecutionRepo{
		getExecutionFunc: func(ctx context.Context, id string) (*api.Execution, error) {
			return execution, nil
		},
		updateExecutionFunc: func(ctx context.Context, exec *api.Execution) error {
			updateCalled = true
			return nil
		},
	}

	p := &Processor{
		executionRepo: execRepo,
	}

	// Test with a status that should be ignored (e.g., PROVISIONING)
	event := &events.CloudWatchEvent{
		Detail: mustMarshal(ECSTaskStateChangeEvent{
			TaskArn:    taskArn,
			LastStatus: "PROVISIONING",
		}),
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	err := p.handleECSTaskEvent(ctx, event, logger)

	assert.NoError(t, err)
	assert.False(t, updateCalled, "should not update for ignored statuses")
}

// Helper functions

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func mustParseTime(timeStr string) time.Time {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		panic(err)
	}
	return t
}

// BenchmarkDetermineStatusAndExitCode measures status determination performance
func BenchmarkDetermineStatusAndExitCode(b *testing.B) {
	event := &ECSTaskStateChangeEvent{
		StopCode: "EssentialContainerExited",
		Containers: []ContainerDetail{
			{
				Name:     awsConstants.RunnerContainerName,
				ExitCode: intPtr(0),
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = determineStatusAndExitCode(event)
	}
}

// BenchmarkExtractExecutionIDFromTaskArn measures ARN parsing performance
func BenchmarkExtractExecutionIDFromTaskArn(b *testing.B) {
	taskArn := "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/execution-id-123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractExecutionIDFromTaskArn(taskArn)
	}
}
