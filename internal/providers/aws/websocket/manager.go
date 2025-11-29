package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/auth"
	"github.com/runvoy/runvoy/internal/config"
	"github.com/runvoy/runvoy/internal/constants"
	"github.com/runvoy/runvoy/internal/database"
	"github.com/runvoy/runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"golang.org/x/sync/errgroup"
)

// Manager implements the websocket.Manager interface for AWS.
// It uses API Gateway Management API to communicate with WebSocket clients
// and DynamoDB to store connection metadata.
type Manager struct {
	connRepo      database.ConnectionRepository
	tokenRepo     database.TokenRepository
	logEventRepo  database.LogEventRepository
	apiGwClient   Client
	apiGwEndpoint *string
	logger        *slog.Logger
	connectionIDs []string
}

// Initialize creates a new AWS WebSocket manager.
func Initialize(
	cfg *config.Config,
	connRepo database.ConnectionRepository,
	tokenRepo database.TokenRepository,
	logEventRepo database.LogEventRepository,
	log *slog.Logger,
) *Manager {
	apiGwSDKClient := apigatewaymanagementapi.NewFromConfig(*cfg.AWS.SDKConfig, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(cfg.AWS.WebSocketAPIEndpoint)
	})
	apiGwClient := NewClientAdapter(apiGwSDKClient)
	connectionIDs := make([]string, 0)

	log.Debug("websocket manager initialized",
		"context", map[string]string{
			"connections_table": cfg.AWS.WebSocketConnectionsTable,
			"tokens_table":      cfg.AWS.WebSocketTokensTable,
			"api_endpoint":      cfg.AWS.WebSocketAPIEndpoint,
		},
	)

	return &Manager{
		connRepo:      connRepo,
		tokenRepo:     tokenRepo,
		logEventRepo:  logEventRepo,
		apiGwClient:   apiGwClient,
		apiGwEndpoint: aws.String(cfg.AWS.WebSocketAPIEndpoint),
		logger:        log,
		connectionIDs: connectionIDs,
	}
}

func (m *Manager) deriveLogger(ctx context.Context) *slog.Logger {
	return logger.DeriveRequestLogger(ctx, m.logger)
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
func (m *Manager) HandleRequest(
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

	resp, err := m.dispatchWebSocketRoute(ctx, reqLogger, &req)
	if err != nil {
		return handled, err
	}

	if webSocketErr := m.evaluateRouteResponse(reqLogger, req.RequestContext.RouteKey, resp); webSocketErr != nil {
		return handled, webSocketErr
	}

	return handled, nil
}

func (m *Manager) dispatchWebSocketRoute(
	ctx context.Context,
	reqLogger *slog.Logger,
	req *events.APIGatewayWebsocketProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	routeHandlers := map[string]func(
		context.Context,
		*slog.Logger,
		events.APIGatewayWebsocketProxyRequest,
	) (events.APIGatewayProxyResponse, error){
		"$connect":    m.handleConnect,
		"$disconnect": m.handleDisconnect,
	}

	handler, ok := routeHandlers[req.RequestContext.RouteKey]
	if !ok {
		reqLogger.Error("unrecognized WebSocket route", "context", map[string]string{
			"route_key": req.RequestContext.RouteKey,
		})
		return events.APIGatewayProxyResponse{}, fmt.Errorf("unrecognized WebSocket route: %s", req.RequestContext.RouteKey)
	}

	resp, err := handler(ctx, reqLogger, *req)
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

func (m *Manager) evaluateRouteResponse(
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
	}

	return nil
}

// handleConnect handles the $connect route key.
// It validates the WebSocket token, stores the connection in DynamoDB, and returns a success response.
//
//nolint:gocritic // Lambda event types are passed by value per AWS Lambda conventions
func (m *Manager) handleConnect(
	ctx context.Context,
	reqLogger *slog.Logger,
	req events.APIGatewayWebsocketProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	connectionID := req.RequestContext.ConnectionID
	executionID := req.QueryStringParameters["execution_id"]
	token := req.QueryStringParameters["token"]

	if errResp := m.validateConnectionParams(reqLogger, connectionID, executionID, token); errResp != nil {
		return *errResp, nil
	}

	wsToken, errResp := m.fetchWebSocketToken(ctx, reqLogger, token, executionID)
	if errResp != nil {
		return *errResp, nil
	}

	connection := m.newWebSocketConnection(&req, token, wsToken)

	if err := m.connRepo.CreateConnection(ctx, connection); err != nil {
		reqLogger.Error("failed to store connection", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to store connection: %v", err),
		}, nil
	}

	m.logConnectionEstablished(reqLogger, connection)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "Connected",
	}, nil
}

