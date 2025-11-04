// Package websocket provides WebSocket connection management for runvoy.
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
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// ConnectionManager handles WebSocket connection lifecycle events.
type ConnectionManager struct {
	connRepo database.ConnectionRepository
	logger   *slog.Logger
}

// NewConnectionManager creates a new connection manager with AWS backend.
func NewConnectionManager(ctx context.Context, cfg *config.Config, log *slog.Logger) (*ConnectionManager, error) {
	if cfg.WebSocketConnectionsTable == "" {
		return nil, fmt.Errorf("WebSocketConnectionsTable cannot be empty")
	}

	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	connRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.WebSocketConnectionsTable, log)

	log.Debug("connection manager initialized", "table", cfg.WebSocketConnectionsTable)

	return &ConnectionManager{
		connRepo: connRepo,
		logger:   log,
	}, nil
}

// HandleRequest is the main entry point for Lambda WebSocket event processing.
// It routes events based on their route key ($connect or $disconnect).
//
//nolint:gocritic // Lambda event types are passed by value per AWS Lambda conventions
func (cm *ConnectionManager) HandleRequest(
	ctx context.Context,
	req events.APIGatewayWebsocketProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, cm.logger)

	reqLogger.Debug("received WebSocket event", "context", map[string]string{
		"route_key":     req.RequestContext.RouteKey,
		"connection_id": req.RequestContext.ConnectionID,
	})

	switch req.RequestContext.RouteKey {
	case "$connect":
		return cm.handleConnect(ctx, req, reqLogger)
	case "$disconnect":
		return cm.handleDisconnect(ctx, req, reqLogger)
	default:
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       fmt.Sprintf("Unknown route: %s", req.RequestContext.RouteKey),
		}, nil
	}
}

//nolint:gocritic // Lambda event types are passed by value per AWS Lambda conventions
func (cm *ConnectionManager) handleConnect(
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

	err := cm.connRepo.CreateConnection(ctx, connection)
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
func (cm *ConnectionManager) handleDisconnect(
	ctx context.Context,
	req events.APIGatewayWebsocketProxyRequest,
	reqLogger *slog.Logger,
) (events.APIGatewayProxyResponse, error) {
	connectionID := req.RequestContext.ConnectionID

	reqLogger.Debug("deleting connection", "context", map[string]string{
		"connection_id": connectionID,
	})

	err := cm.connRepo.DeleteConnection(ctx, connectionID)
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
