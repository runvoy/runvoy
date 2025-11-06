// Package websocket provides WebSocket log forwarding for runvoy.
// It handles CloudWatch Logs subscription filter events and forwards them to connected WebSocket clients.
package websocket

import (
	"context"
	"encoding/json"
	"errors"
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
	"golang.org/x/sync/errgroup"
)

// LogForwarder handles CloudWatch Logs events and forwards them to WebSocket clients.
type LogForwarder struct {
	connRepo      database.ConnectionRepository
	logRepo       database.LogRepository
	executionRepo database.ExecutionRepository
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
	logRepo := dynamoRepo.NewLogRepository(dynamoClient, cfg.ExecutionLogsTable, log)
	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.ExecutionsTable, log)

	apiGwClient := apigatewaymanagementapi.NewFromConfig(awsCfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(cfg.WebSocketAPIEndpoint)
	})

	log.Info("log forwarder initialized",
		"context", map[string]string{
			"connections_table": cfg.WebSocketConnectionsTable,
			"logs_table":        cfg.ExecutionLogsTable,
			"executions_table":  cfg.ExecutionsTable,
			"api_endpoint":      cfg.WebSocketAPIEndpoint,
		},
	)

	return &LogForwarder{
		connRepo:      connRepo,
		logRepo:       logRepo,
		executionRepo: executionRepo,
		apiGwClient:   apiGwClient,
		apiGwEndpoint: aws.String(cfg.WebSocketAPIEndpoint),
		logger:        log,
	}, nil
}

// Handle is the main entry point for Lambda event processing.
// It processes CloudWatch Logs subscription filter events.
func (lf *LogForwarder) Handle(ctx context.Context, event events.CloudwatchLogsEvent) error {
	reqLogger := logger.DeriveRequestLogger(ctx, lf.logger)
	return lf.handleLogs(ctx, event, reqLogger)
}

// handleLogs processes CloudWatch Logs events and forwards them to connected clients.
func (lf *LogForwarder) handleLogs(
	ctx context.Context,
	event events.CloudwatchLogsEvent,
	reqLogger *slog.Logger,
) error {
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

	// Check execution status - only process RUNNING executions
	if !lf.shouldProcessExecution(ctx, executionID, reqLogger) {
		return nil
	}

	// Convert and store logs in DynamoDB
	apiLogEvents := lf.convertToAPILogEvents(logsData.LogEvents)
	maxIndex, storeErr := lf.logRepo.StoreLogs(ctx, executionID, apiLogEvents)
	if storeErr != nil {
		reqLogger.Error("failed to store logs in DynamoDB",
			"context", map[string]any{
				"execution_id": executionID,
				"error":        storeErr.Error(),
				"events_count": len(apiLogEvents),
			},
		)
		return fmt.Errorf("failed to store logs: %w", storeErr)
	}

	reqLogger.Debug("logs stored in DynamoDB",
		"context", map[string]any{
			"execution_id": executionID,
			"events_count": len(apiLogEvents),
			"max_index":    maxIndex,
		},
	)

	return lf.forwardLogsToConnections(ctx, executionID, reqLogger)
}

// shouldProcessExecution checks if the execution is RUNNING and should be processed.
// Returns false if execution is not found or not RUNNING.
func (lf *LogForwarder) shouldProcessExecution(
	ctx context.Context,
	executionID string,
	reqLogger *slog.Logger,
) bool {
	execution, err := lf.executionRepo.GetExecution(ctx, executionID)
	if err != nil {
		reqLogger.Warn("failed to get execution status, skipping log processing",
			"context", map[string]string{
				"execution_id": executionID,
				"error":        err.Error(),
			},
		)
		return false
	}

	if execution.Status != string(constants.ExecutionRunning) {
		reqLogger.Debug("execution is not RUNNING, skipping log processing",
			"context", map[string]string{
				"execution_id": executionID,
				"status":       execution.Status,
			},
		)
		return false
	}

	return true
}

