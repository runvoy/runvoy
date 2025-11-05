// Package websocket provides WebSocket management for runvoy.
// It handles connection lifecycle events and manages WebSocket connections in DynamoDB.
package websocket

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

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

// WebSocketManager handles WebSocket connection lifecycle events and disconnect notifications.
//
//nolint:revive // exported type name is intentional for clarity
type WebSocketManager struct {
	connRepo      database.ConnectionRepository
	apiGwClient   *apigatewaymanagementapi.Client
	apiGwEndpoint *string
	logger        *slog.Logger
	connectionIDs []string
}

// NewWebSocketManager creates a new WebSocket manager.
func NewWebSocketManager(ctx context.Context, cfg *config.Config, log *slog.Logger) (*WebSocketManager, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	connRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.WebSocketConnectionsTable, log)
	apiGwClient := apigatewaymanagementapi.NewFromConfig(awsCfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(cfg.WebSocketAPIEndpoint)
	})
	reqLogger := logger.DeriveRequestLogger(ctx, log)
	connectionIDs := make([]string, 0)

	reqLogger.Debug("websocket manager initialized",
		"table", cfg.WebSocketConnectionsTable,
		"api_endpoint", cfg.WebSocketAPIEndpoint,
	)

	return &WebSocketManager{
		connRepo:      connRepo,
		apiGwClient:   apiGwClient,
		apiGwEndpoint: aws.String(cfg.WebSocketAPIEndpoint),
		logger:        reqLogger,
		connectionIDs: connectionIDs,
	}, nil
}

// HandleRequest is the main entry point for Lambda event processing.
// It routes API Gateway WebSocket events based on their route key:
// - $connect: stores WebSocket connection in DynamoDB
// - $disconnect: removes WebSocket connection from DynamoDB
// - $disconnect-execution: sends disconnect notifications to all clients for an execution
//
//nolint:gocritic // Lambda event types are passed by value per AWS Lambda conventions
func (wm *WebSocketManager) HandleRequest(
	ctx context.Context,
	req events.APIGatewayWebsocketProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	wm.logger.Debug("received WebSocket event", "context", map[string]string{
		"route_key":     req.RequestContext.RouteKey,
		"connection_id": req.RequestContext.ConnectionID,
	})

	switch req.RequestContext.RouteKey {
	case "$connect":
		return wm.handleConnect(ctx, req)
	case "$disconnect":
		return wm.handleDisconnect(ctx, req)
	case "$disconnect-execution":
		return wm.handleDisconnectExecution(ctx, req)
	default:
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       fmt.Sprintf("Unknown route: %s", req.RequestContext.RouteKey),
		}, nil
	}
}

// handleConnect handles the $connect route key.
// It stores the WebSocket connection in DynamoDB and returns a success response.
//
//nolint:gocritic // Lambda event types are passed by value per AWS Lambda conventions
func (wm *WebSocketManager) handleConnect(
	ctx context.Context,
	req events.APIGatewayWebsocketProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	connectionID := req.RequestContext.ConnectionID
	executionID := req.QueryStringParameters["execution_id"]

	if connectionID == "" {
		wm.logger.Info("missing connection_id in connection request")
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Missing connection_id",
		}, nil
	}

	if executionID == "" {
		wm.logger.Info("missing execution_id in connection request")
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Missing execution_id query parameter",
		}, nil
	}

	expiresAt := time.Now().Add(constants.ConnectionTTLHours * time.Hour).Unix()

	connection := &api.WebSocketConnection{
		ConnectionID:  connectionID,
		ExecutionID:   executionID,
		Functionality: constants.FunctionalityLogStreaming,
		ExpiresAt:     expiresAt,
	}

	wm.logger.Debug("storing connection", "context", map[string]string{
		"connection_id": connection.ConnectionID,
		"execution_id":  connection.ExecutionID,
		"functionality": connection.Functionality,
		"expires_at":    fmt.Sprintf("%d", connection.ExpiresAt),
	})

	err := wm.connRepo.CreateConnection(ctx, connection)
	if err != nil {
		wm.logger.Error("failed to store connection", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to store connection: %v", err),
		}, nil
	}

	wm.logger.Info("connection established", "context", map[string]string{
		"connection_id": connection.ConnectionID,
		"execution_id":  connection.ExecutionID,
		"functionality": connection.Functionality,
		"expires_at":    fmt.Sprintf("%d", connection.ExpiresAt),
	})

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "Connected",
	}, nil
}

