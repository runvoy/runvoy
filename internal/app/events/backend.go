// Package events provides event processing interfaces and utilities.
package events

import (
	"context"
	"encoding/json"
)

// WebSocketResponse represents a generic HTTP response for WebSocket events.
// This type is provider-agnostic and can be adapted to any cloud provider's
// response format (AWS API Gateway, GCP Cloud Run, Azure Function HTTP response, etc.).
type WebSocketResponse struct {
	// StatusCode is the HTTP status code (e.g., 200, 400, 500)
	StatusCode int

	// Headers are the HTTP response headers
	Headers map[string]string

	// Body is the response body content
	Body string
}

// Processor defines the interface for event processing across different cloud providers.
// Each provider implements this interface to handle events specific to their platform.
type Processor interface {
	// Handle processes a raw cloud event and returns a result (or nil for non-WebSocket events).
	// The result may be a WebSocketResponse for WebSocket events.
	Handle(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error)

	// HandleEventJSON is a helper for testing that accepts raw JSON and returns an error.
	// It's used for test cases that expect error returns.
	HandleEventJSON(ctx context.Context, eventJSON *json.RawMessage) error
}
