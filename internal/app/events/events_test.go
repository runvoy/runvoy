package events

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
)

// Mock processor for testing
type mockProcessor struct {
	handleFunc          func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error)
	handleEventJSONFunc func(ctx context.Context, eventJSON *json.RawMessage) error
}

func (m *mockProcessor) Handle(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
	if m.handleFunc != nil {
		return m.handleFunc(ctx, rawEvent)
	}
	return nil, nil
}

func (m *mockProcessor) HandleEventJSON(ctx context.Context, eventJSON *json.RawMessage) error {
	if m.handleEventJSONFunc != nil {
		return m.handleEventJSONFunc(ctx, eventJSON)
	}
	return nil
}

func TestHandleEvent_IgnoresUnknownEventType(t *testing.T) {
	ctx := context.Background()

	processor := &mockProcessor{
		handleFunc: func(_ context.Context, _ *json.RawMessage) (*json.RawMessage, error) {
			return nil, fmt.Errorf("unhandled event type")
		},
	}

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

	processor := &mockProcessor{
		handleEventJSONFunc: func(_ context.Context, _ *json.RawMessage) error {
			return nil
		},
	}

	eventJSON := json.RawMessage([]byte(`{
		"detail-type": "Unknown Event Type",
		"source": "custom.source"
	}`))

	err := processor.HandleEventJSON(ctx, &eventJSON)
	assert.NoError(t, err)
}

func TestHandleEventJSON_InvalidJSON(t *testing.T) {
	ctx := context.Background()

	processor := &mockProcessor{
		handleEventJSONFunc: func(_ context.Context, _ *json.RawMessage) error {
			return fmt.Errorf("failed to unmarshal event")
		},
	}

	eventJSON := json.RawMessage([]byte(`invalid json`))

	err := processor.HandleEventJSON(ctx, &eventJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal event")
}

func TestHandle_CloudEvent(t *testing.T) {
	ctx := context.Background()
	handled := false

	processor := &mockProcessor{
		handleFunc: func(_ context.Context, _ *json.RawMessage) (*json.RawMessage, error) {
			handled = true
			return nil, nil
		},
	}

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

	processor := &mockProcessor{
		handleFunc: func(_ context.Context, _ *json.RawMessage) (*json.RawMessage, error) {
			handled = true
			return nil, nil
		},
	}

	eventJSON := json.RawMessage([]byte(`{"test": "event"}`))

	result, err := processor.Handle(ctx, &eventJSON)
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.True(t, handled, "Logs event should have been handled")
}

func TestHandle_WebSocketEvent(t *testing.T) {
	ctx := context.Background()
	handled := false

	expectedResponse := events.APIGatewayProxyResponse{StatusCode: http.StatusOK}
	respBytes, _ := json.Marshal(expectedResponse)

	processor := &mockProcessor{
		handleFunc: func(_ context.Context, _ *json.RawMessage) (*json.RawMessage, error) {
			handled = true
			raw := json.RawMessage(respBytes)
			return &raw, nil
		},
	}

	eventJSON := json.RawMessage([]byte(`{"test": "websocket"}`))

	result, err := processor.Handle(ctx, &eventJSON)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, handled, "WebSocket event should have been handled")

	var resp events.APIGatewayProxyResponse
	assert.NoError(t, json.Unmarshal(*result, &resp))
	assert.Equal(t, expectedResponse.StatusCode, resp.StatusCode)
}
