package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	"runvoy/internal/testutil"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
)

// Mock execution repository for testing
type mockExecutionRepo struct {
	getExecutionFunc    func(ctx context.Context, executionID string) (*api.Execution, error)
	updateExecutionFunc func(ctx context.Context, execution *api.Execution) error
}

func (m *mockExecutionRepo) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	if m.getExecutionFunc != nil {
		return m.getExecutionFunc(ctx, executionID)
	}
	return nil, nil
}

func (m *mockExecutionRepo) UpdateExecution(ctx context.Context, execution *api.Execution) error {
	if m.updateExecutionFunc != nil {
		return m.updateExecutionFunc(ctx, execution)
	}
	return nil
}

func (m *mockExecutionRepo) CreateExecution(_ context.Context, _ *api.Execution) error {
	return nil
}

func (m *mockExecutionRepo) ListExecutions(_ context.Context) ([]*api.Execution, error) {
	return nil, nil
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name      string
		timeStr   string
		wantError bool
	}{
		{
			name:      "valid RFC3339 timestamp",
			timeStr:   "2024-11-03T10:15:30Z",
			wantError: false,
		},
		{
			name:      "valid RFC3339 with timezone",
			timeStr:   "2024-11-03T10:15:30-05:00",
			wantError: false,
		},
		{
			name:      "invalid timestamp",
			timeStr:   "not-a-timestamp",
			wantError: true,
		},
		{
			name:      "empty string",
			timeStr:   "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTime(tt.timeStr)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.False(t, result.IsZero())
			}
		})
	}
}

func TestExtractExecutionIDFromTaskArn(t *testing.T) {
	tests := []struct {
		name       string
		taskArn    string
		expectedID string
	}{
		{
			name:       "standard ECS task ARN",
			taskArn:    "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/abc123def456",
			expectedID: "abc123def456",
		},
		{
			name:       "task ARN with UUID",
			taskArn:    "arn:aws:ecs:eu-west-1:999888777666:task/prod-cluster/550e8400-e29b-41d4-a716-446655440000",
			expectedID: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:       "simple ID",
			taskArn:    "exec-123",
			expectedID: "exec-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractExecutionIDFromTaskArn(tt.taskArn)
			assert.Equal(t, tt.expectedID, result)
		})
	}
}

