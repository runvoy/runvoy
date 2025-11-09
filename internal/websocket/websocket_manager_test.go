package websocket

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	"runvoy/internal/testutil"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConnectionRepoForWS implements database.ConnectionRepository for testing.
type mockConnectionRepoForWS struct {
	createConnectionFunc            func(context.Context, *api.WebSocketConnection) error
	updateConnectionFunc            func(context.Context, *api.WebSocketConnection) error
	deleteConnectionsFunc           func(context.Context, []string) (int, error)
	getConnectionsByExecutionIDFunc func(context.Context, string) ([]*api.WebSocketConnection, error)
}

// mockLogRepoForWS implements database.LogRepository for testing.
type mockLogRepoForWS struct {
	getLogsByExecutionIDFunc      func(context.Context, string) ([]api.LogEvent, error)
	getLogsByExecutionIDSinceFunc func(context.Context, string, *int64) ([]api.LogEvent, error)
}

func (m *mockLogRepoForWS) GetLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error) {
	if m.getLogsByExecutionIDFunc != nil {
		return m.getLogsByExecutionIDFunc(ctx, executionID)
	}
	return []api.LogEvent{}, nil
}

func (m *mockLogRepoForWS) GetLogsByExecutionIDSince(
	ctx context.Context, executionID string, sinceTimestampMS *int64) ([]api.LogEvent, error) {
	if m.getLogsByExecutionIDSinceFunc != nil {
		return m.getLogsByExecutionIDSinceFunc(ctx, executionID, sinceTimestampMS)
	}
	return []api.LogEvent{}, nil
}

func (m *mockConnectionRepoForWS) CreateConnection(ctx context.Context, conn *api.WebSocketConnection) error {
	if m.createConnectionFunc != nil {
		return m.createConnectionFunc(ctx, conn)
	}
	return nil
}

func (m *mockConnectionRepoForWS) UpdateConnection(ctx context.Context, conn *api.WebSocketConnection) error {
	if m.updateConnectionFunc != nil {
		return m.updateConnectionFunc(ctx, conn)
	}
	return nil
}

func (m *mockConnectionRepoForWS) DeleteConnections(ctx context.Context, connIDs []string) (int, error) {
	if m.deleteConnectionsFunc != nil {
		return m.deleteConnectionsFunc(ctx, connIDs)
	}
	return len(connIDs), nil
}

func (m *mockConnectionRepoForWS) GetConnectionsByExecutionID(
	ctx context.Context, executionID string,
) ([]*api.WebSocketConnection, error) {
	if m.getConnectionsByExecutionIDFunc != nil {
		return m.getConnectionsByExecutionIDFunc(ctx, executionID)
	}
	return nil, nil
}

func TestValidateConnectionParams(t *testing.T) {
	tests := []struct {
		name          string
		connectionID  string
		executionID   string
		token         string
		expectedCode  int
		expectedError bool
	}{
		{
			name:         "valid params",
			connectionID: "conn-123",
			executionID:  "exec-456",
			token:        "token-789",
		},
		{
			name:          "missing connection_id",
			connectionID:  "",
			executionID:   "exec-456",
			token:         "token-789",
			expectedCode:  http.StatusBadRequest,
			expectedError: true,
		},
		{
			name:          "missing execution_id",
			connectionID:  "conn-123",
			executionID:   "",
			token:         "token-789",
			expectedCode:  http.StatusBadRequest,
			expectedError: true,
		},
		{
			name:          "missing token",
			connectionID:  "conn-123",
			executionID:   "exec-456",
			token:         "",
			expectedCode:  http.StatusUnauthorized,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wm := &WebSocketManager{
				logRepo: &mockLogRepoForWS{},
				logger:  testutil.SilentLogger(),
			}

			resp := wm.validateConnectionParams(tt.connectionID, tt.executionID, tt.token)

			if tt.expectedError {
				require.NotNil(t, resp)
				assert.Equal(t, tt.expectedCode, resp.StatusCode)
			} else {
				assert.Nil(t, resp)
			}
		})
	}
}

