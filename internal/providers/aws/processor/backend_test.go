package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	awsConstants "runvoy/internal/providers/aws/constants"
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

func (m *mockExecutionRepo) ListExecutions(_ context.Context, _ int, _ []string) ([]*api.Execution, error) {
	return nil, nil
}

// Mock WebSocket handler for testing
type mockWebSocketHandler struct {
	handleRequestFunc             func(ctx context.Context, rawEvent *json.RawMessage, logger *slog.Logger) (bool, error)
	notifyExecutionCompletionFunc func(ctx context.Context, executionID *string) error
}

func (m *mockWebSocketHandler) HandleRequest(
	ctx context.Context, rawEvent *json.RawMessage, logger *slog.Logger) (bool, error) {
	if m.handleRequestFunc != nil {
		return m.handleRequestFunc(ctx, rawEvent, logger)
	}
	return false, nil
}

func (m *mockWebSocketHandler) NotifyExecutionCompletion(ctx context.Context, executionID *string) error {
	if m.notifyExecutionCompletionFunc != nil {
		return m.notifyExecutionCompletionFunc(ctx, executionID)
	}
	return nil
}

func (m *mockWebSocketHandler) SendLogsToExecution(_ context.Context, _ *string, _ []api.LogEvent) error {
	return nil
}