func (m *Manager) fetchWebSocketToken(
	ctx context.Context,
	reqLogger *slog.Logger,
	token string,
	executionID string,
) (*api.WebSocketToken, *events.APIGatewayProxyResponse) {
	wsToken, err := m.tokenRepo.GetToken(ctx, token)
	if err != nil {
		reqLogger.Error("failed to validate token", "error", err, "execution_id", executionID)
		return nil, &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Failed to validate token",
		}
	}

	if wsToken == nil {
		reqLogger.Info("invalid or expired websocket token", "execution_id", executionID)
		return nil, &events.APIGatewayProxyResponse{
			StatusCode: http.StatusUnauthorized,
			Body:       "Invalid or expired token",
		}
	}

	if wsToken.ExecutionID != executionID {
		reqLogger.Warn("execution ID mismatch in websocket token",
			"token_execution_id", wsToken.ExecutionID,
			"request_execution_id", executionID)
		return nil, &events.APIGatewayProxyResponse{
			StatusCode: http.StatusForbidden,
			Body:       "Token is not valid for this execution",
		}
	}

	return wsToken, nil
}

func (m *Manager) newWebSocketConnection(
	req *events.APIGatewayWebsocketProxyRequest,
	token string,
	wsToken *api.WebSocketToken,
) *api.WebSocketConnection {
	return &api.WebSocketConnection{
		ConnectionID:         req.RequestContext.ConnectionID,
		ExecutionID:          req.QueryStringParameters["execution_id"],
		Functionality:        constants.FunctionalityLogStreaming,
		ExpiresAt:            time.Now().Add(constants.ConnectionTTLHours * time.Hour).Unix(),
		LastEventID:          req.QueryStringParameters["last_event_id"],
		Token:                token, // Keep the token for cleanup on disconnect
		ClientIP:             getClientIPFromWebSocketRequest(req),
		UserEmail:            wsToken.UserEmail,
		TokenRequestClientIP: wsToken.ClientIP,
	}
}

func (m *Manager) logConnectionEstablished(reqLogger *slog.Logger, connection *api.WebSocketConnection) {
	reqLogger.Info("authenticated connection established", "context", map[string]string{
		"connection_id":           connection.ConnectionID,
		"execution_id":            connection.ExecutionID,
		"functionality":           connection.Functionality,
		"expires_at":              time.Unix(connection.ExpiresAt, 0).Format(time.RFC3339),
		"client_ip":               connection.ClientIP,
		"user_email":              connection.UserEmail,
		"token_request_client_ip": connection.TokenRequestClientIP,
	})
}

// validateConnectionParams validates required connection parameters.
func (m *Manager) validateConnectionParams(
	reqLogger *slog.Logger,
	connectionID, executionID, token string,
) *events.APIGatewayProxyResponse {
	if connectionID == "" {
		reqLogger.Info("missing connection_id in connection request")
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Missing connection_id",
		}
	}

	if executionID == "" {
		reqLogger.Info("missing execution_id in connection request")
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Missing execution_id query parameter",
		}
	}

	if token == "" {
		reqLogger.Info("missing token in connection request", "execution_id", executionID)
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusUnauthorized,
			Body:       "Missing token query parameter",
		}
	}

	return nil
}

