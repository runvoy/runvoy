// Package websocket provides WebSocket management for runvoy.
// It handles connection lifecycle events and manages WebSocket connections in DynamoDB.
package websocket

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
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"golang.org/x/sync/errgroup"
)

// Manager exposes the subset of WebSocket manager functionality used by the event processor.
type Manager interface {
	HandleRequest(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error)
	NotifyExecutionCompletion(ctx context.Context, executionID *string) error
	SendLogsToExecution(ctx context.Context, executionID *string, logEvents []api.LogEvent) error
}

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
func NewWebSocketManager(
	cfg *config.Config,
	awsCfg *aws.Config,
	connRepo database.ConnectionRepository,
	log *slog.Logger,
) *WebSocketManager {
	apiGwClient := apigatewaymanagementapi.NewFromConfig(*awsCfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(cfg.WebSocketAPIEndpoint)
	})
	connectionIDs := make([]string, 0)

	log.Debug("websocket manager initialized",
		"table", cfg.WebSocketConnectionsTable,
		"websocket_api_endpoint", cfg.WebSocketAPIEndpoint,
	)

	return &WebSocketManager{
		connRepo:      connRepo,
		apiGwClient:   apiGwClient,
		apiGwEndpoint: aws.String(cfg.WebSocketAPIEndpoint),
		logger:        log,
		connectionIDs: connectionIDs,
	}
}

// getClientIPFromWebSocketRequest extracts the client IP address from a WebSocket proxy request.
// It checks the RequestContext.Identity.SourceIP field which contains the client's IP address.
func getClientIPFromWebSocketRequest(req *events.APIGatewayWebsocketProxyRequest) string {
	if req.RequestContext.Identity.SourceIP != "" {
		return req.RequestContext.Identity.SourceIP
	}
	return ""
}

// HandleRequest adapts WebSocket events so the generic event processor can route them.
// It attempts to unmarshal the raw Lambda event as an API Gateway WebSocket request and,
// when successful, dispatches based on the route key.
func (wm *WebSocketManager) HandleRequest(
	ctx context.Context,
	rawEvent *json.RawMessage,
	reqLogger *slog.Logger,
) (bool, error) {
	if rawEvent == nil {
		return false, nil
	}

	var req events.APIGatewayWebsocketProxyRequest
	if err := json.Unmarshal(*rawEvent, &req); err != nil {
		return false, nil
	}

	handled := true

	if req.RequestContext.RouteKey == "" {
		reqLogger.Error("missing route key in WebSocket request")
		return handled, fmt.Errorf("missing route key in WebSocket request")
	}

	clientIP := getClientIPFromWebSocketRequest(&req)
	logArgs := []any{
		"route_key", req.RequestContext.RouteKey,
	}
	if req.RequestContext.ConnectionID != "" {
		logArgs = append(logArgs, "connection_id", req.RequestContext.ConnectionID)
	}
	if clientIP != "" {
		logArgs = append(logArgs, "client_ip", clientIP)
	}
	reqLogger.Debug("received WebSocket event", "context", logger.SliceToMap(logArgs))

	resp, err := wm.dispatchWebSocketRoute(ctx, reqLogger, &req)
	if err != nil {
		return handled, err
	}

	if webSocketErr := wm.evaluateRouteResponse(reqLogger, req.RequestContext.RouteKey, resp); webSocketErr != nil {
		return handled, webSocketErr
	}

	return handled, nil
}

func (wm *WebSocketManager) dispatchWebSocketRoute(
	ctx context.Context,
	reqLogger *slog.Logger,
	req *events.APIGatewayWebsocketProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	routeHandlers := map[string]func(
		context.Context, events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error){
		"$connect":    wm.handleConnect,
		"$disconnect": wm.handleDisconnect,
	}

	handler, ok := routeHandlers[req.RequestContext.RouteKey]
	if !ok {
		reqLogger.Error("unrecognized WebSocket route", "context", map[string]string{
			"route_key": req.RequestContext.RouteKey,
		})
		return events.APIGatewayProxyResponse{}, fmt.Errorf("unrecognized WebSocket route: %s", req.RequestContext.RouteKey)
	}

	resp, err := handler(ctx, *req)
	if err != nil {
		reqLogger.Error("websocket route failed", "context", map[string]any{
			"route_key":   req.RequestContext.RouteKey,
			"error":       err.Error(),
			"status_code": resp.StatusCode,
			"body":        resp.Body,
		})
		return events.APIGatewayProxyResponse{}, fmt.Errorf("websocket route %s failed: %w", req.RequestContext.RouteKey, err)
	}

	return resp, nil
}

