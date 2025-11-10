package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
)

// WebSocketEvent represents a parsed WebSocket connection event in a provider-agnostic format.
type WebSocketEvent struct {
	RouteKey     string            // The WebSocket route (e.g., "$connect", "$disconnect")
	ConnectionID string            // Unique identifier for this connection
	QueryParams  map[string]string // Query string parameters from the connection request
	ClientIP     string            // Client IP address if available
}

// WebSocketResponse represents a provider-agnostic WebSocket response.
type WebSocketResponse struct {
	StatusCode int    // HTTP status code
	Body       string // Response body
}

// Backend abstracts provider-specific WebSocket operations.
// This allows the WebSocket manager to work with different cloud providers'
// WebSocket implementations (AWS API Gateway, GCP Cloud Run WebSockets, etc.).
type Backend interface {
	// ParseEvent parses a raw WebSocket event into a provider-agnostic format.
	// Returns nil if the event is not a WebSocket event.
	ParseEvent(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (*WebSocketEvent, error)

	// SendToConnection sends data to a specific WebSocket connection.
	SendToConnection(ctx context.Context, connectionID string, data []byte) error

	// BuildResponse creates a provider-specific response for a WebSocket event.
	BuildResponse(statusCode int, body string) any
}
