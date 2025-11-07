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
	"runvoy/internal/websocket"

	"github.com/aws/aws-lambda-go/events"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// WebSocketHandler stores the subset of WebSocket manager functionality used by the processor.
type WebSocketHandler interface {
	HandleRequest(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error)
}

// Processor handles async events from EventBridge
type Processor struct {
	executionRepo    database.ExecutionRepository
	connectionRepo   database.ConnectionRepository
	websocketHandler WebSocketHandler
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
	if cfg.WebSocketAPIEndpoint == "" {
		return nil, fmt.Errorf("WebSocketAPIEndpoint cannot be empty")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	executionRepo := dynamorepo.NewExecutionRepository(dynamoClient, cfg.ExecutionsTable, log)
	connectionRepo := dynamorepo.NewConnectionRepository(dynamoClient, cfg.WebSocketConnectionsTable, log)

	websocketManager, err := websocket.NewWebSocketManager(ctx, cfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebSocket manager: %w", err)
	}

	log.Debug("event processor initialized",
		"context", map[string]string{
			"executions_table":             cfg.ExecutionsTable,
			"web_socket_connections_table": cfg.WebSocketConnectionsTable,
			"websocket_api_endpoint":       cfg.WebSocketAPIEndpoint,
		},
	)

	return &Processor{
		executionRepo:    executionRepo,
		connectionRepo:   connectionRepo,
		websocketHandler: websocketManager,
		logger:           log,
	}, nil
}

// Handle is the universal entry point for Lambda event processing
// It attempts to unmarshal as CloudWatch Event or CloudWatch Logs Event
func (p *Processor) Handle(ctx context.Context, rawEvent *json.RawMessage) error {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	if handled, err := p.handleCloudWatchEvent(ctx, rawEvent, reqLogger); handled {
		return err
	}

	if handled, err := p.handleCloudWatchLogsEvent(ctx, rawEvent, reqLogger); handled {
		return err
	}

	if handled, err := p.handleWebSocketEvent(ctx, rawEvent, reqLogger); handled {
		return err
	}

	reqLogger.Warn("unhandled event type", "context", map[string]any{
		"event": *rawEvent,
	})
	return nil
}

// HandleEventJSON is a helper for testing that accepts raw JSON
func (p *Processor) HandleEventJSON(ctx context.Context, eventJSON *json.RawMessage) error {
	var event events.CloudWatchEvent
	if err := json.Unmarshal(*eventJSON, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}
	return p.Handle(ctx, eventJSON)
}

func (p *Processor) handleCloudWatchEvent(
	ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error) {
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
		return true, p.handleECSTaskCompletion(ctx, &cwEvent)
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

func (p *Processor) handleCloudWatchLogsEvent(
	_ context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error) { //nolint:unparam
	var cwLogsEvent events.CloudwatchLogsEvent
	if err := json.Unmarshal(*rawEvent, &cwLogsEvent); err != nil || cwLogsEvent.AWSLogs.Data == "" {
		return false, nil
	}

	reqLogger.Debug("processing CloudWatch Logs event, not implemented yet")

	return true, nil
}

func (p *Processor) handleWebSocketEvent(
	ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error) {
	var webSocketEvent events.APIGatewayWebsocketProxyRequest
	if err := json.Unmarshal(*rawEvent, &webSocketEvent); err != nil {
		return false, nil
	}

	reqLogger.Debug("processing WebSocket event",
		"context", map[string]string{
			"route_key": webSocketEvent.RequestContext.RouteKey,
		},
	)

	if p.websocketHandler == nil {
		reqLogger.Warn("websocket handler not configured")
		return true, fmt.Errorf("websocket handler not configured")
	}

	resp, err := p.websocketHandler.HandleRequest(ctx, webSocketEvent)
	if err != nil {
		reqLogger.Error("failed to handle WebSocket event", "error", err)
		return true, err
	}

	reqLogger.Debug("websocket event handled",
		"context", map[string]any{
			"status_code": resp.StatusCode,
		})

	return true, nil
}
