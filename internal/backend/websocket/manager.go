package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
)

// Manager exposes the subset of WebSocket manager functionality used by the event processor.
// Different cloud providers can implement this interface to support their specific WebSocket infrastructure.
type Manager interface {
	// HandleRequest processes WebSocket lifecycle events (connect, disconnect, etc.).
	// Returns true if the event was handled, false otherwise.
	HandleRequest(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error)

	// NotifyExecutionCompletion sends disconnect notifications to all connected clients for an execution
	// and removes the connections.
	NotifyExecutionCompletion(ctx context.Context, executionID *string) error

	// SendLogsToExecution flushes buffered log events to all connected clients for an execution.
	SendLogsToExecution(ctx context.Context, executionID *string) error

	// GenerateWebSocketURL creates a WebSocket token and returns the connection URL.
	// It stores the token for validation when the client connects.
	// Returns an empty string if URL generation fails (errors are logged).
	GenerateWebSocketURL(
		ctx context.Context,
		executionID string,
		userEmail *string,
		clientIPAtCreationTime *string,
	) string
}
