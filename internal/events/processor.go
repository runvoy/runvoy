package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"runvoy/internal/logger"
)

// Processor handles async events from cloud providers
type Processor struct {
	backend Backend
	logger  *slog.Logger
}

// NewProcessor creates a new event processor with the specified backend
func NewProcessor(backend Backend, log *slog.Logger) *Processor {
	return &Processor{
		backend: backend,
		logger:  log,
	}
}

// Handle is the universal entry point for event processing
// It supports CloudWatch Event, CloudWatch Logs Event and WebSocket Event natively.
// This method returns an interface{} to support both error responses (for non-WebSocket events)
// and WebSocketResponse (for WebSocket events).
// Provider-specific wrappers should convert the response to their required format.
func (p *Processor) Handle(ctx context.Context, rawEvent *json.RawMessage) (any, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	// Try cloud-specific events
	if handled, err := p.backend.HandleCloudEvent(ctx, rawEvent, reqLogger); handled {
		return nil, err
	}

	// Try logs events
	if handled, err := p.backend.HandleLogsEvent(ctx, rawEvent, reqLogger); handled {
		return nil, err
	}

	// Try WebSocket events
	if resp, handled := p.backend.HandleWebSocketEvent(ctx, rawEvent, reqLogger); handled {
		// Return the generic response - let provider-specific code convert if needed
		return resp, nil
	}

	reqLogger.Error("unhandled event type", "context", map[string]any{
		"event": *rawEvent,
	})

	return nil, fmt.Errorf("unhandled event type: %s", string(*rawEvent))
}