func TestDetermineStatusAndExitCode(t *testing.T) {
	tests := []struct {
		name           string
		event          ECSTaskStateChangeEvent
		expectedStatus string
		expectedExit   int
	}{
		{
			name: "successful execution with exit code 0",
			event: ECSTaskStateChangeEvent{
				StopCode: "EssentialContainerExited",
				Containers: []ContainerDetail{
					{
						Name:     constants.RunnerContainerName,
						ExitCode: intPtr(0),
					},
				},
			},
			expectedStatus: string(constants.ExecutionSucceeded),
			expectedExit:   0,
		},
		{
			name: "failed execution with exit code 1",
			event: ECSTaskStateChangeEvent{
				StopCode: "EssentialContainerExited",
				Containers: []ContainerDetail{
					{
						Name:     constants.RunnerContainerName,
						ExitCode: intPtr(1),
					},
				},
			},
			expectedStatus: string(constants.ExecutionFailed),
			expectedExit:   1,
		},
		{
			name: "user initiated stop",
			event: ECSTaskStateChangeEvent{
				StopCode: "UserInitiated",
				Containers: []ContainerDetail{
					{
						Name:     constants.RunnerContainerName,
						ExitCode: intPtr(0),
					},
				},
			},
			expectedStatus: string(constants.ExecutionStopped),
			expectedExit:   130,
		},
		{
			name: "task failed to start",
			event: ECSTaskStateChangeEvent{
				StopCode:   "TaskFailedToStart",
				Containers: []ContainerDetail{},
			},
			expectedStatus: string(constants.ExecutionFailed),
			expectedExit:   1,
		},
		{
			name: "container exited with custom exit code",
			event: ECSTaskStateChangeEvent{
				StopCode: "EssentialContainerExited",
				Containers: []ContainerDetail{
					{
						Name:     constants.RunnerContainerName,
						ExitCode: intPtr(137),
					},
				},
			},
			expectedStatus: string(constants.ExecutionFailed),
			expectedExit:   137,
		},
		{
			name: "no exit code available",
			event: ECSTaskStateChangeEvent{
				StopCode: "EssentialContainerExited",
				Containers: []ContainerDetail{
					{
						Name:     constants.RunnerContainerName,
						ExitCode: nil,
					},
				},
			},
			expectedStatus: string(constants.ExecutionFailed),
			expectedExit:   1,
		},
		{
			name: "runner container not found",
			event: ECSTaskStateChangeEvent{
				StopCode: "EssentialContainerExited",
				Containers: []ContainerDetail{
					{
						Name:     "other-container",
						ExitCode: intPtr(0),
					},
				},
			},
			expectedStatus: string(constants.ExecutionFailed),
			expectedExit:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, exitCode := determineStatusAndExitCode(&tt.event)
			assert.Equal(t, tt.expectedStatus, status)
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}

func TestHandleECSTaskCompletion_Success(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now().Add(-5 * time.Minute)
	stopTime := time.Now()

	exitCode := 0
	execution := &api.Execution{
		ExecutionID: "test-exec-123",
		UserEmail:   "user@example.com",
		Command:     "echo hello",
		Status:      string(constants.ExecutionRunning),
		StartedAt:   startTime,
	}

	var updatedExecution *api.Execution
	mockRepo := &mockExecutionRepo{
		getExecutionFunc: func(_ context.Context, executionID string) (*api.Execution, error) {
			assert.Equal(t, "test-exec-123", executionID)
			return execution, nil
		},
		updateExecutionFunc: func(_ context.Context, exec *api.Execution) error {
			updatedExecution = exec
			return nil
		},
	}

	processor := &Processor{
		executionRepo: mockRepo,
		logger:        testutil.SilentLogger(),
	}

	taskEvent := ECSTaskStateChangeEvent{
		TaskArn:    "arn:aws:ecs:us-east-1:123456789012:task/cluster/test-exec-123",
		LastStatus: "STOPPED",
		Containers: []ContainerDetail{
			{
				Name:     constants.RunnerContainerName,
				ExitCode: &exitCode,
			},
		},
		StartedAt: startTime.Format(time.RFC3339),
		StoppedAt: stopTime.Format(time.RFC3339),
		StopCode:  "EssentialContainerExited",
	}

	detailJSON, _ := json.Marshal(taskEvent)
	event := events.CloudWatchEvent{
		DetailType: "ECS Task State Change",
		Source:     "aws.ecs",
		Detail:     detailJSON,
	}

	err := processor.handleECSTaskCompletion(ctx, &event)
	assert.NoError(t, err)
	assert.NotNil(t, updatedExecution)
	assert.Equal(t, string(constants.ExecutionSucceeded), updatedExecution.Status)
	assert.Equal(t, 0, updatedExecution.ExitCode)
	assert.NotNil(t, updatedExecution.CompletedAt)
	assert.Greater(t, updatedExecution.DurationSeconds, 0)
}

func TestHandleECSTaskCompletion_OrphanedTask(t *testing.T) {
	ctx := context.Background()

	mockRepo := &mockExecutionRepo{
		getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
			// Return nil to simulate orphaned task
			return nil, nil
		},
	}

	processor := &Processor{
		executionRepo: mockRepo,
		logger:        testutil.SilentLogger(),
	}

	exitCode := 0
	taskEvent := ECSTaskStateChangeEvent{
		TaskArn:    "arn:aws:ecs:us-east-1:123456789012:task/cluster/orphaned-123",
		LastStatus: "STOPPED",
		Containers: []ContainerDetail{
			{
				Name:     constants.RunnerContainerName,
				ExitCode: &exitCode,
			},
		},
		StartedAt: time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		StoppedAt: time.Now().Format(time.RFC3339),
		StopCode:  "EssentialContainerExited",
	}

	detailJSON, _ := json.Marshal(taskEvent)
	event := events.CloudWatchEvent{
		DetailType: "ECS Task State Change",
		Source:     "aws.ecs",
		Detail:     detailJSON,
	}

	// Should not fail for orphaned tasks
	err := processor.handleECSTaskCompletion(ctx, &event)
	assert.NoError(t, err)
}

func TestHandleECSTaskCompletion_MissingStartedAt(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now().Add(-5 * time.Minute)
	stopTime := time.Now()

	exitCode := 0
	execution := &api.Execution{
		ExecutionID: "test-exec-123",
		UserEmail:   "user@example.com",
		Command:     "echo hello",
		Status:      string(constants.ExecutionRunning),
		StartedAt:   startTime,
	}

	var updatedExecution *api.Execution
	mockRepo := &mockExecutionRepo{
		getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
			return execution, nil
		},
		updateExecutionFunc: func(_ context.Context, exec *api.Execution) error {
			updatedExecution = exec
			return nil
		},
	}

	processor := &Processor{
		executionRepo: mockRepo,
		logger:        testutil.SilentLogger(),
	}

	taskEvent := ECSTaskStateChangeEvent{
		TaskArn:    "arn:aws:ecs:us-east-1:123456789012:task/cluster/test-exec-123",
		LastStatus: "STOPPED",
		Containers: []ContainerDetail{
			{
				Name:     constants.RunnerContainerName,
				ExitCode: &exitCode,
			},
		},
		StartedAt: "", // Empty startedAt - should use execution's StartedAt
		StoppedAt: stopTime.Format(time.RFC3339),
		StopCode:  "EssentialContainerExited",
	}

	detailJSON, _ := json.Marshal(taskEvent)
	event := events.CloudWatchEvent{
		DetailType: "ECS Task State Change",
		Source:     "aws.ecs",
		Detail:     detailJSON,
	}

	err := processor.handleECSTaskCompletion(ctx, &event)
	assert.NoError(t, err)
	assert.NotNil(t, updatedExecution)
	assert.Greater(t, updatedExecution.DurationSeconds, 0)
}

