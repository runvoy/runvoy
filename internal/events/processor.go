package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/database"
	dynamorepo "runvoy/internal/database/dynamodb"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/events"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Processor handles async events from EventBridge
type Processor struct {
	executionRepo database.ExecutionRepository
	logger        *slog.Logger
}

// NewProcessor creates a new event processor with AWS backend
func NewProcessor(ctx context.Context, cfg *config.EventProcessorEnv, log *slog.Logger) (*Processor, error) {
	if cfg.ExecutionsTable == "" {
		return nil, fmt.Errorf("ExecutionsTable cannot be empty")
	}

	// Load AWS configuration
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	executionRepo := dynamorepo.NewExecutionRepository(dynamoClient, cfg.ExecutionsTable, log)

	log.Debug("event processor initialized", "executionsTable", cfg.ExecutionsTable)

	return &Processor{
		executionRepo: executionRepo,
		logger:        log,
	}, nil
}

// HandleEvent is the main entry point for Lambda event processing
// It routes events based on their detail-type
func (p *Processor) HandleEvent(ctx context.Context, event events.CloudWatchEvent) error {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	reqLogger.Debug("received event",
		"source", event.Source,
		"detailType", event.DetailType,
		"region", event.Region,
	)

	// Route by detail type
	switch event.DetailType {
	case "ECS Task State Change":
		return p.handleECSTaskCompletion(ctx, event)
	default:
		// Log and ignore unknown event types (don't fail)
		reqLogger.Info("ignoring unhandled event type",
			"detailType", event.DetailType,
			"source", event.Source,
		)
		return nil
	}
}

// HandleEventJSON is a helper for testing that accepts raw JSON
func (p *Processor) HandleEventJSON(ctx context.Context, eventJSON []byte) error {
	var event events.CloudWatchEvent
	if err := json.Unmarshal(eventJSON, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}
	return p.HandleEvent(ctx, event)
}