func (wm *WebSocketManager) evaluateRouteResponse(
	reqLogger *slog.Logger,
	routeKey string,
	resp events.APIGatewayProxyResponse,
) error {
	switch {
	case resp.StatusCode >= http.StatusInternalServerError:
		return fmt.Errorf(
			"websocket route %s returned %d: %s",
			routeKey,
			resp.StatusCode,
			resp.Body,
		)
	case resp.StatusCode >= http.StatusBadRequest:
		reqLogger.Error("websocket handler returned client error response", "context", map[string]any{
			"route_key":   routeKey,
			"status_code": resp.StatusCode,
			"body":        resp.Body,
		})
	default:
		reqLogger.Debug("websocket handler completed", "context", map[string]any{
			"route_key":   routeKey,
			"status_code": resp.StatusCode,
		})
	}

	return nil
}

// handleConnect handles the $connect route key.
// It validates the WebSocket token, stores the connection in DynamoDB, and returns a success response.
//
//nolint:gocritic // Lambda event types are passed by value per AWS Lambda conventions
func (wm *WebSocketManager) handleConnect(
	ctx context.Context,
	req events.APIGatewayWebsocketProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	connectionID := req.RequestContext.ConnectionID
	executionID := req.QueryStringParameters["execution_id"]
	token := req.QueryStringParameters["token"]

	if errResp := wm.validateConnectionParams(connectionID, executionID, token); errResp != nil {
		return *errResp, nil
	}

	pendingConnection, errResp := wm.validateAndConsumePendingToken(ctx, executionID, token)
	if errResp != nil {
		return *errResp, nil
	}

	connection := &api.WebSocketConnection{
		ConnectionID:       connectionID,
		ExecutionID:        executionID,
		Functionality:      constants.FunctionalityLogStreaming,
		ExpiresAt:          pendingConnection.ExpiresAt,
		ClientIP:           getClientIPFromWebSocketRequest(&req),
		UserEmail:          pendingConnection.UserEmail,
		ClientIPAtLogsTime: pendingConnection.ClientIPAtLogsTime,
	}

	if err := wm.connRepo.CreateConnection(ctx, connection); err != nil {
		wm.logger.Error("failed to store connection", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to store connection: %v", err),
		}, nil
	}

	wm.logger.Info("authenticated connection established", "context", map[string]any{
		"connection_id":     connection.ConnectionID,
		"execution_id":      connection.ExecutionID,
		"functionality":     connection.Functionality,
		"expires_at":        connection.ExpiresAt,
		"client_ip":         connection.ClientIP,
		"user_email":        connection.UserEmail,
		"client_ip_at_logs": connection.ClientIPAtLogsTime,
	})

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "Connected",
	}, nil
}

// validateConnectionParams validates required connection parameters.
func (wm *WebSocketManager) validateConnectionParams(
	connectionID, executionID, token string,
) *events.APIGatewayProxyResponse {
	if connectionID == "" {
		wm.logger.Info("missing connection_id in connection request")
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Missing connection_id",
		}
	}

	if executionID == "" {
		wm.logger.Info("missing execution_id in connection request")
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Missing execution_id query parameter",
		}
	}

	if token == "" {
		wm.logger.Info("missing token in connection request", "execution_id", executionID)
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusUnauthorized,
			Body:       "Missing token query parameter",
		}
	}

	return nil
}

// validateAndConsumePendingToken validates the WebSocket token against pending connections.
// The token is reusable and persists until it expires via TTL, allowing clients to reconnect
// without needing to call /logs again. This enables seamless reconnection on connection drops.
func (wm *WebSocketManager) validateAndConsumePendingToken(
	ctx context.Context,
	executionID, token string,
) (*api.WebSocketConnection, *events.APIGatewayProxyResponse) {
	pendingConnections, err := wm.connRepo.GetConnectionsByExecutionID(ctx, executionID)
	if err != nil {
		wm.logger.Error("failed to validate websocket token", "error", err,
			"execution_id", executionID)
		return nil, &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Failed to validate token",
		}
	}

	var pendingConnection *api.WebSocketConnection
	for _, conn := range pendingConnections {
		if conn.Token == token {
			pendingConnection = conn
			break
		}
	}

	if pendingConnection == nil {
		wm.logger.Info("invalid or expired websocket token", "execution_id", executionID)
		return nil, &events.APIGatewayProxyResponse{
			StatusCode: http.StatusUnauthorized,
			Body:       "Invalid or expired token",
		}
	}

	return pendingConnection, nil
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

// NotifyExecutionCompletion sends disconnect notifications to all connected clients for an execution
// and deletes the connections from DynamoDB.
func (wm *WebSocketManager) NotifyExecutionCompletion(ctx context.Context, executionID *string) error {
	if executionID == nil || *executionID == "" {
		return fmt.Errorf("execution ID is nil or empty")
	}

	err := wm.handleDisconnectNotification(ctx, *executionID)
	if err != nil {
		return fmt.Errorf("failed to notify disconnect: %w", err)
	}

	deletedCount, err := wm.connRepo.DeleteConnections(ctx, wm.connectionIDs)
	if err != nil {
		wm.logger.Error("failed to delete WebSocket connections", "context",
			map[string]string{
				"error":        err.Error(),
				"execution_id": *executionID,
			},
		)
		// Don't fail - notifications were sent
	} else if deletedCount > 0 {
		wm.logger.Debug("deleted WebSocket connections for execution", "context",
			map[string]string{
				"execution_id":  *executionID,
				"deleted_count": fmt.Sprintf("%d", deletedCount),
			},
		)
	}

	return nil
}