func TestHandleEvent_IgnoresUnknownEventType(t *testing.T) {
	ctx := context.Background()

	processor := &Processor{
		executionRepo: &mockExecutionRepo{},
		logger:        testutil.SilentLogger(),
	}

	event := events.CloudWatchEvent{
		DetailType: "Unknown Event Type",
		Source:     "custom.source",
	}

	err := processor.HandleEvent(ctx, &event)
	assert.NoError(t, err)
}

func TestHandleEventJSON(t *testing.T) {
	ctx := context.Background()

	processor := &Processor{
		executionRepo: &mockExecutionRepo{},
		logger:        testutil.SilentLogger(),
	}

	eventJSON := []byte(`{
		"detail-type": "Unknown Event Type",
		"source": "custom.source"
	}`)

	err := processor.HandleEventJSON(ctx, eventJSON)
	assert.NoError(t, err)
}

func TestHandleEventJSON_InvalidJSON(t *testing.T) {
	ctx := context.Background()

	processor := &Processor{
		executionRepo: &mockExecutionRepo{},
		logger:        testutil.SilentLogger(),
	}

	eventJSON := []byte(`invalid json`)

	err := processor.HandleEventJSON(ctx, eventJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal event")
}

func TestECSCompletionHandler(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now().Add(-5 * time.Minute)
	stopTime := time.Now()

	exitCode := 0
	execution := &api.Execution{
		ExecutionID: "test-exec-123",
		UserEmail:   "user@example.com",
		Command:     "echo hello",
		Status:      string(constants.ExecutionRunning),
		StartedAt:   startTime,
	}

	mockRepo := &mockExecutionRepo{
		getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
			return execution, nil
		},
		updateExecutionFunc: func(_ context.Context, _ *api.Execution) error {
			return nil
		},
	}

	handler := ECSCompletionHandler(mockRepo, testutil.SilentLogger())

	taskEvent := ECSTaskStateChangeEvent{
		TaskArn:    "arn:aws:ecs:us-east-1:123456789012:task/cluster/test-exec-123",
		LastStatus: "STOPPED",
		Containers: []ContainerDetail{
			{
				Name:     constants.RunnerContainerName,
				ExitCode: &exitCode,
			},
		},
		StartedAt: startTime.Format(time.RFC3339),
		StoppedAt: stopTime.Format(time.RFC3339),
		StopCode:  "EssentialContainerExited",
	}

	detailJSON, _ := json.Marshal(taskEvent)
	event := events.CloudWatchEvent{
		DetailType: "ECS Task State Change",
		Source:     "aws.ecs",
		Detail:     detailJSON,
	}

	err := handler(ctx, event)
	assert.NoError(t, err)
}

