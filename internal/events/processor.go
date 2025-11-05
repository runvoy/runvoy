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
			"executions_table":                cfg.ExecutionsTable,
			"web_socket_connections_table":    cfg.WebSocketConnectionsTable,
			"websocket_manager_function_name": cfg.WebSocketManagerFunctionName,
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

// HandleEvent is the main entry point for Lambda event processing
// It routes events based on their detail-type
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
