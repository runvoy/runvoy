// Package events provides a backend interface for event processing.
package events

import (
	"context"
	"encoding/json"
	"log/slog"
)

// WebSocketResponse represents a generic WebSocket event response.
// This abstraction allows different cloud providers to return responses
// without coupling to provider-specific types (e.g., AWS API Gateway).
type WebSocketResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       string
}

// Backend abstracts provider-specific event processing operations.
// This allows the event processor to handle events from different cloud providers.
type Backend interface {
	// HandleCloudEvent processes cloud-specific events (e.g., ECS task completion).
	// Returns true if the event was handled, false if it should be passed to the next handler.
	HandleCloudEvent(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error)

	// HandleLogsEvent processes cloud-specific log events.
	// Returns true if the event was handled, false if it should be passed to the next handler.
	HandleLogsEvent(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error)

	// HandleWebSocketEvent processes WebSocket events.
	// Returns the response and true if the event was handled, false otherwise.
	HandleWebSocketEvent(
		ctx context.Context,
		rawEvent *json.RawMessage,
		reqLogger *slog.Logger,
	) (WebSocketResponse, bool)
}
