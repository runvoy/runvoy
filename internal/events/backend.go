// Package events provides a backend interface for event processing.
package events

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
)

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
	) (events.APIGatewayProxyResponse, bool)
}