// handleDisconnect handles the $disconnect route key.
// It deletes the WebSocket connection and its associated token from DynamoDB.
// Token cleanup: DynamoDB TTL automatically deletes tokens after expiration.
// Manual deletion on disconnect is not necessary since the token expires with
// the same TTL as the connection.
//
//nolint:gocritic // Lambda event types are passed by value per AWS Lambda conventions
func (m *Manager) handleDisconnect(
	ctx context.Context,
	reqLogger *slog.Logger,
	req events.APIGatewayWebsocketProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	connectionID := req.RequestContext.ConnectionID

	if connectionID == "" {
		reqLogger.Info("missing connection_id in disconnect request")
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Missing connection_id",
		}, nil
	}

	reqLogger.Debug("deleting connection", "context", map[string]string{
		"connection_id": connectionID,
	})

	deletedCount, err := m.connRepo.DeleteConnections(ctx, []string{connectionID})
	if err != nil {
		reqLogger.Error("failed to delete connection", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to delete connection: %v", err),
		}, nil
	}

	reqLogger.Info("connection disconnected", "context", map[string]any{
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
func (m *Manager) NotifyExecutionCompletion(ctx context.Context, executionID *string) error {
	if executionID == nil || *executionID == "" {
		return fmt.Errorf("execution ID is nil or empty")
	}

	reqLogger := m.deriveLogger(ctx)

	err := m.handleDisconnectNotification(ctx, reqLogger, *executionID)
	if err != nil {
		return fmt.Errorf("failed to notify disconnect: %w", err)
	}

	deletedCount, err := m.connRepo.DeleteConnections(ctx, m.connectionIDs)
	if err != nil {
		reqLogger.Error("failed to delete WebSocket connections", "context",
			map[string]string{
				"error":        err.Error(),
				"execution_id": *executionID,
			},
		)
		// Don't fail - notifications were sent
	} else if deletedCount > 0 {
		reqLogger.Debug("deleted WebSocket connections for execution", "context",
			map[string]string{
				"execution_id":  *executionID,
				"deleted_count": fmt.Sprintf("%d", deletedCount),
			},
		)
	}

	return nil
}

// SendLogsToExecution loads buffered log events for an execution and forwards
// them to all connected clients. Each log event is sent individually to all
// connections concurrently.
func (m *Manager) SendLogsToExecution(
	ctx context.Context,
	executionID *string,
) error {
	reqLogger := m.deriveLogger(ctx)

	execID, err := validateExecutionID(executionID)
	if err != nil {
		return err
	}

	connections, err := m.loadConnections(ctx, reqLogger, execID)
	if err != nil {
		return err
	}

	if len(connections) == 0 {
		reqLogger.Debug("no active connections to send logs to", "execution_id", execID)
		return nil
	}

	bufferedEvents, err := m.loadBufferedEvents(ctx, reqLogger, execID)
	if err != nil {
		return err
	}

	if len(bufferedEvents) == 0 {
		reqLogger.Debug("no buffered logs available", "context", map[string]string{
			"execution_id": execID,
		})
		return nil
	}

	return m.distributeBufferedEvents(ctx, reqLogger, execID, connections, bufferedEvents)
}

func (m *Manager) sendBufferedLogsToConnection(
	ctx context.Context,
	reqLogger *slog.Logger,
	connection *api.WebSocketConnection,
	bufferedEvents []api.LogEvent,
) error {
	eventsToSend := filterEventsAfter(bufferedEvents, connection.LastEventID)
	if len(eventsToSend) == 0 {
		reqLogger.Debug("no buffered logs to send to connection", "context", map[string]string{
			"connection_id": connection.ConnectionID,
		})
		return nil
	}

	for _, event := range eventsToSend {
		if err := m.sendLogToConnection(ctx, reqLogger, connection.ConnectionID, event); err != nil {
			return err
		}
	}

	lastEventID := eventsToSend[len(eventsToSend)-1].EventID
	if lastEventID == "" {
		return nil
	}

	if err := m.connRepo.UpdateLastEventID(ctx, connection.ConnectionID, lastEventID); err != nil {
		reqLogger.Error("failed to update last event ID", "context", map[string]any{
			"connection_id": connection.ConnectionID,
			"last_event_id": lastEventID,
			"error":         err.Error(),
		})
		return fmt.Errorf("failed to update last event ID: %w", err)
	}

	return nil
}

func filterEventsAfter(logEvents []api.LogEvent, lastEventID string) []api.LogEvent {
	if lastEventID == "" {
		return logEvents
	}

	for idx, event := range logEvents {
		if event.EventID == lastEventID {
			if idx+1 >= len(logEvents) {
				return []api.LogEvent{}
			}
			return logEvents[idx+1:]
		}
	}

	return logEvents
}

func validateExecutionID(executionID *string) (string, error) {
	if executionID == nil || *executionID == "" {
		return "", fmt.Errorf("execution ID is nil or empty")
	}
	return *executionID, nil
}

func (m *Manager) loadConnections(
	ctx context.Context,
	reqLogger *slog.Logger,
	executionID string,
) ([]*api.WebSocketConnection, error) {
	connections, err := m.connRepo.GetConnectionsByExecutionID(ctx, executionID)
	if err != nil {
		reqLogger.Error("failed to get connections for execution",
			"error", err, "execution_id", executionID)
		return nil, fmt.Errorf("failed to get connections: %w", err)
	}

	return connections, nil
}

func (m *Manager) loadBufferedEvents(
	ctx context.Context,
	reqLogger *slog.Logger,
	executionID string,
) ([]api.LogEvent, error) {
	bufferedEvents, err := m.logEventRepo.ListLogEvents(ctx, executionID)
	if err != nil {
		reqLogger.Error("failed to retrieve buffered logs",
			"error", err, "execution_id", executionID)
		return nil, fmt.Errorf("failed to retrieve buffered logs: %w", err)
	}

	return bufferedEvents, nil
}

func (m *Manager) distributeBufferedEvents(
	ctx context.Context,
	reqLogger *slog.Logger,
	executionID string,
	connections []*api.WebSocketConnection,
	bufferedEvents []api.LogEvent,
) error {
	reqLogger.Debug("sending buffered logs to connections",
		"context", map[string]any{
			"execution_id":     executionID,
			"connection_count": len(connections),
			"log_count":        len(bufferedEvents),
		},
	)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(constants.MaxConcurrentSends)

	for _, conn := range connections {
		eg.Go(func() error {
			return m.sendBufferedLogsToConnection(egCtx, reqLogger, conn, bufferedEvents)
		})
	}

	if egErr := eg.Wait(); egErr != nil {
		reqLogger.Error("some log sends failed", "context", map[string]any{
			"error":        egErr.Error(),
			"execution_id": executionID,
		})
		return fmt.Errorf("failed to send logs to some connections: %w", egErr)
	}

	reqLogger.Debug("all buffered logs sent to connections", "context", map[string]string{
		"execution_id":     executionID,
		"connection_count": fmt.Sprintf("%d", len(connections)),
	})

	return nil
}

// sendLogToConnection sends a single log event to a WebSocket connection.
func (m *Manager) sendLogToConnection(
	ctx context.Context,
	reqLogger *slog.Logger,
	connectionID string,
	logEvent api.LogEvent,
) error {
	if connectionID == "" {
		return fmt.Errorf("connection ID is empty")
	}

	logJSON, err := json.Marshal(logEvent)
	if err != nil {
		reqLogger.Error("failed to marshal log event",
			"context", map[string]any{
				"error":         err.Error(),
				"connection_id": connectionID,
				"timestamp":     logEvent.Timestamp,
			},
		)
		return fmt.Errorf("failed to marshal log event: %w", err)
	}

	_, err = m.apiGwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         logJSON,
	})

	if err != nil {
		reqLogger.Error("failed to send log to connection",
			"context", map[string]string{
				"error":         err.Error(),
				"connection_id": connectionID,
			},
		)
		return fmt.Errorf("failed to send log to connection %s: %w", connectionID, err)
	}

	return nil
}