func TestParseTaskTimes_NegativeDuration(t *testing.T) {
	logger := testutil.SilentLogger()
	reqLogger := logger.With("test", "negative_duration")

	// Create times where stopTime is before startTime (should result in 0 duration)
	startTime := time.Now()
	stopTime := startTime.Add(-5 * time.Minute) // 5 minutes before start

	execution := &api.Execution{
		StartedAt: startTime,
	}

	taskEvent := &ECSTaskStateChangeEvent{
		StartedAt: startTime.Format(time.RFC3339),
		StoppedAt: stopTime.Format(time.RFC3339),
	}

	parsedStart, parsedStop, duration, err := parseTaskTimes(taskEvent, execution.StartedAt, reqLogger)

	assert.NoError(t, err)
	assert.Equal(t, 0, duration, "Negative duration should be set to 0")
	assert.False(t, parsedStart.IsZero())
	assert.False(t, parsedStop.IsZero())
}

func TestParseTaskTimes_InvalidStoppedAt(t *testing.T) {
	logger := testutil.SilentLogger()
	reqLogger := logger.With("test", "invalid_stopped")

	startTime := time.Now()

	taskEvent := &ECSTaskStateChangeEvent{
		StartedAt: startTime.Format(time.RFC3339),
		StoppedAt: "invalid-timestamp",
	}

	_, _, duration, err := parseTaskTimes(taskEvent, startTime, reqLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse stoppedAt")
	assert.Equal(t, 0, duration)
}

func TestParseTaskTimes_InvalidStartedAt(t *testing.T) {
	logger := testutil.SilentLogger()
	reqLogger := logger.With("test", "invalid_started")

	stopTime := time.Now()

	taskEvent := &ECSTaskStateChangeEvent{
		StartedAt: "invalid-timestamp",
		StoppedAt: stopTime.Format(time.RFC3339),
	}

	startedAt, stoppedAt, duration, err := parseTaskTimes(taskEvent, time.Now(), reqLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse startedAt")
	assert.True(t, startedAt.IsZero())
	assert.True(t, stoppedAt.IsZero())
	assert.Equal(t, 0, duration)
}

func TestParseTaskTimes_ValidParse(t *testing.T) {
	logger := testutil.SilentLogger()
	reqLogger := logger.With("test", "valid_parse")

	startTime := time.Now().Add(-5 * time.Minute)
	stopTime := time.Now()

	taskEvent := &ECSTaskStateChangeEvent{
		StartedAt: startTime.Format(time.RFC3339),
		StoppedAt: stopTime.Format(time.RFC3339),
	}

	parsedStart, parsedStop, duration, err := parseTaskTimes(taskEvent, time.Now(), reqLogger)

	assert.NoError(t, err)
	assert.WithinDuration(t, startTime, parsedStart, time.Second)
	assert.WithinDuration(t, stopTime, parsedStop, time.Second)
	assert.GreaterOrEqual(t, duration, 299) // At least 299 seconds (allowing for minor time drift)
	assert.LessOrEqual(t, duration, 301)    // At most 301 seconds
}

// Helper function to create int pointers
func intPtr(i int) *int {
	return &i
}