// convertToAPILogEvents converts CloudWatch log events to API log events.
func (lf *LogForwarder) convertToAPILogEvents(logEvents []events.CloudwatchLogsLogEvent) []api.LogEvent {
	apiLogEvents := make([]api.LogEvent, len(logEvents))
	for i, logEvent := range logEvents {
		apiLogEvents[i] = api.LogEvent{
			Timestamp: logEvent.Timestamp,
			Message:   logEvent.Message,
		}
	}
	return apiLogEvents
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

// forwardLogsToConnections forwards log events from DynamoDB to all active WebSocket connections for an execution.
// For each connection, it queries DynamoDB for logs after the connection's last_index and forwards them.
func (lf *LogForwarder) forwardLogsToConnections(
	ctx context.Context,
	executionID string,
	reqLogger *slog.Logger,
) error {
	connections, queryErr := lf.connRepo.GetConnectionsWithMetadataByExecutionID(ctx, executionID)
	if queryErr != nil {
		reqLogger.Error("failed to get connections for execution", "context", map[string]string{
			"execution_id": executionID,
			"error":        queryErr.Error(),
		})
		return fmt.Errorf("failed to get connections for execution: %w", queryErr)
	}

	if len(connections) == 0 {
		reqLogger.Debug("no active connections found for execution", "context", map[string]string{
			"execution_id": executionID,
		})
		return nil
	}

	reqLogger.Debug("forwarding logs to connections", "context", map[string]any{
		"execution_id":      executionID,
		"connections_count": len(connections),
	})

	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.SetLimit(constants.MaxConcurrentSends)

	for _, conn := range connections {
		connection := conn // Capture for closure
		errGroup.Go(func() error {
			if forwardErr := lf.forwardLogsToConnection(
				ctx, connection, executionID, reqLogger,
			); forwardErr != nil {
				reqLogger.Error("failed to forward logs to connection", "context", map[string]any{
					"error":         forwardErr.Error(),
					"connection_id": connection.ConnectionID,
					"execution_id":  executionID,
				})
				return forwardErr
			}
			return nil
		})
	}

	switch err := errGroup.Wait(); {
	case err != nil:
		return errors.New("error(s) occurred while forwarding log events to connections")
	default:
		connectionIDs := make([]string, len(connections))
		for i, conn := range connections {
			connectionIDs[i] = conn.ConnectionID
		}
		reqLogger.Info("all log events forwarded to connections", "context", map[string]any{
			"execution_id":      executionID,
			"connections_count": len(connections),
			"connections":       connectionIDs,
		})

		return nil
	}
}

// forwardLogsToConnection forwards logs from DynamoDB to a single WebSocket connection.
// It queries DynamoDB for logs after the connection's last_index.
func (lf *LogForwarder) forwardLogsToConnection(
	ctx context.Context,
	connection *api.WebSocketConnection,
	executionID string,
	reqLogger *slog.Logger,
) error {
	lastIndex := connection.LastIndex

	// Query DynamoDB for logs after last_index
	logEvents, err := lf.logRepo.GetLogsSinceIndex(ctx, executionID, lastIndex)
	if err != nil {
		return fmt.Errorf("failed to query logs from DynamoDB: %w", err)
	}

	if len(logEvents) == 0 {
		reqLogger.Debug("no logs to forward for connection", "context", map[string]any{
			"connection_id": connection.ConnectionID,
			"execution_id":  executionID,
			"last_index":    lastIndex,
		})
		return nil
	}

	reqLogger.Debug("forwarding logs to connection", "context", map[string]any{
		"connection_id": connection.ConnectionID,
		"execution_id":  executionID,
		"logs_count":    len(logEvents),
		"last_index":    lastIndex,
	})

	return lf.sendLogEventsToConnection(ctx, connection.ConnectionID, logEvents)
}

// sendLogEventsToConnection sends a batch of log events to a WebSocket connection via API Gateway Management API.
// The events are sent as newline-delimited JSON (NDJSON) format.
func (lf *LogForwarder) sendLogEventsToConnection(
	ctx context.Context,
	connectionID string,
	logEvents []api.LogEvent,
) error {
	reqLogger := logger.DeriveRequestLogger(ctx, lf.logger)

	if len(logEvents) == 0 {
		return nil
	}

	var batchData []byte
	for i, logEvent := range logEvents {
		jsonEventData, err := json.Marshal(logEvent)
		if err != nil {
			return fmt.Errorf("failed to marshal log event at index %d: %w", i, err)
		}

		if i > 0 {
			batchData = append(batchData, '\n')
		}
		batchData = append(batchData, jsonEventData...)
	}

	_, err := lf.apiGwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: &connectionID,
		Data:         batchData,
	})

	if err != nil {
		reqLogger.Error("failed to post batch to connection", "context", map[string]any{
			"error":         err.Error(),
			"connection_id": connectionID,
			"events_count":  len(logEvents),
		})
		return fmt.Errorf("failed to post batch to connection %s: %w", connectionID, err)
	}

	return nil
}