func (m *mockWebSocketHandler) GenerateWebSocketURL(_ context.Context, _ string, _, _ *string) string {
	return ""
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
						Name:     awsConstants.RunnerContainerName,
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
						Name:     awsConstants.RunnerContainerName,
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
						Name:     awsConstants.RunnerContainerName,
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
						Name:     awsConstants.RunnerContainerName,
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
						Name:     awsConstants.RunnerContainerName,
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

	mockWebSocket := &mockWebSocketHandler{
		notifyExecutionCompletionFunc: func(_ context.Context, executionID *string) error {
			assert.NotNil(t, executionID)
			assert.Equal(t, "test-exec-123", *executionID)
			return nil
		},
	}

	backend := NewProcessor(mockRepo, mockWebSocket, nil, testutil.SilentLogger())

	taskEvent := ECSTaskStateChangeEvent{
		TaskArn:    "arn:aws:ecs:us-east-1:123456789012:task/cluster/test-exec-123",
		LastStatus: "STOPPED",
		Containers: []ContainerDetail{
			{
				Name:     awsConstants.RunnerContainerName,
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

	err := backend.handleECSTaskEvent(ctx, &event, testutil.SilentLogger())
	assert.NoError(t, err)
	assert.NotNil(t, updatedExecution)
	assert.Equal(t, string(constants.ExecutionSucceeded), updatedExecution.Status)
	assert.Equal(t, 0, updatedExecution.ExitCode)
	assert.NotNil(t, updatedExecution.CompletedAt)
	assert.Greater(t, updatedExecution.DurationSeconds, 0)
}

func TestHandleECSTaskCompletion_MarkRunning(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now()

	execution := &api.Execution{
		ExecutionID: "run-exec-123",
		Status:      string(constants.ExecutionStarting),
		StartedAt:   startTime,
	}

	updateCalled := false
	mockRepo := &mockExecutionRepo{
		getExecutionFunc: func(_ context.Context, executionID string) (*api.Execution, error) {
			assert.Equal(t, "run-exec-123", executionID)
			return execution, nil
		},
		updateExecutionFunc: func(_ context.Context, exec *api.Execution) error {
			updateCalled = true
			assert.Equal(t, string(constants.ExecutionRunning), exec.Status)
			assert.Nil(t, exec.CompletedAt)
			return nil
		},
	}

	mockWebSocket := &mockWebSocketHandler{
		notifyExecutionCompletionFunc: func(_ context.Context, _ *string) error {
			assert.Fail(t, "should not notify completion for RUNNING status")
			return nil
		},
	}

	backend := NewProcessor(mockRepo, mockWebSocket, nil, testutil.SilentLogger())

	taskEvent := ECSTaskStateChangeEvent{
		TaskArn:    "arn:aws:ecs:us-east-1:123456789012:task/cluster/run-exec-123",
		LastStatus: string(awsConstants.EcsStatusRunning),
		StartedAt:  startTime.Format(time.RFC3339),
	}

	detailJSON, _ := json.Marshal(taskEvent)
	event := events.CloudWatchEvent{
		DetailType: "ECS Task State Change",
		Source:     "aws.ecs",
		Detail:     detailJSON,
	}

	err := backend.handleECSTaskEvent(ctx, &event, testutil.SilentLogger())
	assert.NoError(t, err)
	assert.True(t, updateCalled)
}

func TestHandleECSTaskCompletion_OrphanedTask(t *testing.T) {
	ctx := context.Background()

	mockRepo := &mockExecutionRepo{
		getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
			// Return nil to simulate orphaned task
			return nil, nil
		},
	}

	mockWebSocket := &mockWebSocketHandler{}

	backend := NewProcessor(mockRepo, mockWebSocket, nil, testutil.SilentLogger())

	exitCode := 0
	taskEvent := ECSTaskStateChangeEvent{
		TaskArn:    "arn:aws:ecs:us-east-1:123456789012:task/cluster/orphaned-123",
		LastStatus: "STOPPED",
		Containers: []ContainerDetail{
			{
				Name:     awsConstants.RunnerContainerName,
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
	err := backend.handleECSTaskEvent(ctx, &event, testutil.SilentLogger())
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

	mockWebSocket := &mockWebSocketHandler{
		notifyExecutionCompletionFunc: func(_ context.Context, executionID *string) error {
			assert.NotNil(t, executionID)
			return nil
		},
	}

	backend := NewProcessor(mockRepo, mockWebSocket, nil, testutil.SilentLogger())

	taskEvent := ECSTaskStateChangeEvent{
		TaskArn:    "arn:aws:ecs:us-east-1:123456789012:task/cluster/test-exec-123",
		LastStatus: "STOPPED",
		Containers: []ContainerDetail{
			{
				Name:     awsConstants.RunnerContainerName,
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

	err := backend.handleECSTaskEvent(ctx, &event, testutil.SilentLogger())
	assert.NoError(t, err)
	assert.NotNil(t, updatedExecution)
	assert.Greater(t, updatedExecution.DurationSeconds, 0)
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

func TestHandle_EventRouting(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	t.Run("routes CloudWatch event correctly", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{
			getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
				return nil, nil // Execution not found
			},
		}
		mockWebSocket := &mockWebSocketHandler{}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		// Test with actual CloudWatch event
		event := events.CloudWatchEvent{
			DetailType: "ECS Task State Change",
			Source:     "aws.ecs",
			Detail:     json.RawMessage(`{"taskArn":"arn:aws:ecs:us-east-1:123456789012:task/cluster/test-123"}`),
		}

		eventJSON, _ := json.Marshal(event)
		rawEvent := json.RawMessage(eventJSON)

		// Should route to cloud event handler
		// Execution not found returns nil (no error) for orphaned tasks
		_, err := processor.Handle(ctx, &rawEvent)
		// Should not error with "unhandled event type" - confirms routing worked
		if err != nil {
			assert.NotContains(t, err.Error(), "unhandled event type")
		} else {
			// No error is also valid - confirms it routed and handled gracefully
			assert.NoError(t, err)
		}
	})

	t.Run("routes CloudWatch Logs event correctly", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{}
		mockWebSocket := &mockWebSocketHandler{}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		// Create a CloudWatch Logs event (will fail parsing but should route correctly)
		logsEvent := events.CloudwatchLogsEvent{
			AWSLogs: events.CloudwatchLogsRawData{
				Data: "invalid-data", // Will fail parsing but routes to logs handler
			},
		}

		eventJSON, _ := json.Marshal(logsEvent)
		rawEvent := json.RawMessage(eventJSON)

		_, err := processor.Handle(ctx, &rawEvent)
		// Should error on parsing, confirming it routed to logs handler
		assert.Error(t, err)
	})

	t.Run("routes WebSocket event correctly", func(t *testing.T) {
		wsHandled := false
		mockRepo := &mockExecutionRepo{}
		mockWebSocket := &mockWebSocketHandler{}

		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		wsEvent := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				RouteKey: "$connect",
			},
		}

		eventJSON, _ := json.Marshal(wsEvent)
		rawEvent := json.RawMessage(eventJSON)

		result, err := processor.Handle(ctx, &rawEvent)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		wsHandled = true
		assert.True(t, wsHandled)
	})

	t.Run("returns error for unhandled event type", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{}
		mockWebSocket := &mockWebSocketHandler{}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		rawEvent := json.RawMessage(`{"unknown": "event", "type": "not_supported"}`)

		_, err := processor.Handle(ctx, &rawEvent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unhandled event type")
	})
}

func TestHandle_EventValidation(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	mockRepo := &mockExecutionRepo{}
	mockWebSocket := &mockWebSocketHandler{}
	processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

	t.Run("handles invalid JSON", func(t *testing.T) {
		rawEvent := json.RawMessage(`invalid json{`)
		_, err := processor.Handle(ctx, &rawEvent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unhandled event type")
	})

	t.Run("handles empty event", func(t *testing.T) {
		rawEvent := json.RawMessage(`{}`)
		_, err := processor.Handle(ctx, &rawEvent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unhandled event type")
	})

	t.Run("handles CloudWatch event with missing source", func(t *testing.T) {
		event := events.CloudWatchEvent{
			DetailType: "ECS Task State Change",
			// Missing Source
		}
		eventJSON, _ := json.Marshal(event)
		rawEvent := json.RawMessage(eventJSON)

		_, err := processor.Handle(ctx, &rawEvent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unhandled event type")
	})

	t.Run("handles CloudWatch event with missing detail type", func(t *testing.T) {
		event := events.CloudWatchEvent{
			Source: "aws.ecs",
			// Missing DetailType
		}
		eventJSON, _ := json.Marshal(event)
		rawEvent := json.RawMessage(eventJSON)

		_, err := processor.Handle(ctx, &rawEvent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unhandled event type")
	})

	t.Run("handles CloudWatch Logs event with empty data", func(t *testing.T) {
		logsEvent := events.CloudwatchLogsEvent{
			AWSLogs: events.CloudwatchLogsRawData{
				Data: "", // Empty data
			},
		}
		eventJSON, _ := json.Marshal(logsEvent)
		rawEvent := json.RawMessage(eventJSON)

		_, err := processor.Handle(ctx, &rawEvent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unhandled event type")
	})

	t.Run("handles WebSocket event with missing route key", func(t *testing.T) {
		wsEvent := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				// Missing RouteKey
			},
		}
		eventJSON, _ := json.Marshal(wsEvent)
		rawEvent := json.RawMessage(eventJSON)

		_, err := processor.Handle(ctx, &rawEvent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unhandled event type")
	})

	t.Run("handles unhandled CloudWatch event detail type", func(t *testing.T) {
		event := events.CloudWatchEvent{
			DetailType: "Unknown Event Type",
			Source:     "aws.ecs",
			Detail:     json.RawMessage(`{}`),
		}
		eventJSON, _ := json.Marshal(event)
		rawEvent := json.RawMessage(eventJSON)

		// Should handle but ignore unhandled detail types
		_, err := processor.Handle(ctx, &rawEvent)
		assert.NoError(t, err) // Unhandled detail types are ignored, not errors
	})
}

func TestHandle_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	t.Run("handles repository error when getting execution", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{
			getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
				return nil, fmt.Errorf("database connection failed")
			},
		}
		mockWebSocket := &mockWebSocketHandler{}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		taskEvent := ECSTaskStateChangeEvent{
			TaskArn:    "arn:aws:ecs:us-east-1:123456789012:task/cluster/test-123",
			LastStatus: "STOPPED",
			StoppedAt:  time.Now().Format(time.RFC3339),
			StopCode:   "EssentialContainerExited",
		}

		detailJSON, _ := json.Marshal(taskEvent)
		event := events.CloudWatchEvent{
			DetailType: "ECS Task State Change",
			Source:     "aws.ecs",
			Detail:     detailJSON,
		}

		eventJSON, _ := json.Marshal(event)
		rawEvent := json.RawMessage(eventJSON)

		_, err := processor.Handle(ctx, &rawEvent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get execution")
	})

	t.Run("handles repository error when updating execution", func(t *testing.T) {
		execution := &api.Execution{
			ExecutionID: "test-exec-123",
			Status:      string(constants.ExecutionRunning),
			StartedAt:   time.Now(),
		}

		mockRepo := &mockExecutionRepo{
			getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
				return execution, nil
			},
			updateExecutionFunc: func(_ context.Context, _ *api.Execution) error {
				return fmt.Errorf("update failed")
			},
		}
		mockWebSocket := &mockWebSocketHandler{}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		taskEvent := ECSTaskStateChangeEvent{
			TaskArn:    "arn:aws:ecs:us-east-1:123456789012:task/cluster/test-exec-123",
			LastStatus: "STOPPED",
			StoppedAt:  time.Now().Format(time.RFC3339),
			StopCode:   "EssentialContainerExited",
			Containers: []ContainerDetail{
				{
					Name:     awsConstants.RunnerContainerName,
					ExitCode: intPtr(0),
				},
			},
		}

		detailJSON, _ := json.Marshal(taskEvent)
		event := events.CloudWatchEvent{
			DetailType: "ECS Task State Change",
			Source:     "aws.ecs",
			Detail:     detailJSON,
		}

		eventJSON, _ := json.Marshal(event)
		rawEvent := json.RawMessage(eventJSON)

		_, err := processor.Handle(ctx, &rawEvent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update execution")
	})

	t.Run("handles WebSocket manager error", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{}
		mockWebSocket := &mockWebSocketHandler{
			handleRequestFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
				return false, fmt.Errorf("websocket connection failed")
			},
		}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		wsEvent := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				RouteKey: "$connect",
			},
		}

		eventJSON, _ := json.Marshal(wsEvent)
		rawEvent := json.RawMessage(eventJSON)

		result, err := processor.Handle(ctx, &rawEvent)
		// WebSocket errors should result in error response, not nil
		assert.NoError(t, err)
		assert.NotNil(t, result)
		var resp events.APIGatewayProxyResponse
		err = json.Unmarshal(*result, &resp)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("routes ECS task event correctly even with minimal detail", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{
			getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
				return nil, nil // Execution not found
			},
		}
		mockWebSocket := &mockWebSocketHandler{}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		// Test with minimal detail - empty taskArn
		detailJSON := json.RawMessage(`{}`)
		event := events.CloudWatchEvent{
			DetailType: "ECS Task State Change",
			Source:     "aws.ecs",
			Detail:     detailJSON,
		}

		eventJSON, _ := json.Marshal(event)
		rawEvent := json.RawMessage(eventJSON)

		// Should route to ECS handler (no "unhandled event type" error)
		// Empty taskArn means empty executionID, execution will be nil, returns nil (no error)
		_, err := processor.Handle(ctx, &rawEvent)
		assert.NoError(t, err) // Orphaned tasks handled gracefully
	})

	t.Run("handles CloudWatch Logs parsing error", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{}
		mockWebSocket := &mockWebSocketHandler{}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		logsEvent := events.CloudwatchLogsEvent{
			AWSLogs: events.CloudwatchLogsRawData{
				Data: "invalid-base64-data!!!",
			},
		}

		eventJSON, _ := json.Marshal(logsEvent)
		rawEvent := json.RawMessage(eventJSON)

		_, err := processor.Handle(ctx, &rawEvent)
		assert.Error(t, err)
	})
}

func TestHandleLogsEvent_Scenarios(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	t.Run("handles logs event parsing error gracefully", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{}
		mockWebSocket := &mockWebSocketHandler{}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		// Invalid base64 data that will fail to parse
		logsEvent := events.CloudwatchLogsEvent{
			AWSLogs: events.CloudwatchLogsRawData{
				Data: "invalid-base64-data!!!",
			},
		}

		eventJSON, _ := json.Marshal(logsEvent)
		rawEvent := json.RawMessage(eventJSON)

		_, err := processor.Handle(ctx, &rawEvent)
		// Should return error from parsing
		assert.Error(t, err)
	})

	t.Run("handles empty logs data", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{}
		mockWebSocket := &mockWebSocketHandler{}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		logsEvent := events.CloudwatchLogsEvent{
			AWSLogs: events.CloudwatchLogsRawData{
				Data: "", // Empty data
			},
		}

		eventJSON, _ := json.Marshal(logsEvent)
		rawEvent := json.RawMessage(eventJSON)

		// Should not be recognized as logs event
		_, err := processor.Handle(ctx, &rawEvent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unhandled event type")
	})
}

