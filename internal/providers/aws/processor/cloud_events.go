package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/runvoy/runvoy/internal/backend/contract"
	"github.com/runvoy/runvoy/internal/database"
	"github.com/runvoy/runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/events"
)

// Processor implements the events.Processor interface for AWS.
// It handles CloudWatch events, CloudWatch Logs, API Gateway WebSocket events, and scheduled events.
type Processor struct {
	executionRepo    database.ExecutionRepository
	logEventRepo     database.LogEventRepository
	webSocketManager contract.WebSocketManager
	healthManager    contract.HealthManager
	logger           *slog.Logger
}

// NewProcessor creates a new AWS event processor.
func NewProcessor(
	executionRepo database.ExecutionRepository,
	logEventRepo database.LogEventRepository,
	webSocketManager contract.WebSocketManager,
	healthManager contract.HealthManager,
	log *slog.Logger,
) *Processor {
	return &Processor{
		executionRepo:    executionRepo,
		logEventRepo:     logEventRepo,
		webSocketManager: webSocketManager,
		healthManager:    healthManager,
		logger:           log,
	}
}

// Handle processes a raw AWS event by delegating to the appropriate handler.
// It supports CloudWatch events, CloudWatch Logs, and WebSocket events.
func (p *Processor) Handle(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	// Try cloud-specific events
	if handled, err := p.handleCloudEvent(ctx, rawEvent, reqLogger); handled {
		return nil, err
	}

	// Try logs events
	if handled, err := p.handleLogsEvent(ctx, rawEvent, reqLogger); handled {
		return nil, err
	}

	// Try WebSocket events
	if resp, handled := p.handleWebSocketEvent(ctx, rawEvent, reqLogger); handled {
		marshaled, err := json.Marshal(resp)
		if err != nil {
			reqLogger.Error("failed to marshal response", "error", err)
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}
		result := json.RawMessage(marshaled)
		return &result, nil
	}

	return nil, fmt.Errorf("unhandled event type: %s", string(*rawEvent))
}

// HandleEventJSON is a helper for testing that accepts raw JSON and returns an error.
// It's used for test cases that expect error returns.
func (p *Processor) HandleEventJSON(ctx context.Context, eventJSON *json.RawMessage) error {
	var event events.CloudWatchEvent
	if err := json.Unmarshal(*eventJSON, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	if _, err := p.Handle(ctx, eventJSON); err != nil {
		return err
	}
	return nil
}

// handleCloudEvent processes CloudWatch events (ECS task state changes and scheduled events).
func (p *Processor) handleCloudEvent(
	ctx context.Context,
	rawEvent *json.RawMessage,
	reqLogger *slog.Logger,
) (bool, error) {
	var cwEvent events.CloudWatchEvent
	if err := json.Unmarshal(*rawEvent, &cwEvent); err != nil || cwEvent.Source == "" || cwEvent.DetailType == "" {
		return false, nil
	}

	reqLogger.Debug("processing CloudWatch event",
		"context", map[string]string{
			"source":      cwEvent.Source,
			"detail_type": cwEvent.DetailType,
		},
	)

	switch cwEvent.DetailType {
	case "ECS Task State Change":
		return true, p.handleECSTaskEvent(ctx, &cwEvent, reqLogger)
	case "Scheduled Event":
		return true, p.handleScheduledEvent(ctx, &cwEvent, reqLogger)
	default:
		reqLogger.Warn("ignoring unhandled CloudWatch event detail type",
			"context", map[string]string{
				"detail_type": cwEvent.DetailType,
				"source":      cwEvent.Source,
			},
		)
		return true, nil
	}
}
