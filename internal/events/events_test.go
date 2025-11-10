package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"runvoy/internal/testutil"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
)

// Mock backend for testing
type mockBackend struct {
	handleCloudEventFunc func(
		ctx context.Context,
		rawEvent *json.RawMessage,
		reqLogger *slog.Logger,
	) (bool, error)
	handleLogsEventFunc func(
		ctx context.Context,
		rawEvent *json.RawMessage,
		reqLogger *slog.Logger,
	) (bool, error)
	handleWebSocketEventFunc func(
		ctx context.Context,
		rawEvent *json.RawMessage,
		reqLogger *slog.Logger,
	) (events.APIGatewayProxyResponse, bool)
}

func (m *mockBackend) HandleCloudEvent(
	ctx context.Context,
	rawEvent *json.RawMessage,
	reqLogger *slog.Logger,
) (bool, error) {
	if m.handleCloudEventFunc != nil {
		return m.handleCloudEventFunc(ctx, rawEvent, reqLogger)
	}
	return false, nil
}

func (m *mockBackend) HandleLogsEvent(
	ctx context.Context,
	rawEvent *json.RawMessage,
	reqLogger *slog.Logger,
) (bool, error) {
	if m.handleLogsEventFunc != nil {
		return m.handleLogsEventFunc(ctx, rawEvent, reqLogger)
	}
	return false, nil
}

func (m *mockBackend) HandleWebSocketEvent(
	ctx context.Context,
	rawEvent *json.RawMessage,
	reqLogger *slog.Logger,
) (events.APIGatewayProxyResponse, bool) {
	if m.handleWebSocketEventFunc != nil {
		return m.handleWebSocketEventFunc(ctx, rawEvent, reqLogger)
	}
	return events.APIGatewayProxyResponse{}, false
}

func TestHandleEvent_IgnoresUnknownEventType(t *testing.T) {
	ctx := context.Background()

	backend := &mockBackend{
		handleCloudEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (bool, error) {
			return false, nil
		},
		handleLogsEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (bool, error) {
			return false, nil
		},
		handleWebSocketEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (events.APIGatewayProxyResponse, bool) {
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
		handleCloudEventFunc: func(
			_ context.Context,
			rawEvent *json.RawMessage,
			_ *slog.Logger,
		) (bool, error) {
			var cwEvent events.CloudWatchEvent
			if err := json.Unmarshal(*rawEvent, &cwEvent); err == nil && cwEvent.Source != "" {
				return true, nil
			}
			return false, nil
		},
		handleLogsEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (bool, error) {
			return false, nil
		},
		handleWebSocketEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (events.APIGatewayProxyResponse, bool) {
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

func TestHandle_CloudEvent(t *testing.T) {
	ctx := context.Background()
	handled := false

	backend := &mockBackend{
		handleCloudEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (bool, error) {
			handled = true
			return true, nil
		},
		handleLogsEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (bool, error) {
			return false, nil
		},
		handleWebSocketEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (events.APIGatewayProxyResponse, bool) {
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
		handleCloudEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (bool, error) {
			return false, nil
		},
		handleLogsEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (bool, error) {
			handled = true
			return true, nil
		},
		handleWebSocketEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (events.APIGatewayProxyResponse, bool) {
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
		handleCloudEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (bool, error) {
			return false, nil
		},
		handleLogsEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (bool, error) {
			return false, nil
		},
		handleWebSocketEventFunc: func(
			_ context.Context,
			_ *json.RawMessage,
			_ *slog.Logger,
		) (events.APIGatewayProxyResponse, bool) {
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
