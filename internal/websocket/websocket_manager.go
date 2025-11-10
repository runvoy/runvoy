// Package websocket provides WebSocket management for runvoy.
// It handles connection lifecycle events and manages WebSocket connections in the database.
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	"runvoy/internal/logger"

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
	tokenRepo     database.TokenRepository
	backend       Backend
	logger        *slog.Logger
	connectionIDs []string
}

// NewWebSocketManager creates a new WebSocket manager.
func NewWebSocketManager(
	backend Backend,
	connRepo database.ConnectionRepository,
	tokenRepo database.TokenRepository,
	log *slog.Logger,
) *WebSocketManager {
	connectionIDs := make([]string, 0)

	log.Debug("websocket manager initialized")

	return &WebSocketManager{
		connRepo:      connRepo,
		tokenRepo:     tokenRepo,
		backend:       backend,
		logger:        log,
		connectionIDs: connectionIDs,
	}
}

// HandleRequest adapts WebSocket events so the generic event processor can route them.
// It parses the event using the backend and dispatches based on the route key.
func (wm *WebSocketManager) HandleRequest(
	ctx context.Context,
	rawEvent *json.RawMessage,
	reqLogger *slog.Logger,
) (bool, error) {
	if rawEvent == nil {
		return false, nil
	}

	// Parse event using backend
	event, err := wm.backend.ParseEvent(ctx, rawEvent, reqLogger)
	if err != nil {
		return false, fmt.Errorf("failed to parse websocket event: %w", err)
	}

	if event == nil {
		return false, nil // Not a WebSocket event
	}

	if event.RouteKey == "" {
		reqLogger.Error("missing route key in WebSocket request")
		return true, fmt.Errorf("missing route key in WebSocket request")
	}

	logArgs := []any{
		"route_key", event.RouteKey,
	}
	if event.ConnectionID != "" {
		logArgs = append(logArgs, "connection_id", event.ConnectionID)
	}
	if event.ClientIP != "" {
		logArgs = append(logArgs, "client_ip", event.ClientIP)
	}
	reqLogger.Debug("received WebSocket event", "context", logger.SliceToMap(logArgs))

	resp, err := wm.dispatchWebSocketRoute(ctx, reqLogger, event)
	if err != nil {
		return true, err
	}

	if webSocketErr := wm.evaluateRouteResponse(reqLogger, event.RouteKey, resp); webSocketErr != nil {
		return true, webSocketErr
	}

	return true, nil
}

func (wm *WebSocketManager) dispatchWebSocketRoute(
	ctx context.Context,
	reqLogger *slog.Logger,
	event *WebSocketEvent,
) (WebSocketResponse, error) {
	routeHandlers := map[string]func(
		context.Context, *WebSocketEvent) (WebSocketResponse, error){
		"$connect":    wm.handleConnect,
		"$disconnect": wm.handleDisconnect,
	}

	handler, ok := routeHandlers[event.RouteKey]
	if !ok {
		reqLogger.Error("unrecognized WebSocket route", "context", map[string]string{
			"route_key": event.RouteKey,
		})
		return WebSocketResponse{}, fmt.Errorf("unrecognized WebSocket route: %s", event.RouteKey)
	}

	resp, err := handler(ctx, event)
	if err != nil {
		reqLogger.Error("websocket route failed", "context", map[string]any{
			"route_key":   event.RouteKey,
			"error":       err.Error(),
			"status_code": resp.StatusCode,
			"body":        resp.Body,
		})
		return WebSocketResponse{}, fmt.Errorf("websocket route %s failed: %w", event.RouteKey, err)
	}

	return resp, nil
}

func (wm *WebSocketManager) evaluateRouteResponse(
	reqLogger *slog.Logger,
	routeKey string,
	resp WebSocketResponse,
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
// It validates the WebSocket token, stores the connection in the database, and returns a success response.
func (wm *WebSocketManager) handleConnect(
	ctx context.Context,
	event *WebSocketEvent,
) (WebSocketResponse, error) {
	connectionID := event.ConnectionID
	executionID := event.QueryParams["execution_id"]
	token := event.QueryParams["token"]

	if errResp := wm.validateConnectionParams(connectionID, executionID, token); errResp != nil {
		return *errResp, nil
	}

	wsToken, errResp := wm.fetchWebSocketToken(ctx, token, executionID)
	if errResp != nil {
		return *errResp, nil
	}

	connection := wm.newWebSocketConnection(event, token, wsToken)

	if err := wm.connRepo.CreateConnection(ctx, connection); err != nil {
		wm.logger.Error("failed to store connection", "error", err)
		return WebSocketResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to store connection: %v", err),
		}, nil
	}

	wm.logConnectionEstablished(connection)

	return WebSocketResponse{
		StatusCode: http.StatusOK,
		Body:       "Connected",
	}, nil
}

