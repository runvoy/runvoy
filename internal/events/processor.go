package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"

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
// It attempts to unmarshal as CloudWatch Event, otherwise treats as custom invoke
func (p *Processor) Handle(ctx context.Context, rawEvent json.RawMessage) error {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	// Try to unmarshal as CloudWatch Event
	var event events.CloudWatchEvent
	if err := json.Unmarshal(rawEvent, &event); err == nil && event.Source != "" && event.DetailType != "" {
		reqLogger.Debug("processing CloudWatch event",
			"source", event.Source,
			"detail_type", event.DetailType,
		)

		switch event.DetailType {
		case "ECS Task State Change":
			return p.handleECSTaskCompletion(ctx, &event)
		default:
			reqLogger.Debug("ignoring unhandled CloudWatch event detail type",
				"context", map[string]string{
					"detail_type": event.DetailType,
					"source":      event.Source,
				},
			)
			return nil
		}
	}

	reqLogger.Debug("unhandled event type", "context", map[string]string{
		"type": reflect.TypeFor[json.RawMessage]().String(),
	})
	return nil
}

// HandleEventJSON is a helper for testing that accepts raw JSON
func (p *Processor) HandleEventJSON(ctx context.Context, eventJSON []byte) error {
	var event events.CloudWatchEvent
	if err := json.Unmarshal(eventJSON, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}
	return p.Handle(ctx, eventJSON)
}