// handleDisconnectNotification sends disconnect messages to all connected clients for an execution.
// This notifies clients that the execution has completed.
func (m *Manager) handleDisconnectNotification(
	ctx context.Context,
	reqLogger *slog.Logger,
	executionID string,
) error {
	var err error
	reqLogger.Debug("handling disconnect notification for execution", "execution_id", executionID)

	connections, err := m.connRepo.GetConnectionsByExecutionID(ctx, executionID)
	if err != nil {
		reqLogger.Error("failed to get connections for execution", "error", err, "execution_id", executionID)
		return fmt.Errorf("failed to get connections: %w", err)
	}

	if len(connections) == 0 {
		reqLogger.Debug("no active connections to notify", "execution_id", executionID)
		return nil
	}

	m.connectionIDs = make([]string, 0, len(connections))
	for _, conn := range connections {
		m.connectionIDs = append(m.connectionIDs, conn.ConnectionID)
	}

	errGroup, errCtx := errgroup.WithContext(ctx)
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

	for _, connectionID := range m.connectionIDs {
		errGroup.Go(func() error {
			return m.sendDisconnectToConnection(errCtx, reqLogger, connectionID, disconnectMessageBytes)
		})
	}

	err = errGroup.Wait()
	if err != nil {
		reqLogger.Error("some disconnect notifications failed to send", "context", map[string]string{
			"error":        err.Error(),
			"execution_id": executionID,
		})
	} else {
		reqLogger.Info("all disconnect notifications sent successfully",
			"context", map[string]string{
				"execution_id":     executionID,
				"connection_count": fmt.Sprintf("%d", len(m.connectionIDs)),
			},
		)
	}

	return nil
}