func TestValidateAndConsumePendingToken(t *testing.T) {
	validToken := "valid-token-123"
	validPendingConn := &api.WebSocketConnection{
		ConnectionID:       "pending_exec-456",
		ExecutionID:        "exec-456",
		Token:              validToken,
		Functionality:      constants.FunctionalityLogStreaming,
		ExpiresAt:          9999999999,
		UserEmail:          "test@example.com",
		ClientIPAtLogsTime: "192.168.1.1",
	}

	tests := []struct {
		name              string
		executionID       string
		token             string
		mockConnections   []*api.WebSocketConnection
		mockGetErr        error
		mockDeleteErr     error
		expectedConnFound bool
		expectedErrCode   int
	}{
		{
			name:              "valid token",
			executionID:       "exec-456",
			token:             validToken,
			mockConnections:   []*api.WebSocketConnection{validPendingConn},
			expectedConnFound: true,
		},
		{
			name:            "invalid token",
			executionID:     "exec-456",
			token:           "wrong-token",
			mockConnections: []*api.WebSocketConnection{validPendingConn},
			expectedErrCode: http.StatusUnauthorized,
		},
		{
			name:            "no connections",
			executionID:     "exec-456",
			token:           validToken,
			mockConnections: []*api.WebSocketConnection{},
			expectedErrCode: http.StatusUnauthorized,
		},
		{
			name:            "database error on get",
			executionID:     "exec-456",
			token:           validToken,
			mockGetErr:      fmt.Errorf("database error"),
			expectedErrCode: http.StatusInternalServerError,
		},
		{
			name:            "database error on delete",
			executionID:     "exec-456",
			token:           validToken,
			mockConnections: []*api.WebSocketConnection{validPendingConn},
			mockDeleteErr:   fmt.Errorf("delete failed"),
			expectedErrCode: http.StatusInternalServerError,
		},
		{
			name:        "multiple connections, match second",
			executionID: "exec-456",
			token:       validToken,
			mockConnections: []*api.WebSocketConnection{
				{
					ConnectionID:       "pending_exec-999",
					ExecutionID:        "exec-456",
					Token:              "other-token",
					UserEmail:          "other@example.com",
					ClientIPAtLogsTime: "203.0.113.1",
				},
				validPendingConn,
			},
			expectedConnFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockConnectionRepoForWS{
				getConnectionsByExecutionIDFunc: func(_ context.Context, _ string) ([]*api.WebSocketConnection, error) {
					return tt.mockConnections, tt.mockGetErr
				},
				deleteConnectionsFunc: func(_ context.Context, _ []string) (int, error) {
					if tt.mockDeleteErr != nil {
						return 0, tt.mockDeleteErr
					}
					return 1, nil
				},
			}

			wm := &WebSocketManager{
				connRepo: mockRepo,
				logRepo:  &mockLogRepoForWS{},
				logger:   testutil.SilentLogger(),
			}

			ctx := context.Background()
			conn, errResp := wm.validateAndConsumePendingToken(ctx, tt.executionID, tt.token)

			if tt.expectedConnFound {
				assert.Nil(t, errResp)
				require.NotNil(t, conn)
				assert.Equal(t, validPendingConn.ConnectionID, conn.ConnectionID)
				assert.Equal(t, validToken, conn.Token)
			} else {
				require.NotNil(t, errResp)
				assert.Equal(t, tt.expectedErrCode, errResp.StatusCode)
				assert.Nil(t, conn)
			}
		})
	}
}

