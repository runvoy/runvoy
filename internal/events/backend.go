// Package events provides event processing interfaces and utilities.
package events

import (
	"context"
	"encoding/json"
)

// Processor defines the interface for event processing across different cloud providers.
// Each provider implements this interface to handle events specific to their platform.
type Processor interface {
	// Handle processes a raw cloud event and returns a result (or nil for non-WebSocket events).
	// The result may be an APIGatewayProxyResponse for WebSocket events.
	Handle(ctx context.Context, rawEvent *json.RawMessage) (any, error)

	// HandleEventJSON is a helper for testing that accepts raw JSON and returns an error.
	// It's used for test cases that expect error returns.
	HandleEventJSON(ctx context.Context, eventJSON *json.RawMessage) error
}