// SendLogsToExecution sends log events to all connected clients for an execution.
// Each log event is sent individually to all connected clients concurrently.
func (wm *WebSocketManager) SendLogsToExecution(
	ctx context.Context,
	executionID *string,
	logEvents []api.LogEvent,
) error {
	if executionID == nil || *executionID == "" {
		return fmt.Errorf("execution ID is nil or empty")
	}

	if len(logEvents) == 0 {
		return nil
	}

	connections, err := wm.connRepo.GetConnectionsByExecutionID(ctx, *executionID)
	if err != nil {
		wm.logger.Error("failed to get connections for execution",
			"error", err, "execution_id", *executionID)
		return fmt.Errorf("failed to get connections: %w", err)
	}

	if len(connections) == 0 {
		wm.logger.Debug("no active connections to send logs to", "execution_id", *executionID)
		return nil
	}

	wm.logger.Debug("sending logs to connections",
		"context", map[string]any{
			"execution_id":     *executionID,
			"connection_count": len(connections),
			"log_count":        len(logEvents),
		},
	)

	for _, logEvent := range logEvents {
		eg, egCtx := errgroup.WithContext(ctx)
		eg.SetLimit(constants.MaxConcurrentSends)

		for _, conn := range connections {
			eg.Go(func() error {
				return wm.sendLogToConnection(egCtx, &conn.ConnectionID, logEvent)
			})
		}

		if egErr := eg.Wait(); egErr != nil {
			wm.logger.Error("some log sends failed", "context", map[string]any{
				"error":        egErr.Error(),
				"execution_id": *executionID,
				"timestamp":    logEvent.Timestamp,
			})
			return fmt.Errorf("failed to send logs to some connections: %w", egErr)
		}
	}

	wm.logger.Debug("all logs sent to all connections", "context", map[string]string{
		"execution_id":     *executionID,
		"connection_count": fmt.Sprintf("%d", len(connections)),
	})

	return nil
}

// sendLogToConnection sends a single log event to a WebSocket connection.
func (wm *WebSocketManager) sendLogToConnection(
	ctx context.Context,
	connectionID *string,
	logEvent api.LogEvent,
) error {
	if connectionID == nil {
		return fmt.Errorf("connection ID is nil")
	}

	logJSON, err := json.Marshal(logEvent)
	if err != nil {
		wm.logger.Error("failed to marshal log event",
			"context", map[string]any{
				"error":         err.Error(),
				"connection_id": connectionID,
				"timestamp":     logEvent.Timestamp,
			},
		)
		return fmt.Errorf("failed to marshal log event: %w", err)
	}

	_, err = wm.apiGwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: connectionID,
		Data:         logJSON,
	})

	if err != nil {
		wm.logger.Error("failed to send log to connection",
			"context", map[string]string{
				"error":         err.Error(),
				"connection_id": *connectionID,
			},
		)
		return fmt.Errorf("failed to send log to connection %s: %w", *connectionID, err)
	}

	return nil
}

// handleDisconnectNotification sends disconnect messages to all connected clients for an execution.
// This notifies clients that the execution has completed.
func (wm *WebSocketManager) handleDisconnectNotification(
	ctx context.Context,
	executionID string,
) error {
	var err error
	wm.logger.Debug("handling disconnect notification for execution", "execution_id", executionID)

	// Get all connections for this execution
	connections, err := wm.connRepo.GetConnectionsByExecutionID(ctx, executionID)
	if err != nil {
		wm.logger.Error("failed to get connections for execution", "error", err, "execution_id", executionID)
		return fmt.Errorf("failed to get connections: %w", err)
	}

	if len(connections) == 0 {
		wm.logger.Debug("no active connections to notify", "execution_id", executionID)
		return nil
	}

	// Extract connection IDs from connections
	wm.connectionIDs = make([]string, 0, len(connections))
	for _, conn := range connections {
		wm.connectionIDs = append(wm.connectionIDs, conn.ConnectionID)
	}

	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.SetLimit(constants.MaxConcurrentSends)

	reason := api.WebSocketDisconnectReasonExecutionCompleted
	disconnectMessage := api.WebSocketMessage{
		Type:   api.WebSocketMessageTypeDisconnect,
		Reason: &reason,
	}
	disconnectMessageBytes, err := json.Marshal(disconnectMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal disconnect message: %w", err)
	}

	for _, connectionID := range wm.connectionIDs {
		errGroup.Go(func() error {
			return wm.sendDisconnectToConnection(ctx, connectionID, disconnectMessageBytes)
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