func TestHandleConnect(t *testing.T) {
	validToken := "valid-token-abc"
	validPendingConn := &api.WebSocketConnection{
		ConnectionID:       "pending_exec-123",
		ExecutionID:        "exec-123",
		Token:              validToken,
		Functionality:      constants.FunctionalityLogStreaming,
		ExpiresAt:          9999999999,
		UserEmail:          "alice@example.com",
		ClientIPAtLogsTime: "10.0.0.1",
	}

	tests := []struct {
		name               string
		req                events.APIGatewayWebsocketProxyRequest
		mockConnections    []*api.WebSocketConnection
		mockGetErr         error
		mockDeleteErr      error
		mockCreateErr      error
		expectedStatusCode int
		expectedSuccess    bool
	}{
		{
			name: "successful connection",
			req: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "real-conn-id",
				},
				QueryStringParameters: map[string]string{
					"execution_id": "exec-123",
					"token":        validToken,
				},
			},
			mockConnections:    []*api.WebSocketConnection{validPendingConn},
			expectedStatusCode: http.StatusOK,
			expectedSuccess:    true,
		},
		{
			name: "missing connection_id",
			req: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "",
				},
				QueryStringParameters: map[string]string{
					"execution_id": "exec-123",
					"token":        validToken,
				},
			},
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "missing execution_id",
			req: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "real-conn-id",
				},
				QueryStringParameters: map[string]string{
					"execution_id": "",
					"token":        validToken,
				},
			},
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "missing token",
			req: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "real-conn-id",
				},
				QueryStringParameters: map[string]string{
					"execution_id": "exec-123",
					"token":        "",
				},
			},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name: "invalid token",
			req: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "real-conn-id",
				},
				QueryStringParameters: map[string]string{
					"execution_id": "exec-123",
					"token":        "wrong-token",
				},
			},
			mockConnections:    []*api.WebSocketConnection{validPendingConn},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name: "database error getting connections",
			req: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "real-conn-id",
				},
				QueryStringParameters: map[string]string{
					"execution_id": "exec-123",
					"token":        validToken,
				},
			},
			mockGetErr:         fmt.Errorf("database error"),
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			name: "database error deleting pending connection",
			req: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "real-conn-id",
				},
				QueryStringParameters: map[string]string{
					"execution_id": "exec-123",
					"token":        validToken,
				},
			},
			mockConnections:    []*api.WebSocketConnection{validPendingConn},
			mockDeleteErr:      fmt.Errorf("delete failed"),
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			name: "database error creating real connection",
			req: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "real-conn-id",
				},
				QueryStringParameters: map[string]string{
					"execution_id": "exec-123",
					"token":        validToken,
				},
			},
			mockConnections:    []*api.WebSocketConnection{validPendingConn},
			mockCreateErr:      fmt.Errorf("create failed"),
			expectedStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var createdConn *api.WebSocketConnection

			mockRepo := &mockConnectionRepoForWS{
				getConnectionsByExecutionIDFunc: func(_ context.Context, _ string) ([]*api.WebSocketConnection, error) {
					if tt.mockGetErr != nil {
						return nil, tt.mockGetErr
					}
					// Return pending connection, plus created connection if it was created
					result := make([]*api.WebSocketConnection, len(tt.mockConnections))
					copy(result, tt.mockConnections)
					if createdConn != nil {
						result = append(result, createdConn)
					}
					return result, nil
				},
				deleteConnectionsFunc: func(_ context.Context, _ []string) (int, error) {
					if tt.mockDeleteErr != nil {
						return 0, tt.mockDeleteErr
					}
					return 1, nil
				},
				createConnectionFunc: func(_ context.Context, conn *api.WebSocketConnection) error {
					if tt.mockCreateErr == nil {
						createdConn = conn
					}
					return tt.mockCreateErr
				},
				updateConnectionFunc: func(_ context.Context, conn *api.WebSocketConnection) error {
					// Update tracking if it's the created connection
					if createdConn != nil && conn.ConnectionID == createdConn.ConnectionID {
						createdConn = conn
					}
					return nil
				},
			}

			wm := &WebSocketManager{
				connRepo: mockRepo,
				logRepo:  &mockLogRepoForWS{},
				logger:   testutil.SilentLogger(),
			}

			ctx := context.Background()
			resp, err := wm.handleConnect(ctx, tt.req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatusCode, resp.StatusCode)

			if tt.expectedSuccess {
				assert.Equal(t, "Connected", resp.Body)
			}
		})
	}
}

func TestHandleConnect_PreservesPendingConnectionExpiry(t *testing.T) {
	// Verify that the real connection reuses the pending connection's ExpiresAt
	validToken := "token-xyz"
	originalExpiresAt := int64(1234567890)

	pendingConn := &api.WebSocketConnection{
		ConnectionID:       "pending_exec-789",
		ExecutionID:        "exec-789",
		Token:              validToken,
		Functionality:      constants.FunctionalityLogStreaming,
		ExpiresAt:          originalExpiresAt,
		UserEmail:          "bob@example.com",
		ClientIPAtLogsTime: "172.16.0.1",
	}

	var createdConnection *api.WebSocketConnection

	mockRepo := &mockConnectionRepoForWS{
		getConnectionsByExecutionIDFunc: func(_ context.Context, _ string) ([]*api.WebSocketConnection, error) {
			result := []*api.WebSocketConnection{pendingConn}
			// Also return the created connection if it was created
			if createdConnection != nil {
				result = append(result, createdConnection)
			}
			return result, nil
		},
		deleteConnectionsFunc: func(_ context.Context, _ []string) (int, error) {
			return 1, nil
		},
		createConnectionFunc: func(_ context.Context, conn *api.WebSocketConnection) error {
			createdConnection = conn
			return nil
		},
		updateConnectionFunc: func(_ context.Context, conn *api.WebSocketConnection) error {
			// Update tracking if it's the created connection
			if createdConnection != nil && conn.ConnectionID == createdConnection.ConnectionID {
				createdConnection = conn
			}
			return nil
		},
	}

	wm := &WebSocketManager{
		connRepo: mockRepo,
		logRepo:  &mockLogRepoForWS{},
		logger:   testutil.SilentLogger(),
	}

	req := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "real-conn-id",
		},
		QueryStringParameters: map[string]string{
			"execution_id": "exec-789",
			"token":        validToken,
		},
	}

	ctx := context.Background()
	resp, err := wm.handleConnect(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, createdConnection)
	assert.Equal(t, originalExpiresAt, createdConnection.ExpiresAt)
	assert.Equal(t, "real-conn-id", createdConnection.ConnectionID)
	assert.Equal(t, "exec-789", createdConnection.ExecutionID)
}
