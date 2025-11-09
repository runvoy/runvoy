// Package websocket provides WebSocket management for runvoy.
// It handles connection lifecycle events and manages WebSocket connections in DynamoDB.
package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"golang.org/x/sync/errgroup"
)

const (
	// ReplayLockTTLSeconds is the time-to-live for replay locks (30 seconds)
	ReplayLockTTLSeconds = 30
)

// Manager exposes the subset of WebSocket manager functionality used by the event processor.
type Manager interface {
	HandleRequest(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error)
	NotifyExecutionCompletion(ctx context.Context, executionID string) error
	SendLogsToExecution(ctx context.Context, executionID string, logEvents []api.LogEvent) error
}

// WebSocketManager handles WebSocket connection lifecycle events and disconnect notifications.
//
//nolint:revive // exported type name is intentional for clarity
type WebSocketManager struct {
	connRepo      database.ConnectionRepository
	logRepo       database.LogRepository
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
	logRepo database.LogRepository,
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
		logRepo:       logRepo,
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
		"$connect":              wm.handleConnect,
		"$disconnect":           wm.handleDisconnect,
		"$disconnect-execution": wm.handleDisconnectExecution,
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
		reqLogger.Debug("websocket handler returned client error response", "context", map[string]any{
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
// It validates the WebSocket token, stores the connection in DynamoDB with a replay lock,
// fetches any historical logs for backlog replay, sends them to the client,
// and then clears the replay lock to allow live events to be sent.
//
//nolint:gocritic // Lambda event types passed by value; orchestrates replay helpers
func (wm *WebSocketManager) handleConnect(
	ctx context.Context,
	req events.APIGatewayWebsocketProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	params := buildConnectParams(&req)

	pendingConnection, errResp := wm.validateConnectRequest(ctx, params)
	if errResp != nil {
		return *errResp, nil
	}

	connection, errResp := wm.initializeConnection(ctx, &req, params, pendingConnection)
	if errResp != nil {
		return *errResp, nil
	}

	backlog, errResp := wm.replayBacklogForConnection(ctx, connection, params.lastSeenTimestamp)
	if errResp != nil {
		return *errResp, nil
	}

	wm.logSuccessfulConnect(connection, backlog)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "Connected",
	}, nil
}

type connectRequestParams struct {
	connectionID      string
	executionID       string
	token             string
	lastSeenTimestamp *int64
}

func buildConnectParams(req *events.APIGatewayWebsocketProxyRequest) connectRequestParams {
	return connectRequestParams{
		connectionID:      req.RequestContext.ConnectionID,
		executionID:       req.QueryStringParameters["execution_id"],
		token:             req.QueryStringParameters["token"],
		lastSeenTimestamp: parseLastSeenTimestamp(req.QueryStringParameters["last_seen_timestamp"]),
	}
}

func parseLastSeenTimestamp(raw string) *int64 {
	if raw == "" {
		return nil
	}

	var ts int64
	if _, err := fmt.Sscanf(raw, "%d", &ts); err == nil && ts > 0 {
		return &ts
	}

	return nil
}

func (wm *WebSocketManager) validateConnectRequest(
	ctx context.Context,
	params connectRequestParams,
) (*api.WebSocketConnection, *events.APIGatewayProxyResponse) {
	if errResp := wm.validateConnectionParams(params.connectionID, params.executionID, params.token); errResp != nil {
		return nil, errResp
	}

	return wm.validateAndConsumePendingToken(ctx, params.executionID, params.token)
}

func (wm *WebSocketManager) initializeConnection(
	ctx context.Context,
	req *events.APIGatewayWebsocketProxyRequest,
	params connectRequestParams,
	pendingConnection *api.WebSocketConnection,
) (*api.WebSocketConnection, *events.APIGatewayProxyResponse) {
	replayLockTTL := time.Now().Unix() + ReplayLockTTLSeconds

	connection := &api.WebSocketConnection{
		ConnectionID:         params.connectionID,
		ExecutionID:          params.executionID,
		Functionality:        constants.FunctionalityLogStreaming,
		ExpiresAt:            pendingConnection.ExpiresAt,
		ClientIP:             getClientIPFromWebSocketRequest(req),
		UserEmail:            pendingConnection.UserEmail,
		ClientIPAtLogsTime:   pendingConnection.ClientIPAtLogsTime,
		LastSeenLogTimestamp: 0, // Will be updated after fetching backlog
		ReplayLock:           true,
		ReplayLockExpiresAt:  replayLockTTL,
	}

	if err := wm.connRepo.CreateConnection(ctx, connection); err != nil {
		wm.logger.Error("failed to store connection", "context", map[string]any{
			"error":         err.Error(),
			"connection_id": connection.ConnectionID,
			"execution_id":  connection.ExecutionID,
		})
		return nil, &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to store connection: %v", err),
		}
	}

	wm.logger.Debug("connection created with replay lock", "context", map[string]any{
		"connection_id": connection.ConnectionID,
		"execution_id":  connection.ExecutionID,
		"replay_lock":   true,
	})

	return connection, nil
}

func (wm *WebSocketManager) replayBacklogForConnection(
	ctx context.Context,
	connection *api.WebSocketConnection,
	lastSeenTimestamp *int64,
) ([]api.LogEvent, *events.APIGatewayProxyResponse) {
	backlog, err := wm.fetchBacklogWithRetry(ctx, connection.ExecutionID, lastSeenTimestamp)
	if err != nil {
		wm.logger.Error("failed to fetch backlog after retries", "context", map[string]any{
			"error":               err.Error(),
			"execution_id":        connection.ExecutionID,
			"last_seen_timestamp": lastSeenTimestamp,
		})

		if clearErr := wm.clearReplayLock(ctx, connection.ConnectionID, connection.ExecutionID); clearErr != nil {
			wm.logger.Error("failed to clear replay lock after backlog fetch error", "error", clearErr)
		}

		return nil, &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to fetch backlog: %v", err),
		}
	}

	if len(backlog) > 0 {
		wm.logger.Debug("sending backlog to client", "context", map[string]any{
			"execution_id": connection.ExecutionID,
			"log_count":    len(backlog),
		})

		if sendErr := wm.SendLogsToExecution(ctx, connection.ExecutionID, backlog); sendErr != nil {
			wm.logger.Error("failed to send backlog", "error", sendErr, "execution_id", connection.ExecutionID)
		}

		connection.LastSeenLogTimestamp = backlog[len(backlog)-1].Timestamp
	}

	if clearErr := wm.clearReplayLock(ctx, connection.ConnectionID, connection.ExecutionID); clearErr != nil {
		wm.logger.Error("failed to clear replay lock", "error", clearErr)
		return nil, &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to finalize connection: %v", clearErr),
		}
	}

	return backlog, nil
}

