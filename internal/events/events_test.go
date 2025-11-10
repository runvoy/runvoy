package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	eventsAws "runvoy/internal/events/aws"
	"runvoy/internal/api"
	"runvoy/internal/constants"
	"runvoy/internal/testutil"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
)

// Mock backend for testing
type mockBackend struct {
	handleCloudEventFunc     func(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error)
	handleLogsEventFunc      func(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error)
	handleWebSocketEventFunc func(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (events.APIGatewayProxyResponse, bool)
}

func (m *mockBackend) HandleCloudEvent(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error) {
	if m.handleCloudEventFunc != nil {
		return m.handleCloudEventFunc(ctx, rawEvent, reqLogger)
	}
	return false, nil
}

func (m *mockBackend) HandleLogsEvent(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error) {
	if m.handleLogsEventFunc != nil {
		return m.handleLogsEventFunc(ctx, rawEvent, reqLogger)
	}
	return false, nil
}

func (m *mockBackend) HandleWebSocketEvent(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (events.APIGatewayProxyResponse, bool) {
	if m.handleWebSocketEventFunc != nil {
		return m.handleWebSocketEventFunc(ctx, rawEvent, reqLogger)
	}
	return events.APIGatewayProxyResponse{}, false
}

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

// Mock connection repository for testing
type mockConnectionRepo struct {
}

func (m *mockConnectionRepo) CreateConnection(_ context.Context, _ *api.WebSocketConnection) error {
	return nil
}

func (m *mockConnectionRepo) DeleteConnections(_ context.Context, _ []string) (int, error) {
	return 0, nil
}

func (m *mockConnectionRepo) GetConnectionsByExecutionID(
	_ context.Context, _ string,
) ([]*api.WebSocketConnection, error) {
	return nil, nil
}

// Mock WebSocket handler for testing
type mockWebSocketHandler struct {
	notifyExecutionCompletionFunc func(ctx context.Context, executionID *string) error
}

func (m *mockWebSocketHandler) HandleRequest(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
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

func TestHandleEvent_IgnoresUnknownEventType(t *testing.T) {
	ctx := context.Background()

	backend := &mockBackend{
		handleCloudEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
			return false, nil
		},
		handleLogsEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
			return false, nil
		},
		handleWebSocketEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (events.APIGatewayProxyResponse, bool) {
			return events.APIGatewayProxyResponse{}, false
		},
	}

	processor := NewProcessor(backend, testutil.SilentLogger())

	event := events.CloudWatchEvent{
		DetailType: "Unknown Event Type",
		Source:     "custom.source",
	}

	eventJSON, _ := json.Marshal(event)
	rawEvent := json.RawMessage(eventJSON)

	_, err := processor.Handle(ctx, &rawEvent)
	// Should return error for unhandled event
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unhandled event type")
}

func TestHandleEventJSON(t *testing.T) {
	ctx := context.Background()

	backend := &mockBackend{
		handleCloudEventFunc: func(_ context.Context, rawEvent *json.RawMessage, _ *slog.Logger) (bool, error) {
			var cwEvent events.CloudWatchEvent
			if err := json.Unmarshal(*rawEvent, &cwEvent); err == nil && cwEvent.Source != "" {
				return true, nil
			}
			return false, nil
		},
		handleLogsEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
			return false, nil
		},
		handleWebSocketEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (events.APIGatewayProxyResponse, bool) {
			return events.APIGatewayProxyResponse{}, false
		},
	}

	processor := NewProcessor(backend, testutil.SilentLogger())

	eventJSON := json.RawMessage([]byte(`{
		"detail-type": "Unknown Event Type",
		"source": "custom.source"
	}`))

	err := processor.HandleEventJSON(ctx, &eventJSON)
	assert.NoError(t, err)
}

