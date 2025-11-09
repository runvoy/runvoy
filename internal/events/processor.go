package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"runvoy/internal/api"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	dynamorepo "runvoy/internal/database/dynamodb"
	"runvoy/internal/logger"
	"runvoy/internal/websocket"

	"github.com/aws/aws-lambda-go/events"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Processor handles async events from EventBridge
type Processor struct {
	executionRepo    database.ExecutionRepository
	connectionRepo   database.ConnectionRepository
	webSocketManager websocket.Manager
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
	if cfg.WebSocketTokensTable == "" {
		return nil, fmt.Errorf("WebSocketTokensTable cannot be empty")
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
	tokenRepo := dynamorepo.NewTokenRepository(dynamoClient, cfg.WebSocketTokensTable, log)
	websocketManager := websocket.NewWebSocketManager(cfg, &awsCfg, connectionRepo, tokenRepo, log)

	log.Debug("event processor initialized",
		"context", map[string]string{
			"executions_table":             cfg.ExecutionsTable,
			"web_socket_connections_table": cfg.WebSocketConnectionsTable,
			"web_socket_tokens_table":      cfg.WebSocketTokensTable,
			"websocket_api_endpoint":       cfg.WebSocketAPIEndpoint,
		},
	)

	return &Processor{
		executionRepo:    executionRepo,
		connectionRepo:   connectionRepo,
		webSocketManager: websocketManager,
		logger:           log,
	}, nil
}

// Handle is the universal entry point for Lambda event processing
// It supports CloudWatch Event, CloudWatch Logs Event and WebSocket Event natively.
// This method returns an interface{} to support both error responses (for non-WebSocket events)
// and APIGatewayProxyResponse (for WebSocket events).
func (p *Processor) Handle(ctx context.Context, rawEvent *json.RawMessage) (any, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	if handled, err := p.handleCloudWatchEvent(ctx, rawEvent, reqLogger); handled {
		return nil, err
	}

	if handled, err := p.handleCloudWatchLogsEvent(ctx, rawEvent, reqLogger); handled {
		return nil, err
	}

	// Try to handle as WebSocket event and return the response directly
	if resp, handled := p.handleWebSocketRequest(ctx, rawEvent, reqLogger); handled {
		return resp, nil
	}

	reqLogger.Error("unhandled event type", "context", map[string]any{
		"event": *rawEvent,
	})

	return nil, fmt.Errorf("unhandled event type: %s", string(*rawEvent))
}

// handleWebSocketRequest processes WebSocket events and returns the APIGatewayProxyResponse
// If the event is not a WebSocket event, it returns an empty response and false.
func (p *Processor) handleWebSocketRequest(
	ctx context.Context,
	rawEvent *json.RawMessage,
	reqLogger *slog.Logger,
) (events.APIGatewayProxyResponse, bool) {
	var wsReq events.APIGatewayWebsocketProxyRequest
	if err := json.Unmarshal(*rawEvent, &wsReq); err != nil || wsReq.RequestContext.RouteKey == "" {
		return events.APIGatewayProxyResponse{}, false
	}

	// This is a WebSocket request, handle it through the manager
	if _, err := p.webSocketManager.HandleRequest(ctx, rawEvent, reqLogger); err != nil {
		// Return error response to API Gateway
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Internal server error: %v", err),
		}, true
	}

	// Build the response based on the route
	resp := events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "OK",
	}

	return resp, true
}

// HandleEventJSON is a helper for testing that accepts raw JSON and returns an error.
// It's used for test cases that expect error returns.
func (p *Processor) HandleEventJSON(ctx context.Context, eventJSON *json.RawMessage) error {
	var event events.CloudWatchEvent
	if err := json.Unmarshal(*eventJSON, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	result, err := p.Handle(ctx, eventJSON)
	if err != nil {
		return err
	}
	// Convert result to error if it's an error type
	if resultErr, ok := result.(error); ok {
		return resultErr
	}
	return nil
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
	ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error) {
	var cwLogsEvent events.CloudwatchLogsEvent
	if err := json.Unmarshal(*rawEvent, &cwLogsEvent); err != nil || cwLogsEvent.AWSLogs.Data == "" {
		return false, nil
	}

	data, err := cwLogsEvent.AWSLogs.Parse()
	if err != nil {
		reqLogger.Error("failed to parse CloudWatch Logs data",
			"error", err,
		)
		return true, err
	}

	executionID := constants.ExtractExecutionIDFromLogStream(data.LogStream)
	if executionID == "" {
		reqLogger.Warn("unable to extract execution ID from log stream",
			"context", map[string]string{
				"log_stream": data.LogStream,
			},
		)
		return true, nil
	}

	reqLogger.Debug("processing CloudWatch logs event",
		"context", map[string]any{
			"log_group":    data.LogGroup,
			"log_stream":   data.LogStream,
			"execution_id": executionID,
			"log_count":    len(data.LogEvents),
		},
	)

	// Convert CloudWatch log events to api.LogEvent format
	logEvents := make([]api.LogEvent, 0, len(data.LogEvents))
	for _, cwLogEvent := range data.LogEvents {
		logEvents = append(logEvents, api.LogEvent{
			Timestamp: cwLogEvent.Timestamp,
			Message:   cwLogEvent.Message,
		})
	}

	sendErr := p.webSocketManager.SendLogsToExecution(ctx, &executionID, logEvents)
	if sendErr != nil {
		reqLogger.Error("failed to send logs to WebSocket connections",
			"error", sendErr,
			"execution_id", executionID,
		)
		// Don't return error - logs were processed correctly, connection issue shouldn't fail processing
	}

	return true, nil
}