func TestHandleWebSocketEvent_Scenarios(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	t.Run("handles WebSocket event with manager error", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{}
		mockWebSocket := &mockWebSocketHandler{}

		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		wsEvent := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				RouteKey: "$connect",
			},
		}

		eventJSON, _ := json.Marshal(wsEvent)
		rawEvent := json.RawMessage(eventJSON)

		result, err := processor.Handle(ctx, &rawEvent)
		assert.NoError(t, err)
		assert.NotNil(t, result)

		var resp events.APIGatewayProxyResponse
		err = json.Unmarshal(*result, &resp)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("handles different WebSocket route keys", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{}
		mockWebSocket := &mockWebSocketHandler{}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		routeKeys := []string{"$connect", "$disconnect", "$default", "custom-route"}

		for _, routeKey := range routeKeys {
			wsEvent := events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					RouteKey: routeKey,
				},
			}

			eventJSON, _ := json.Marshal(wsEvent)
			rawEvent := json.RawMessage(eventJSON)

			result, err := processor.Handle(ctx, &rawEvent)
			assert.NoError(t, err, "route key: %s", routeKey)
			assert.NotNil(t, result, "route key: %s", routeKey)
		}
	})
}

func TestHandleEventJSON(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	t.Run("handles valid CloudWatch event JSON", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{
			getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
				return nil, fmt.Errorf("execution not found")
			},
		}
		mockWebSocket := &mockWebSocketHandler{}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		eventJSON := json.RawMessage(`{
			"detail-type": "ECS Task State Change",
			"source": "aws.ecs",
			"detail": {"taskArn": "arn:aws:ecs:us-east-1:123456789012:task/cluster/test-123"}
		}`)

		// Will error on execution lookup
		err := processor.HandleEventJSON(ctx, &eventJSON)
		assert.Error(t, err)
	})

	t.Run("handles invalid JSON", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{}
		mockWebSocket := &mockWebSocketHandler{}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		eventJSON := json.RawMessage(`invalid json`)

		err := processor.HandleEventJSON(ctx, &eventJSON)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal event")
	})

	t.Run("handles non-CloudWatch event JSON", func(t *testing.T) {
		mockRepo := &mockExecutionRepo{}
		mockWebSocket := &mockWebSocketHandler{}
		processor := NewProcessor(mockRepo, mockWebSocket, nil, logger)

		eventJSON := json.RawMessage(`{"type": "not-cloudwatch"}`)

		err := processor.HandleEventJSON(ctx, &eventJSON)
		// Should error because it's not a valid CloudWatch event structure
		assert.Error(t, err)
	})
}

// Helper function to create int pointers
func intPtr(i int) *int {
	return &i
}