// fetchBacklogWithRetry attempts to fetch the execution backlog with retries and tolerance.
// It tolerates the log stream not existing yet (common during task startup) and returns
// an empty backlog in that case. Non-recoverable errors are returned after retries exhaust.
func (wm *WebSocketManager) fetchBacklogWithRetry(
	ctx context.Context,
	executionID string,
	lastSeenTimestamp *int64,
) ([]api.LogEvent, error) {
	const maxRetries = 4
	const retryDelay = 5 * time.Second

	var lastErr error

	for attempt := range maxRetries {
		backlog, err := wm.logRepo.GetLogsByExecutionIDSince(ctx, executionID, lastSeenTimestamp)
		if err == nil {
			// Success
			return backlog, nil
		}

		lastErr = err

		// Check if this is a "stream doesn't exist yet" error, which is expected during early startup
		if wm.isLogStreamNotReadyError(err, executionID) {
			wm.logger.Debug("log stream not ready yet, will retry", "context", map[string]any{
				"attempt":      attempt + 1,
				"max_retries":  maxRetries,
				"execution_id": executionID,
				"delay":        retryDelay,
			})

			// Wait before retrying, unless this is the last attempt
			if attempt < maxRetries-1 {
				select {
				case <-time.After(retryDelay):
					// Continue to next iteration
				case <-ctx.Done():
					return nil, fmt.Errorf("context canceled while waiting to retry backlog fetch: %w", ctx.Err())
				}
			}
			continue
		}

		// For non-recoverable errors, fail immediately
		return nil, fmt.Errorf("failed to fetch backlog: %w", err)
	}

	// After all retries, if stream still doesn't exist, return empty backlog (graceful degradation)
	if wm.isLogStreamNotReadyError(lastErr, executionID) {
		wm.logger.Warn("log stream not ready, proceeding with empty backlog", "context", map[string]string{
			"execution_id": executionID,
			"max_retries":  fmt.Sprintf("%d", maxRetries),
		})
		return []api.LogEvent{}, nil
	}

	// Otherwise return the last error
	return nil, lastErr
}

