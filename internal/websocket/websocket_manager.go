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
}

// NewWebSocketManager creates a new WebSocket manager with AWS backend.
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

	log.Debug("websocket manager initialized",
		"table", cfg.WebSocketConnectionsTable,
		"api_endpoint", cfg.WebSocketAPIEndpoint,
	)

	return &WebSocketManager{
		connRepo:      connRepo,
		apiGwClient:   apiGwClient,
		apiGwEndpoint: aws.String(cfg.WebSocketAPIEndpoint),
		logger:        log,
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
	reqLogger := logger.DeriveRequestLogger(ctx, wm.logger)

	reqLogger.Debug("received WebSocket event", "context", map[string]string{
		"route_key":     req.RequestContext.RouteKey,
		"connection_id": req.RequestContext.ConnectionID,
	})

	switch req.RequestContext.RouteKey {
	case "$connect":
		return wm.handleConnect(ctx, req, reqLogger)
	case "$disconnect":
		return wm.handleDisconnect(ctx, req, reqLogger)
	case "$disconnect-execution":
		// Handle disconnect notification from event processor
		// ConnectionID contains the executionID in this case
		err := wm.handleDisconnectNotification(ctx, req.RequestContext.ConnectionID, reqLogger)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       fmt.Sprintf("Failed to notify disconnect: %v", err),
			}, nil
		}
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Body:       "Disconnect notifications sent",
		}, nil
	default:
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       fmt.Sprintf("Unknown route: %s", req.RequestContext.RouteKey),
		}, nil
	}
}

//nolint:gocritic // Lambda event types are passed by value per AWS Lambda conventions
func (wm *WebSocketManager) handleConnect(
	ctx context.Context,
	req events.APIGatewayWebsocketProxyRequest,
	reqLogger *slog.Logger,
) (events.APIGatewayProxyResponse, error) {
	connectionID := req.RequestContext.ConnectionID
	executionID := req.QueryStringParameters["execution_id"]

	if executionID == "" {
		reqLogger.Info("missing execution_id in connection request")
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

	reqLogger.Debug("storing connection", "context", map[string]string{
		"connection_id": connection.ConnectionID,
		"execution_id":  connection.ExecutionID,
		"functionality": connection.Functionality,
		"expires_at":    fmt.Sprintf("%d", connection.ExpiresAt),
	})

	err := wm.connRepo.CreateConnection(ctx, connection)
	if err != nil {
		reqLogger.Error("failed to store connection", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to store connection: %v", err),
		}, nil
	}

	reqLogger.Info("connection established", "context", map[string]string{
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

//nolint:gocritic // Lambda event types are passed by value per AWS Lambda conventions
func (wm *WebSocketManager) handleDisconnect(
	ctx context.Context,
	req events.APIGatewayWebsocketProxyRequest,
	reqLogger *slog.Logger,
) (events.APIGatewayProxyResponse, error) {
	connectionID := req.RequestContext.ConnectionID

	reqLogger.Debug("deleting connection", "context", map[string]string{
		"connection_id": connectionID,
	})

	err := wm.connRepo.DeleteConnection(ctx, connectionID)
	if err != nil {
		reqLogger.Error("failed to delete connection", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to delete connection: %v", err),
		}, nil
	}

	reqLogger.Info("connection disconnected", "context", map[string]string{
		"connection_id": connectionID,
	})

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "Disconnected",
	}, nil
}

// handleDisconnectNotification sends disconnect messages to all connected clients for an execution.
// This notifies clients that the execution has completed.
func (wm *WebSocketManager) handleDisconnectNotification(
	ctx context.Context,
	executionID string,
	reqLogger *slog.Logger,
) error {
	reqLogger.Debug("handling disconnect notification for execution", "execution_id", executionID)

	// Get all connection IDs for this execution
	connectionIDs, err := wm.connRepo.GetConnectionsByExecutionID(ctx, executionID)
	if err != nil {
		reqLogger.Error("failed to get connections for execution", "error", err, "execution_id", executionID)
		return fmt.Errorf("failed to get connections: %w", err)
	}

	if len(connectionIDs) == 0 {
		reqLogger.Debug("no active connections to notify", "execution_id", executionID)
		return nil
	}

	reqLogger.Debug("sending disconnect notifications to connections",
		"execution_id", executionID,
		"connection_count", len(connectionIDs),
	)

	// Send disconnect message to all connections concurrently
	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.SetLimit(constants.MaxConcurrentSends)

	disconnectMessage := []byte(`{"type":"disconnect","reason":"execution_completed"}`)

	for _, connectionID := range connectionIDs {
		connID := connectionID // Capture for closure
		errGroup.Go(func() error {
			return wm.sendDisconnectToConnection(ctx, connID, disconnectMessage, reqLogger)
		})
	}

	err = errGroup.Wait()
	if err != nil {
		reqLogger.Error("some disconnect notifications failed to send", "error", err, "execution_id", executionID)
		// Don't fail the handler - best effort delivery
	} else {
		reqLogger.Info("all disconnect notifications sent successfully",
			"execution_id", executionID,
			"connection_count", len(connectionIDs),
		)
	}

	return nil
}

// sendDisconnectToConnection sends a disconnect message to a single WebSocket connection.
func (wm *WebSocketManager) sendDisconnectToConnection(
	ctx context.Context,
	connectionID string,
	message []byte,
	reqLogger *slog.Logger,
) error {
	reqLogger.Debug("sending disconnect notification to connection", "connection_id", connectionID)

	_, err := wm.apiGwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         message,
	})

	if err != nil {
		reqLogger.Error("failed to send disconnect notification to connection",
			"error", err,
			"connection_id", connectionID,
		)
		return fmt.Errorf("failed to send disconnect notification to connection %s: %w", connectionID, err)
	}

	reqLogger.Debug("disconnect notification sent to connection", "connection_id", connectionID)
	return nil
}
