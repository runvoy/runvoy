package lambdaapi

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProcessor implements processor.Processor for testing.
type mockProcessor struct {
	handleFunc          func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error)
	handleEventJSONFunc func(ctx context.Context, eventJSON *json.RawMessage) error
	callCount           int
}

func (m *mockProcessor) Handle(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
	m.callCount++
	if m.handleFunc != nil {
		return m.handleFunc(ctx, rawEvent)
	}
	return nil, nil
}

func (m *mockProcessor) HandleEventJSON(ctx context.Context, eventJSON *json.RawMessage) error {
	if m.handleEventJSONFunc != nil {
		return m.handleEventJSONFunc(ctx, eventJSON)
	}
	_, err := m.Handle(ctx, eventJSON)
	return err
}

func TestNewEventProcessorHandler(t *testing.T) {
	handler := NewEventProcessorHandler(&mockProcessor{})
	assert.NotNil(t, handler)
}

func TestNewEventProcessorHandler_WithNilProcessor(t *testing.T) {
	assert.Panics(t, func() {
		NewEventProcessorHandler(nil)
	})
}

func TestEventProcessorHandler_ReturnsProcessorResult(t *testing.T) {
	expected := json.RawMessage(`{"status":"ok"}`)

	handler := NewEventProcessorHandler(&mockProcessor{
		handleFunc: func(_ context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
			require.NotNil(t, rawEvent)
			return &expected, nil
		},
	})

	resp, err := handler.Invoke(context.Background(), []byte(`{"hello":"world"}`))
	require.NoError(t, err)
	assert.JSONEq(t, string(expected), string(resp))
}

func TestEventProcessorHandler_ReturnsOKWhenProcessorReturnsNil(t *testing.T) {
	handler := NewEventProcessorHandler(&mockProcessor{
		handleFunc: func(_ context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
			require.NotNil(t, rawEvent)
			return nil, nil
		},
	})

	resp, err := handler.Invoke(context.Background(), []byte(`{}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"status_code":200,"body":"OK"}`, string(resp))
}

func TestEventProcessorHandler_ReturnsErrorPayload(t *testing.T) {
	handler := NewEventProcessorHandler(&mockProcessor{
		handleFunc: func(_ context.Context, _ *json.RawMessage) (*json.RawMessage, error) {
			return nil, errors.New("processing failed")
		},
	})

	resp, err := handler.Invoke(context.Background(), []byte(`{}`))
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "processing failed")
}
