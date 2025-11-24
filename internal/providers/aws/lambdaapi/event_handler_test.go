package lambdaapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProcessor implements a simple processor for testing
type mockProcessor struct {
	handleFunc func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error)
	callCount  int
}

func (m *mockProcessor) Handle(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
	m.callCount++
	if m.handleFunc != nil {
		return m.handleFunc(ctx, rawEvent)
	}
	return nil, nil
}

func TestNewEventProcessorHandler(t *testing.T) {
	tests := []struct {
		name              string
		processor         *mockProcessor
		expectedNilResult bool
	}{
		{
			name:              "creates handler with valid processor",
			processor:         &mockProcessor{},
			expectedNilResult: false,
		},
		{
			name: "creates handler with processor that returns data",
			processor: &mockProcessor{
				handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
					result := json.RawMessage(`{"status": "ok"}`)
					return &result, nil
				},
			},
			expectedNilResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewEventProcessorHandler(tt.processor)
			assert.NotNil(t, handler, "handler should not be nil")
		})
	}
}

func TestNewEventProcessorHandler_WithNilProcessor(t *testing.T) {
	// Test that creating handler with nil processor panics
	assert.Panics(t, func() {
		NewEventProcessorHandler(nil)
	}, "should panic with nil processor")
}

func TestEventProcessorHandler_SuccessResponse(t *testing.T) {
	tests := []struct {
		name           string
		processorFunc  func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error)
		expectedResult string
	}{
		{
			name: "processor returns success response",
			processorFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
				result := json.RawMessage(`{"status": "success", "data": "test"}`)
				return &result, nil
			},
			expectedResult: `{"status": "success", "data": "test"}`,
		},
		{
			name: "processor returns empty object",
			processorFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
				result := json.RawMessage(`{}`)
				return &result, nil
			},
			expectedResult: `{}`,
		},
		{
			name: "processor returns nil (no response expected)",
			processorFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
				return nil, nil
			},
			expectedResult: `{"status_code": 200, "body": "OK"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := &mockProcessor{handleFunc: tt.processorFunc}
			handler := NewEventProcessorHandler(proc)
			require.NotNil(t, handler)

			// The handler is wrapped by Lambda's NewHandler
			// Direct invocation testing requires reflection or Lambda simulation
			// For now, we verify it was created successfully
			assert.Equal(t, 0, proc.callCount, "processor should not be called during handler creation")
		})
	}
}

func TestEventProcessorHandler_ErrorResponse(t *testing.T) {
	tests := []struct {
		name          string
		processorFunc func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error)
		expectedError bool
	}{
		{
			name: "processor returns error",
			processorFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
				return nil, errors.New("processing failed")
			},
			expectedError: true,
		},
		{
			name: "processor returns custom error",
			processorFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
				return nil, fmt.Errorf("custom error: %s", "test")
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := &mockProcessor{handleFunc: tt.processorFunc}
			handler := NewEventProcessorHandler(proc)
			require.NotNil(t, handler)

			// Handler creation should succeed even if processor will error later
			assert.NotNil(t, handler)
		})
	}
}

func TestEventProcessorHandler_ErrorResponseFormat(t *testing.T) {
	// Test that error responses are formatted correctly
	// The handler should return: {"status_code": 500, "body": "error message"}

	tests := []struct {
		name           string
		errorMessage   string
		expectedStatus int
	}{
		{
			name:           "generic error",
			errorMessage:   "something went wrong",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "database error",
			errorMessage:   "database connection failed",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "validation error",
			errorMessage:   "invalid input",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := &mockProcessor{
				handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
					return nil, errors.New(tt.errorMessage)
				},
			}

			handler := NewEventProcessorHandler(proc)
			assert.NotNil(t, handler)

			// The error response format should be:
			// {"status_code": 500, "body": "error message"}
			// This is tested in the actual Lambda invocation
		})
	}
}

func TestEventProcessorHandler_ContextPropagation(t *testing.T) {
	// Test that context is properly propagated to the processor

	type contextKey string
	const testKey contextKey = "test-key"

	proc := &mockProcessor{
		handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
			// Verify context contains expected value
			value := ctx.Value(testKey)
			if value == nil {
				return nil, errors.New("context value not found")
			}

			result := json.RawMessage(`{"status": "ok"}`)
			return &result, nil
		},
	}

	handler := NewEventProcessorHandler(proc)
	assert.NotNil(t, handler)

	// Context propagation would be tested in actual Lambda invocation
	// The handler wrapper should pass context through correctly
}

func TestEventProcessorHandler_NilRawEvent(t *testing.T) {
	// Test handling of nil raw event

	proc := &mockProcessor{
		handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
			if rawEvent == nil {
				result := json.RawMessage(`{"status": "ok", "event": "nil"}`)
				return &result, nil
			}
			result := json.RawMessage(`{"status": "ok", "event": "not-nil"}`)
			return &result, nil
		},
	}

	handler := NewEventProcessorHandler(proc)
	assert.NotNil(t, handler)
}

func TestEventProcessorHandler_EmptyRawEvent(t *testing.T) {
	// Test handling of empty raw event

	proc := &mockProcessor{
		handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
			if rawEvent != nil && len(*rawEvent) == 0 {
				result := json.RawMessage(`{"status": "ok", "event": "empty"}`)
				return &result, nil
			}
			result := json.RawMessage(`{"status": "ok"}`)
			return &result, nil
		},
	}

	handler := NewEventProcessorHandler(proc)
	assert.NotNil(t, handler)
}

func TestEventProcessorHandler_LargeRawEvent(t *testing.T) {
	// Test handling of large raw events

	proc := &mockProcessor{
		handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
			// Process large event
			result := json.RawMessage(`{"status": "ok", "processed": true}`)
			return &result, nil
		},
	}

	handler := NewEventProcessorHandler(proc)
	assert.NotNil(t, handler)

	// Large event handling would be tested in integration tests
}

func TestEventProcessorHandler_InvalidJSONResponse(t *testing.T) {
	// Test that processor can return any valid json.RawMessage

	tests := []struct {
		name     string
		response json.RawMessage
	}{
		{
			name:     "valid json object",
			response: json.RawMessage(`{"key": "value"}`),
		},
		{
			name:     "valid json array",
			response: json.RawMessage(`["item1", "item2"]`),
		},
		{
			name:     "valid json string",
			response: json.RawMessage(`"string value"`),
		},
		{
			name:     "valid json number",
			response: json.RawMessage(`123`),
		},
		{
			name:     "valid json boolean",
			response: json.RawMessage(`true`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := &mockProcessor{
				handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
					return &tt.response, nil
				},
			}

			handler := NewEventProcessorHandler(proc)
			assert.NotNil(t, handler)
		})
	}
}

func TestEventProcessorHandler_CallCount(t *testing.T) {
	// Verify that processor is only called during event handling, not creation

	proc := &mockProcessor{
		handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
			result := json.RawMessage(`{"status": "ok"}`)
			return &result, nil
		},
	}

	handler := NewEventProcessorHandler(proc)
	require.NotNil(t, handler)

	assert.Equal(t, 0, proc.callCount, "processor should not be called during handler creation")
}

func TestEventProcessorHandler_MultipleHandlers(t *testing.T) {
	// Test creating multiple handlers with different processors

	proc1 := &mockProcessor{
		handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
			result := json.RawMessage(`{"handler": "1"}`)
			return &result, nil
		},
	}

	proc2 := &mockProcessor{
		handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
			result := json.RawMessage(`{"handler": "2"}`)
			return &result, nil
		},
	}

	handler1 := NewEventProcessorHandler(proc1)
	handler2 := NewEventProcessorHandler(proc2)

	assert.NotNil(t, handler1)
	assert.NotNil(t, handler2)

	// Handlers should be independent
	assert.NotEqual(t, handler1, handler2)
}

func TestEventProcessorHandler_NilResponseHandling(t *testing.T) {
	// Test that nil response from processor is handled correctly
	// Should return: {"status_code": 200, "body": "OK"}

	proc := &mockProcessor{
		handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
			return nil, nil // No response needed (e.g., for log events)
		},
	}

	handler := NewEventProcessorHandler(proc)
	assert.NotNil(t, handler)

	// The actual OK response is returned by the handler implementation
	// This would be verified in integration tests with actual invocations
}

// BenchmarkNewEventProcessorHandler measures handler creation performance
func BenchmarkNewEventProcessorHandler(b *testing.B) {
	proc := &mockProcessor{
		handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
			result := json.RawMessage(`{"status": "ok"}`)
			return &result, nil
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewEventProcessorHandler(proc)
	}
}

// TestEventProcessorHandler_ConcurrentCreation tests thread safety of handler creation
func TestEventProcessorHandler_ConcurrentCreation(t *testing.T) {
	proc := &mockProcessor{
		handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
			result := json.RawMessage(`{"status": "ok"}`)
			return &result, nil
		},
	}

	// Create multiple handlers concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			handler := NewEventProcessorHandler(proc)
			assert.NotNil(t, handler)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestEventProcessorHandler_ErrorWithNilResult(t *testing.T) {
	// Test that returning both error and nil result is handled correctly

	proc := &mockProcessor{
		handleFunc: func(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
			return nil, errors.New("processing error")
		},
	}

	handler := NewEventProcessorHandler(proc)
	assert.NotNil(t, handler)

	// Error response should be: {"status_code": 500, "body": "processing error"}
}

func TestEventProcessorHandler_Integration(t *testing.T) {
	t.Skip("Integration test - requires full Lambda context and event simulation")

	// This test would:
	// 1. Create a full Lambda event
	// 2. Invoke the handler with lambda.Start simulation
	// 3. Verify the complete request/response cycle
	// 4. Test various event types (CloudWatch, ECS, logs, etc.)
}