// isLogStreamNotReadyError checks whether the provided error indicates that the log stream has not
// been created yet for the given execution.
func (wm *WebSocketManager) isLogStreamNotReadyError(err error, executionID string) bool {
	if err == nil {
		return false
	}

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		return false
	}

	if appErr.Code != apperrors.ErrCodeNotFound {
		return false
	}

	expectedMessage := fmt.Sprintf("log stream '%s' does not exist yet", constants.BuildLogStreamName(executionID))
	return appErr.Message == expectedMessage
}

func (wm *WebSocketManager) logSuccessfulConnect(
	connection *api.WebSocketConnection,
	backlog []api.LogEvent,
) {
	wm.logger.Info("authenticated connection established with backlog replay", "context", map[string]any{
		"connection_id":           connection.ConnectionID,
		"execution_id":            connection.ExecutionID,
		"functionality":           connection.Functionality,
		"expires_at":              connection.ExpiresAt,
		"client_ip":               connection.ClientIP,
		"user_email":              connection.UserEmail,
		"backlog_sent":            len(backlog) > 0,
		"backlog_count":           len(backlog),
		"last_seen_log_timestamp": connection.LastSeenLogTimestamp,
	})
}

// clearReplayLock clears the replay lock flag on a connection and updates it in DynamoDB.
func (wm *WebSocketManager) clearReplayLock(
	ctx context.Context,
	connectionID string,
	executionID string,
) error {
	// Fetch the current connection
	connections, err := wm.connRepo.GetConnectionsByExecutionID(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get connection for replay lock cleanup: %w", err)
	}

	var currentConn *api.WebSocketConnection
	for _, conn := range connections {
		if conn.ConnectionID == connectionID {
			currentConn = conn
			break
		}
	}

	if currentConn == nil {
		return fmt.Errorf("connection not found for cleanup: %s", connectionID)
	}

	// Clear the replay lock
	currentConn.ReplayLock = false
	currentConn.ReplayLockExpiresAt = 0

	if updateErr := wm.connRepo.UpdateConnection(ctx, currentConn); updateErr != nil {
		return fmt.Errorf("failed to update connection after replay lock clear: %w", updateErr)
	}

	wm.logger.Debug("replay lock cleared", "context", map[string]string{
		"connection_id": connectionID,
		"execution_id":  executionID,
	})

	return nil
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

// validateAndConsumePendingToken validates the WebSocket token against pending connections
// and deletes the pending connection entry after validation.
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

	_, delErr := wm.connRepo.DeleteConnections(ctx, []string{pendingConnection.ConnectionID})
	if delErr != nil {
		wm.logger.Error("failed to delete pending connection", "error", delErr,
			"connection_id", pendingConnection.ConnectionID)
		return nil, &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Failed to finalize connection",
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
func (wm *WebSocketManager) NotifyExecutionCompletion(ctx context.Context, executionID string) error {
	if executionID == "" {
		return fmt.Errorf("execution ID is empty")
	}

	err := wm.handleDisconnectNotification(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to notify disconnect: %w", err)
	}

	deletedCount, err := wm.connRepo.DeleteConnections(ctx, wm.connectionIDs)
	if err != nil {
		wm.logger.Error("failed to delete WebSocket connections", "context",
			map[string]string{
				"error":        err.Error(),
				"execution_id": executionID,
			},
		)
		// Don't fail - notifications were sent
	} else if deletedCount > 0 {
		wm.logger.Debug("deleted WebSocket connections for execution", "context",
			map[string]string{
				"execution_id":  executionID,
				"deleted_count": fmt.Sprintf("%d", deletedCount),
			},
		)
	}

	return nil
}

// SendLogsToExecution sends log events to all connected clients for an execution.
// Connections with an active replay lock are skipped (no buffering, they will catch up
// when their backlog replay completes and the lock is cleared).
// Each log event is sent individually to all non-locked clients concurrently.
//
//nolint:gocyclo,funlen // Replay lock logic increases complexity but is necessary
func (wm *WebSocketManager) SendLogsToExecution(
	ctx context.Context,
	executionID string,
	logEvents []api.LogEvent,
) error {
	if executionID == "" {
		return fmt.Errorf("execution ID is empty")
	}

	if len(logEvents) == 0 {
		return nil
	}

	connections, err := wm.connRepo.GetConnectionsByExecutionID(ctx, executionID)
	if err != nil {
		wm.logger.Error("failed to get connections for execution",
			"error", err, "execution_id", executionID)
		return fmt.Errorf("failed to get connections: %w", err)
	}

	if len(connections) == 0 {
		wm.logger.Debug("no active connections to send logs to", "execution_id", executionID)
		return nil
	}

	// Filter out connections with active replay lock
	now := time.Now().Unix()
	activeConnections := make([]*api.WebSocketConnection, 0, len(connections))
	skippedConnections := 0

	for _, conn := range connections {
		// Check if replay lock is active (not yet expired)
		if conn.ReplayLock && conn.ReplayLockExpiresAt > now {
			skippedConnections++
			wm.logger.Debug("skipping connection with active replay lock", "context", map[string]any{
				"connection_id":          conn.ConnectionID,
				"execution_id":           executionID,
				"replay_lock_expires_at": conn.ReplayLockExpiresAt,
			})
			continue
		}

		// Clear stale replay locks (expired)
		if conn.ReplayLock && conn.ReplayLockExpiresAt <= now {
			wm.logger.Warn("clearing stale replay lock", "context", map[string]any{
				"connection_id": conn.ConnectionID,
				"execution_id":  executionID,
			})
			conn.ReplayLock = false
			conn.ReplayLockExpiresAt = 0
			if updateErr := wm.connRepo.UpdateConnection(ctx, conn); updateErr != nil {
				wm.logger.Error("failed to clear stale replay lock", "error", updateErr)
			}
		}

		activeConnections = append(activeConnections, conn)
	}

	if len(activeConnections) == 0 {
		wm.logger.Debug("all connections have active replay locks, logs not sent", "context", map[string]any{
			"execution_id": executionID,
			"log_count":    len(logEvents),
		})
		return nil
	}

	wm.logger.Debug("sending logs to connections",
		"context", map[string]any{
			"execution_id":     executionID,
			"connection_count": len(activeConnections),
			"skipped_count":    skippedConnections,
			"log_count":        len(logEvents),
		},
	)

	// Send logs to active (non-locked) connections
	for _, logEvent := range logEvents {
		eg, egCtx := errgroup.WithContext(ctx)
		eg.SetLimit(constants.MaxConcurrentSends)

		for _, conn := range activeConnections {
			eg.Go(func() error {
				// Send log and update LastSeenLogTimestamp
				if sendErr := wm.sendLogToConnection(egCtx, &conn.ConnectionID, logEvent); sendErr != nil {
					return sendErr
				}
				// Update the connection record with the latest timestamp
				conn.LastSeenLogTimestamp = logEvent.Timestamp
				if updateErr := wm.connRepo.UpdateConnection(egCtx, conn); updateErr != nil {
					wm.logger.Error("failed to update connection timestamp", "error", updateErr)
					// Don't fail the send just because we couldn't update the timestamp
				}
				return nil
			})
		}

		if egErr := eg.Wait(); egErr != nil {
			wm.logger.Error("some log sends failed", "context", map[string]any{
				"error":        egErr.Error(),
				"execution_id": executionID,
				"timestamp":    logEvent.Timestamp,
			})
			return fmt.Errorf("failed to send logs to some connections: %w", egErr)
		}
	}

	wm.logger.Debug("logs sent to active connections", "context", map[string]any{
		"execution_id":     executionID,
		"connection_count": len(activeConnections),
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

	// Use the shared notification method
	err := wm.NotifyExecutionCompletion(ctx, executionID)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to notify disconnect: %v", err),
		}, nil
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