func TestHandleEventJSON_InvalidJSON(t *testing.T) {
	ctx := context.Background()

	backend := &mockBackend{}
	processor := NewProcessor(backend, testutil.SilentLogger())

	eventJSON := json.RawMessage([]byte(`invalid json`))

	err := processor.HandleEventJSON(ctx, &eventJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal event")
}

func TestNewProcessorForAWS(t *testing.T) {
	mockRepo := &mockExecutionRepo{}
	mockWebSocket := &mockWebSocketHandler{}
	logger := testutil.SilentLogger()

	processor := NewProcessorForAWS(mockRepo, mockWebSocket, logger)

	assert.NotNil(t, processor)
	assert.NotNil(t, processor.backend)
	assert.Equal(t, logger, processor.logger)

	// Verify the backend is an AWS backend
	_, ok := processor.backend.(*eventsAws.Backend)
	assert.True(t, ok, "Expected AWS backend type")
}

func TestHandle_CloudEvent(t *testing.T) {
	ctx := context.Background()
	handled := false

	backend := &mockBackend{
		handleCloudEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
			handled = true
			return true, nil
		},
		handleLogsEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
			return false, nil
		},
		handleWebSocketEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (events.APIGatewayProxyResponse, bool) {
			return events.APIGatewayProxyResponse{}, false
		},
	}

	processor := NewProcessor(backend, testutil.SilentLogger())

	event := events.CloudWatchEvent{
		DetailType: "Test Event",
		Source:     "test.source",
	}

	eventJSON, _ := json.Marshal(event)
	rawEvent := json.RawMessage(eventJSON)

	result, err := processor.Handle(ctx, &rawEvent)
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.True(t, handled, "Cloud event should have been handled")
}

func TestHandle_LogsEvent(t *testing.T) {
	ctx := context.Background()
	handled := false

	backend := &mockBackend{
		handleCloudEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
			return false, nil
		},
		handleLogsEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
			handled = true
			return true, nil
		},
		handleWebSocketEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (events.APIGatewayProxyResponse, bool) {
			return events.APIGatewayProxyResponse{}, false
		},
	}

	processor := NewProcessor(backend, testutil.SilentLogger())

	eventJSON := json.RawMessage([]byte(`{"test": "event"}`))

	result, err := processor.Handle(ctx, &eventJSON)
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.True(t, handled, "Logs event should have been handled")
}

func TestHandle_WebSocketEvent(t *testing.T) {
	ctx := context.Background()
	handled := false

	backend := &mockBackend{
		handleCloudEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
			return false, nil
		},
		handleLogsEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
			return false, nil
		},
		handleWebSocketEventFunc: func(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (events.APIGatewayProxyResponse, bool) {
			handled = true
			return events.APIGatewayProxyResponse{StatusCode: 200}, true
		},
	}

	processor := NewProcessor(backend, testutil.SilentLogger())

	eventJSON := json.RawMessage([]byte(`{"test": "websocket"}`))

	result, err := processor.Handle(ctx, &eventJSON)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, handled, "WebSocket event should have been handled")

	resp, ok := result.(events.APIGatewayProxyResponse)
	assert.True(t, ok)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestECSCompletionHandler(t *testing.T) {
	ctx := context.Background()

	execution := &api.Execution{
		ExecutionID: "test-exec-123",
		UserEmail:   "user@example.com",
		Command:     "echo hello",
		Status:      string(constants.ExecutionRunning),
	}

	mockRepo := &mockExecutionRepo{
		getExecutionFunc: func(_ context.Context, _ string) (*api.Execution, error) {
			return execution, nil
		},
		updateExecutionFunc: func(_ context.Context, _ *api.Execution) error {
			return nil
		},
	}

	mockConnRepo := &mockConnectionRepo{}
	mockWebSocket := &mockWebSocketHandler{
		notifyExecutionCompletionFunc: func(_ context.Context, executionID *string) error {
			assert.NotNil(t, executionID)
			return nil
		},
	}

	// Use the deprecated factory function for backward compatibility test
	handler := eventsAws.ECSCompletionHandler(mockRepo, mockConnRepo, mockWebSocket, testutil.SilentLogger())
	assert.NotNil(t, handler)

	// Create a simple event (handler will be called separately, not testing the full flow here)
	event := events.CloudWatchEvent{
		DetailType: "ECS Task State Change",
		Source:     "aws.ecs",
	}

	// Just verify handler is created properly
	assert.NotNil(t, handler)
	_ = ctx
	_ = event
}
