// Package websocket provides WebSocket log forwarding for runvoy.
// It handles CloudWatch Logs subscription filter events and forwards them to connected WebSocket clients.
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"runvoy/internal/api"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	dynamoRepo "runvoy/internal/database/dynamodb"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// LogForwarder handles CloudWatch Logs events and forwards them to WebSocket clients.
type LogForwarder struct {
	connRepo      database.ConnectionRepository
	apiGwClient   *apigatewaymanagementapi.Client
	apiGwEndpoint *string
	logger        *slog.Logger
}

// NewLogForwarder creates a new log forwarder with AWS backend.
func NewLogForwarder(ctx context.Context, cfg *config.Config, log *slog.Logger) (*LogForwarder, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	connRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.WebSocketConnectionsTable, log)

	apiGwClient := apigatewaymanagementapi.NewFromConfig(awsCfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(cfg.WebSocketAPIEndpoint)
	})

	log.Info("log forwarder initialized",
		"context", map[string]string{
			"table":        cfg.WebSocketConnectionsTable,
			"api_endpoint": cfg.WebSocketAPIEndpoint,
		},
	)

	return &LogForwarder{
		connRepo:      connRepo,
		apiGwClient:   apiGwClient,
		apiGwEndpoint: aws.String(cfg.WebSocketAPIEndpoint),
		logger:        log,
	}, nil
}

// HandleLogs is the main entry point for Lambda CloudWatch Logs event processing.
func (lf *LogForwarder) HandleLogs(ctx context.Context, event events.CloudwatchLogsEvent) error {
	reqLogger := logger.DeriveRequestLogger(ctx, lf.logger)

	logsData, err := lf.decodeLogsEvent(event, reqLogger)
	if err != nil {
		return err
	}

	executionID := constants.ExtractExecutionIDFromLogStream(logsData.LogStream)
	if executionID == "" {
		reqLogger.Debug("could not extract execution_id from log stream, skipping",
			"context", map[string]string{
				"log_stream": logsData.LogStream,
			},
		)
		return nil
	}

	return lf.forwardLogsToConnections(ctx, executionID, logsData.LogEvents, reqLogger)
}

// decodeLogsEvent decodes and unmarshals the CloudWatch Logs event data.
func (lf *LogForwarder) decodeLogsEvent(
	event events.CloudwatchLogsEvent,
	reqLogger *slog.Logger,
) (events.CloudwatchLogsData, error) {
	logsData, err := event.AWSLogs.Parse()
	if err != nil {
		reqLogger.Error("failed to parse CloudWatch Logs data", "error", err)
		return events.CloudwatchLogsData{}, fmt.Errorf("failed to parse CloudWatch Logs data: %w", err)
	}

	reqLogger.Debug("processing CloudWatch Logs event",
		"context", map[string]string{
			"log_group":    logsData.LogGroup,
			"log_stream":   logsData.LogStream,
			"message_type": logsData.MessageType,
			"owner":        logsData.Owner,
		},
	)

	return logsData, nil
}

// forwardLogsToConnections forwards log events to all active WebSocket connections for an execution.
func (lf *LogForwarder) forwardLogsToConnections(
	ctx context.Context,
	executionID string,
	logEvents []events.CloudwatchLogsLogEvent,
	reqLogger *slog.Logger,
) error {
	reqLogger.Debug("extracted execution_id", "context", map[string]string{
		"execution_id": executionID,
	})

	connectionIDs, queryErr := lf.connRepo.GetConnectionsByExecutionID(ctx, executionID)
	if queryErr != nil {
		reqLogger.Error("failed to get connections for execution", "context", map[string]string{
			"execution_id": executionID,
			"error":        queryErr.Error(),
		})
		return fmt.Errorf("failed to get connections for execution: %w", queryErr)
	}

	if len(connectionIDs) == 0 {
		reqLogger.Debug("no active connections found for execution", "context", map[string]string{
			"execution_id": executionID,
		})
		return nil
	}

	reqLogger.Debug("found active connections", "context", map[string]string{
		"execution_id": executionID,
		"count":        fmt.Sprintf("%d", len(connectionIDs)),
	})

	for _, logEvent := range logEvents {
		for _, connectionID := range connectionIDs {
			if sendErr := lf.sendToConnection(ctx, connectionID, logEvent); sendErr != nil {
				reqLogger.Error("failed to send log to connection", "context", map[string]string{
					"error":         sendErr.Error(),
					"connection_id": connectionID,
					"execution_id":  executionID,
				})
				// Continue with other connections even if one fails
			}
		}
	}

	reqLogger.Info("all log events forwarded to connections", "context", map[string]any{
		"execution_id": executionID,
		"events_count": len(logEvents),
		"connections":  connectionIDs,
	})

	return nil
}

// sendToConnection sends a message to a WebSocket connection via API Gateway Management API.
func (lf *LogForwarder) sendToConnection(
	ctx context.Context, connectionID string, logEvent events.CloudwatchLogsLogEvent) error {
	reqLogger := logger.DeriveRequestLogger(ctx, lf.logger)

	reqLogger.Debug("sending message to connection",
		"context", map[string]string{
			"connection_id":  connectionID,
			"message_length": fmt.Sprintf("%d", len(logEvent.Message)),
		},
	)

	jsonEventData, err := json.Marshal(api.LogEvent{
		Timestamp: logEvent.Timestamp,
		Message:   logEvent.Message,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal log event: %w", err)
	}

	_, err = lf.apiGwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: &connectionID,
		Data:         jsonEventData,
	})

	if err != nil {
		reqLogger.Error("failed to post to connection", "context", map[string]string{
			"error":         err.Error(),
			"connection_id": connectionID,
		})
		return fmt.Errorf("failed to post to connection %s: %w", connectionID, err)
	}

	reqLogger.Debug("message forwarded to connection", "context", map[string]string{
		"connection_id": connectionID,
	})

	return nil
}