// sendDisconnectToConnection sends a disconnect message to a single WebSocket connection.
func (m *Manager) sendDisconnectToConnection(
	ctx context.Context,
	reqLogger *slog.Logger,
	connectionID string,
	message []byte,
) error {
	reqLogger.Debug("sending disconnect notification to connection", "context", map[string]string{
		"connection_id": connectionID,
	})

	_, err := m.apiGwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         message,
	})

	if err != nil {
		reqLogger.Error("failed to send disconnect notification to connection",
			"context", map[string]string{
				"error":         err.Error(),
				"connection_id": connectionID,
			},
		)
		return fmt.Errorf("failed to send disconnect notification to connection %s: %w", connectionID, err)
	}

	reqLogger.Debug("disconnect notification sent to connection", "context", map[string]string{
		"connection_id": connectionID,
	})
	return nil
}

// GenerateWebSocketURL creates a WebSocket token and returns the connection URL.
// It stores the token for validation when the client connects.
func (m *Manager) GenerateWebSocketURL(
	ctx context.Context,
	executionID string,
	userEmail *string,
	clientIPAtCreationTime *string,
) string {
	reqLogger := m.deriveLogger(ctx)

	token, tokenGenErr := auth.GenerateSecretToken()
	if tokenGenErr != nil {
		reqLogger.Error("failed to generate websocket token",
			"error", tokenGenErr,
			"execution_id", executionID)
		return ""
	}

	expiresAt := time.Now().Add(constants.ConnectionTTLHours * time.Hour).Unix()
	var email string
	if userEmail != nil {
		email = *userEmail
	}
	var clientIP string
	if clientIPAtCreationTime != nil {
		clientIP = *clientIPAtCreationTime
	}

	wsToken := &api.WebSocketToken{
		Token:       token,
		ExecutionID: executionID,
		UserEmail:   email,
		ClientIP:    clientIP,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now().Unix(),
	}

	if tokenErr := m.tokenRepo.CreateToken(ctx, wsToken); tokenErr != nil {
		reqLogger.Error("failed to store websocket token",
			"error", tokenErr,
			"execution_id", executionID)
		return ""
	}

	wsURL := "wss://" + *m.apiGwEndpoint
	return fmt.Sprintf("%s?execution_id=%s&token=%s", wsURL, executionID, token)
}
