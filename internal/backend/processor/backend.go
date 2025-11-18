// Package processor provides event processing interfaces and utilities.
package processor

import (
	"context"
	"encoding/json"
)

// Processor defines the interface for event processing across different cloud providers.
// Each provider implements this interface to handle events specific to their platform.
type Processor interface {
	// Handle processes a raw cloud event and returns a result (or nil for non-WebSocket events).
	// For WebSocket events, the result is a provider-specific response marshaled as JSON.
	Handle(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error)

	// HandleEventJSON is a helper for testing that accepts raw JSON and returns an error.
	// It's used for test cases that expect error returns.
	HandleEventJSON(ctx context.Context, eventJSON *json.RawMessage) error
}
