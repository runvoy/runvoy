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
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// Processor handles async events from EventBridge
type Processor struct {
	executionRepo    database.ExecutionRepository
	connectionRepo   database.ConnectionRepository
	lambdaClient     *lambda.Client
	websocketManager string // Lambda function name for websocket_manager
	logger           *slog.Logger
}

// NewProcessor creates a new event processor with AWS backend
func NewProcessor(ctx context.Context, cfg *config.Config, log *slog.Logger) (*Processor, error) {
	if cfg.ExecutionsTable == "" {
		return nil, fmt.Errorf("ExecutionsTable cannot be empty")
	}
	if cfg.WebSocketConnectionsTable == "" {
		return nil, fmt.Errorf("WebSocketConnectionsTable cannot be empty")
	}
	if cfg.WebSocketManagerFunctionName == "" {
		return nil, fmt.Errorf("WebSocketManagerFunctionName cannot be empty")
	}

	// Load AWS configuration
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	executionRepo := dynamorepo.NewExecutionRepository(dynamoClient, cfg.ExecutionsTable, log)
	connectionRepo := dynamorepo.NewConnectionRepository(dynamoClient, cfg.WebSocketConnectionsTable, log)

	lambdaClient := lambda.NewFromConfig(awsCfg)

	log.Debug("event processor initialized",
		"context", map[string]string{
			"executions_table":             cfg.ExecutionsTable,
			"web_socket_connections_table": cfg.WebSocketConnectionsTable,
			"websocket_manager_function":   cfg.WebSocketManagerFunctionName,
		},
	)

	return &Processor{
		executionRepo:    executionRepo,
		connectionRepo:   connectionRepo,
		lambdaClient:     lambdaClient,
		websocketManager: cfg.WebSocketManagerFunctionName,
		logger:           log,
	}, nil
}

// Handle is the universal entry point for Lambda event processing
// It detects the event type and routes to the appropriate handler
func (p *Processor) Handle(ctx context.Context, rawEvent json.RawMessage) error {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	eventType := DetectEventType(rawEvent)

	reqLogger.Info("received Lambda event",
		"eventType", eventType.String(),
		"eventSize", len(rawEvent),
	)

	switch eventType {
	case EventTypeCloudWatch:
		return p.handleCloudWatchEvent(ctx, rawEvent, reqLogger)
	case EventTypeAPIGatewayWebSocket,
		EventTypeAPIGatewayProxy,
		EventTypeCloudWatchLogs,
		EventTypeSNS,
		EventTypeSQS:
		reqLogger.Warn("received unsupported event type",
			"eventType", eventType.String(),
		)
		return nil
	default:
		reqLogger.Warn("received unknown event type, treating as custom invoke")
		return p.handleCustomInvoke(ctx, rawEvent, reqLogger)
	}
}

// handleCloudWatchEvent processes CloudWatch Events from EventBridge
func (p *Processor) handleCloudWatchEvent(ctx context.Context, rawEvent json.RawMessage, reqLogger *slog.Logger) error {
	var event events.CloudWatchEvent
	if err := json.Unmarshal(rawEvent, &event); err != nil {
		return fmt.Errorf("failed to unmarshal CloudWatch event: %w", err)
	}

	reqLogger.Debug("processing CloudWatch event",
		"source", event.Source,
		"detailType", event.DetailType,
	)

	switch event.DetailType {
	case "ECS Task State Change":
		return p.handleECSTaskCompletion(ctx, &event)
	default:
		reqLogger.Info("ignoring unhandled CloudWatch event detail type",
			"detailType", event.DetailType,
			"source", event.Source,
		)
		return nil
	}
}

// handleCustomInvoke handles direct Lambda invocations with custom payloads
func (p *Processor) handleCustomInvoke(ctx context.Context, rawEvent json.RawMessage, reqLogger *slog.Logger) error {
	reqLogger.Info("processing custom Lambda invoke")

	var customPayload map[string]interface{}
	if err := json.Unmarshal(rawEvent, &customPayload); err != nil {
		return fmt.Errorf("failed to unmarshal custom payload: %w", err)
	}

	reqLogger.Debug("custom payload received", "payload", customPayload)

	// For now, just log and return - can be extended later
	return nil
}

// HandleEvent is the legacy entry point for Lambda event processing
// Deprecated: Use Handle instead for universal event handling
func (p *Processor) HandleEvent(ctx context.Context, event *events.CloudWatchEvent) error {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	reqLogger.Debug("received event", "event", event)

	switch event.DetailType {
	case "ECS Task State Change":
		return p.handleECSTaskCompletion(ctx, event)
	default:
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
	return p.HandleEvent(ctx, &event)
}
