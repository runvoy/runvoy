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
	logsRepo         database.LogsRepository
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

	// Initialize logs repository if execution logs table is configured
	var logsRepo database.LogsRepository
	if cfg.ExecutionLogsTable != "" {
		logsRepo = dynamorepo.NewLogsRepository(
			dynamoClient,
			cfg.ExecutionLogsTable,
			cfg.ExecutionLogsTTLDays,
			log,
		)
	}

	lambdaClient := lambda.NewFromConfig(awsCfg)

	log.Debug("event processor initialized",
		"context", map[string]string{
			"executions_table":             cfg.ExecutionsTable,
			"web_socket_connections_table": cfg.WebSocketConnectionsTable,
			"websocket_manager_function":   cfg.WebSocketManagerFunctionName,
			"execution_logs_table":         cfg.ExecutionLogsTable,
		},
	)

	return &Processor{
		executionRepo:    executionRepo,
		connectionRepo:   connectionRepo,
		logsRepo:         logsRepo,
		lambdaClient:     lambdaClient,
		websocketManager: cfg.WebSocketManagerFunctionName,
		logger:           log,
	}, nil
}

// HandleEvent is the main entry point for Lambda event processing from EventBridge.
// It routes events based on their detail-type.
func (p *Processor) HandleEvent(ctx context.Context, event *events.CloudWatchEvent) error {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	reqLogger.Debug("received event", "event", event)

	switch event.DetailType {
	case "ECS Task State Change":
		return p.handleECSTaskCompletion(ctx, event)
	case "CloudWatch Logs":
		return p.handleCloudWatchLogs(ctx, event)
	default:
		reqLogger.Debug("ignoring unhandled event type", "context", map[string]string{
			"detail_type": event.DetailType,
			"source":      event.Source,
		})
		return nil
	}
}

// HandleCloudWatchLogsEvent handles a direct CloudWatch Logs subscription filter Lambda invocation.
// This is invoked when CloudWatch Logs directly invokes Lambda (not through EventBridge).
// The data comes in awslogs field and is base64-encoded, gzip-compressed.
func (p *Processor) HandleCloudWatchLogsEvent(ctx context.Context, awslogsData []byte) error {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	cwLogsEvent, err := ParseDirectCloudWatchLogsEvent(awslogsData)
	if err != nil {
		reqLogger.Error("failed to parse direct CloudWatch Logs event", "error", err)
		return fmt.Errorf("failed to parse CloudWatch Logs event: %w", err)
	}

	// Extract execution ID from log stream name
	executionID := extractExecutionIDFromLogStream(cwLogsEvent.LogStream)
	if executionID == "" {
		reqLogger.Warn("could not extract execution ID from log stream",
			"log_stream", cwLogsEvent.LogStream,
		)
		return nil
	}

	reqLogger.Debug("processing direct CloudWatch Logs ingestion",
		"context", map[string]any{
			"execution_id": executionID,
			"log_stream":   cwLogsEvent.LogStream,
			"event_count":  len(cwLogsEvent.LogEvents),
		},
	)

	// Ingest logs for this execution
	ingestedCount := p.ingestExecutionLogs(ctx, executionID, cwLogsEvent.LogEvents, reqLogger)

	reqLogger.Debug("logs ingested successfully",
		"context", map[string]any{
			"execution_id":   executionID,
			"ingested_count": ingestedCount,
		},
	)

	return nil
}

// HandleEventJSON is a helper for testing that accepts raw JSON
func (p *Processor) HandleEventJSON(ctx context.Context, eventJSON []byte) error {
	var event events.CloudWatchEvent
	if err := json.Unmarshal(eventJSON, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}
	return p.HandleEvent(ctx, &event)
}