// handleDisconnect handles the $disconnect route key.
// It deletes the WebSocket connection from DynamoDB and returns a success response.
//
//nolint:gocritic // Lambda event types are passed by value per AWS Lambda conventions
func (wm *WebSocketManager) handleDisconnect(
	ctx context.Context,
	req events.APIGatewayWebsocketProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	connectionID := req.RequestContext.ConnectionID

	if connectionID == "" {
		wm.logger.Info("missing connection_id in disconnect request")
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Missing connection_id",
		}, nil
	}

	wm.logger.Debug("deleting connection", "context", map[string]string{
		"connection_id": connectionID,
	})

	deletedCount, err := wm.connRepo.DeleteConnections(ctx, []string{connectionID})
	if err != nil {
		wm.logger.Error("failed to delete connection", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to delete connection: %v", err),
		}, nil
	}

	wm.logger.Info("connection disconnected", "context", map[string]any{
		"connection_id": connectionID,
		"deleted_count": deletedCount,
	})

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "Disconnected",
	}, nil
}

// handleDisconnectExecution handles the $disconnect-execution route key.
// It sends disconnect notifications to all connected clients for an execution
// and deletes the connections from DynamoDB.
//
//nolint:gocritic // Lambda event types are passed by value per AWS Lambda conventions
func (wm *WebSocketManager) handleDisconnectExecution(
	ctx context.Context,
	req events.APIGatewayWebsocketProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	executionID := req.QueryStringParameters["execution_id"]

	if executionID == "" {
		wm.logger.Info("missing execution_id in disconnect execution request")
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Missing execution_id query parameter",
		}, nil
	}

	// First send disconnect notifications to clients
	err := wm.handleDisconnectNotification(ctx, executionID)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to notify disconnect: %v", err),
		}, nil
	}

	// Then delete all connection records for this execution
	deletedCount, err := wm.connRepo.DeleteConnections(ctx, wm.connectionIDs)
	if err != nil {
		wm.logger.Warn("failed to delete WebSocket connections", "context",
			map[string]string{
				"error":        err.Error(),
				"execution_id": executionID,
			},
		)
		// Don't fail the response - notifications were sent
	} else if deletedCount > 0 {
		wm.logger.Debug("deleted WebSocket connections for execution", "context",
			map[string]string{
				"execution_id":  executionID,
				"deleted_count": fmt.Sprintf("%d", deletedCount),
			},
		)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "Disconnect notifications sent and connections cleaned up",
	}, nil
}

// handleDisconnectNotification sends disconnect messages to all connected clients for an execution.
// This notifies clients that the execution has completed.
func (wm *WebSocketManager) handleDisconnectNotification(
	ctx context.Context,
	executionID string,
) error {
	var err error
	wm.logger.Debug("handling disconnect notification for execution", "execution_id", executionID)

	// Get all connection IDs for this execution
	wm.connectionIDs, err = wm.connRepo.GetConnectionsByExecutionID(ctx, executionID)
	if err != nil {
		wm.logger.Error("failed to get connections for execution", "error", err, "execution_id", executionID)
		return fmt.Errorf("failed to get connections: %w", err)
	}

	if len(wm.connectionIDs) == 0 {
		wm.logger.Debug("no active connections to notify", "execution_id", executionID)
		return nil
	}

	wm.logger.Debug("sending disconnect notifications to connections", "context", map[string]any{
		"execution_id":   executionID,
		"connection_ids": wm.connectionIDs,
	})

	// Send disconnect message to all connections concurrently
	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.SetLimit(constants.MaxConcurrentSends)

	disconnectMessage := []byte(`{"type":"disconnect","reason":"execution_completed"}`)

	for _, connectionID := range wm.connectionIDs {
		connID := connectionID // Capture for closure
		errGroup.Go(func() error {
			return wm.sendDisconnectToConnection(ctx, connID, disconnectMessage)
		})
	}

	err = errGroup.Wait()
	if err != nil {
		wm.logger.Error("some disconnect notifications failed to send", "context", map[string]string{
			"error":        err.Error(),
			"execution_id": executionID,
		})
	} else {
		wm.logger.Info("all disconnect notifications sent successfully",
			"context", map[string]string{
				"execution_id":     executionID,
				"connection_count": fmt.Sprintf("%d", len(wm.connectionIDs)),
			},
		)
	}

	return nil
}

// sendDisconnectToConnection sends a disconnect message to a single WebSocket connection.
func (wm *WebSocketManager) sendDisconnectToConnection(
	ctx context.Context,
	connectionID string,
	message []byte,
) error {
	wm.logger.Debug("sending disconnect notification to connection", "context", map[string]string{
		"connection_id": connectionID,
	})

	_, err := wm.apiGwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         message,
	})

	if err != nil {
		wm.logger.Error("failed to send disconnect notification to connection",
			"context", map[string]string{
				"error":         err.Error(),
				"connection_id": connectionID,
			},
		)
		return fmt.Errorf("failed to send disconnect notification to connection %s: %w", connectionID, err)
	}

	wm.logger.Debug("disconnect notification sent to connection", "context", map[string]string{
		"connection_id": connectionID,
	})
	return nil
}