func (wm *WebSocketManager) fetchWebSocketToken(
	ctx context.Context,
	token string,
	executionID string,
) (*api.WebSocketToken, *WebSocketResponse) {
	wsToken, err := wm.tokenRepo.GetToken(ctx, token)
	if err != nil {
		wm.logger.Error("failed to validate token", "error", err, "execution_id", executionID)
		return nil, &WebSocketResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Failed to validate token",
		}
	}

	if wsToken == nil {
		wm.logger.Info("invalid or expired websocket token", "execution_id", executionID)
		return nil, &WebSocketResponse{
			StatusCode: http.StatusUnauthorized,
			Body:       "Invalid or expired token",
		}
	}

	return wsToken, nil
}

func (wm *WebSocketManager) newWebSocketConnection(
	event *WebSocketEvent,
	token string,
	wsToken *api.WebSocketToken,
) *api.WebSocketConnection {
	return &api.WebSocketConnection{
		ConnectionID:         event.ConnectionID,
		ExecutionID:          event.QueryParams["execution_id"],
		Functionality:        constants.FunctionalityLogStreaming,
		ExpiresAt:            time.Now().Add(constants.ConnectionTTLHours * time.Hour).Unix(),
		Token:                token, // Keep the token for cleanup on disconnect
		ClientIP:             event.ClientIP,
		UserEmail:            wsToken.UserEmail,
		TokenRequestClientIP: wsToken.ClientIP,
	}
}

func (wm *WebSocketManager) logConnectionEstablished(connection *api.WebSocketConnection) {
	wm.logger.Info("authenticated connection established", "context", map[string]any{
		"connection_id":           connection.ConnectionID,
		"execution_id":            connection.ExecutionID,
		"functionality":           connection.Functionality,
		"expires_at":              connection.ExpiresAt,
		"client_ip":               connection.ClientIP,
		"user_email":              connection.UserEmail,
		"token_request_client_ip": connection.TokenRequestClientIP,
	})
}

// validateConnectionParams validates required connection parameters.
func (wm *WebSocketManager) validateConnectionParams(
	connectionID, executionID, token string,
) *WebSocketResponse {
	if connectionID == "" {
		wm.logger.Info("missing connection_id in connection request")
		return &WebSocketResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Missing connection_id",
		}
	}

	if executionID == "" {
		wm.logger.Info("missing execution_id in connection request")
		return &WebSocketResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Missing execution_id query parameter",
		}
	}

	if token == "" {
		wm.logger.Info("missing token in connection request", "execution_id", executionID)
		return &WebSocketResponse{
			StatusCode: http.StatusUnauthorized,
			Body:       "Missing token query parameter",
		}
	}

	return nil
}

// handleDisconnect handles the $disconnect route key.
// It deletes the WebSocket connection and its associated token from the database.
func (wm *WebSocketManager) handleDisconnect(
	ctx context.Context,
	event *WebSocketEvent,
) (WebSocketResponse, error) {
	connectionID := event.ConnectionID

	if connectionID == "" {
		wm.logger.Info("missing connection_id in disconnect request")
		return WebSocketResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Missing connection_id",
		}, nil
	}

	wm.logger.Debug("deleting connection", "context", map[string]string{
		"connection_id": connectionID,
	})

	// Delete the connection
	deletedCount, err := wm.connRepo.DeleteConnections(ctx, []string{connectionID})
	if err != nil {
		wm.logger.Error("failed to delete connection", "error", err)
		return WebSocketResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to delete connection: %v", err),
		}, nil
	}

	// Token cleanup: Database TTL automatically deletes tokens after expiration.
	// Manual deletion on disconnect is not necessary since the token expires with
	// the same TTL as the connection.

	wm.logger.Info("connection disconnected", "context", map[string]any{
		"connection_id": connectionID,
		"deleted_count": deletedCount,
	})

	return WebSocketResponse{
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

	err = wm.backend.SendToConnection(ctx, *connectionID, logJSON)
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

	err := wm.backend.SendToConnection(ctx, connectionID, message)
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
