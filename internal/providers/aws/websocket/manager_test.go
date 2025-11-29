package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/testutil"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConnectionRepoForWS implements database.ConnectionRepository for testing.
type mockConnectionRepoForWS struct {
	createConnectionFunc            func(context.Context, *api.WebSocketConnection) error
	deleteConnectionsFunc           func(context.Context, []string) (int, error)
	getConnectionsByExecutionIDFunc func(context.Context, string) ([]*api.WebSocketConnection, error)
	updateLastEventIDFunc           func(context.Context, string, string) error
}

func (m *mockConnectionRepoForWS) CreateConnection(ctx context.Context, conn *api.WebSocketConnection) error {
	if m.createConnectionFunc != nil {
		return m.createConnectionFunc(ctx, conn)
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

func (m *mockConnectionRepoForWS) UpdateLastEventID(ctx context.Context, connectionID, lastEventID string) error {
	if m.updateLastEventIDFunc != nil {
		return m.updateLastEventIDFunc(ctx, connectionID, lastEventID)
	}
	return nil
}

// mockTokenRepoForWS implements database.TokenRepository for testing.
type mockTokenRepoForWS struct {
	createTokenFunc func(context.Context, *api.WebSocketToken) error
	getTokenFunc    func(context.Context, string) (*api.WebSocketToken, error)
	deleteTokenFunc func(context.Context, string) error
}

type mockLogEventRepoForWS struct {
	saveLogEventsFunc   func(context.Context, string, []api.LogEvent) error
	listLogEventsFunc   func(context.Context, string) ([]api.LogEvent, error)
	deleteLogEventsFunc func(context.Context, string) error
}

func (m *mockLogEventRepoForWS) SaveLogEvents(ctx context.Context, executionID string, logEvents []api.LogEvent) error {
	if m.saveLogEventsFunc != nil {
		return m.saveLogEventsFunc(ctx, executionID, logEvents)
	}
	return nil
}

func (m *mockLogEventRepoForWS) ListLogEvents(ctx context.Context, executionID string) ([]api.LogEvent, error) {
	if m.listLogEventsFunc != nil {
		return m.listLogEventsFunc(ctx, executionID)
	}
	return nil, nil
}

func (m *mockLogEventRepoForWS) DeleteLogEvents(ctx context.Context, executionID string) error {
	if m.deleteLogEventsFunc != nil {
		return m.deleteLogEventsFunc(ctx, executionID)
	}
	return nil
}

func messageListContains(messages []string, fragment string) bool {
	for _, msg := range messages {
		if strings.Contains(msg, fragment) {
			return true
		}
	}

	return false
}

func (m *mockTokenRepoForWS) CreateToken(ctx context.Context, token *api.WebSocketToken) error {
	if m.createTokenFunc != nil {
		return m.createTokenFunc(ctx, token)
	}
	return nil
}

func (m *mockTokenRepoForWS) GetToken(ctx context.Context, tokenValue string) (*api.WebSocketToken, error) {
	if m.getTokenFunc != nil {
		return m.getTokenFunc(ctx, tokenValue)
	}
	return nil, nil
}

func (m *mockTokenRepoForWS) DeleteToken(ctx context.Context, tokenValue string) error {
	if m.deleteTokenFunc != nil {
		return m.deleteTokenFunc(ctx, tokenValue)
	}
	return nil
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
			reqLogger := testutil.SilentLogger()
			wm := &Manager{logger: reqLogger}

			resp := wm.validateConnectionParams(reqLogger, tt.connectionID, tt.executionID, tt.token)

			if tt.expectedError {
				require.NotNil(t, resp)
				assert.Equal(t, tt.expectedCode, resp.StatusCode)
			} else {
				assert.Nil(t, resp)
			}
		})
	}
}

func TestHandleConnect(t *testing.T) {
	validToken := "valid-token-abc"

	tests := []struct {
		name               string
		req                events.APIGatewayWebsocketProxyRequest
		mockGetErr         error
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
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name: "database error getting token",
			req: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "real-conn-id",
				},
				QueryStringParameters: map[string]string{
					"execution_id": "exec-123",
					"token":        validToken,
				},
			},
			mockGetErr:         errors.New("database error"),
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
			mockCreateErr:      errors.New("create failed"),
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			name: "execution ID mismatch",
			req: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "real-conn-id",
				},
				QueryStringParameters: map[string]string{
					"execution_id": "exec-456",
					"token":        validToken,
				},
			},
			expectedStatusCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqLogger := testutil.SilentLogger()
			// Create a WebSocketToken from the pending connection data
			wsToken := &api.WebSocketToken{
				Token:       validToken,
				ExecutionID: "exec-123",
				UserEmail:   "alice@example.com",
				ClientIP:    "10.0.0.1",
				ExpiresAt:   9999999999,
				CreatedAt:   1234567890,
			}

			mockConnRepo := &mockConnectionRepoForWS{
				createConnectionFunc: func(_ context.Context, _ *api.WebSocketConnection) error {
					return tt.mockCreateErr
				},
			}

			mockTokenRepo := &mockTokenRepoForWS{
				getTokenFunc: func(_ context.Context, token string) (*api.WebSocketToken, error) {
					// If test expects an error getting the token, return it
					if tt.mockGetErr != nil {
						return nil, tt.mockGetErr
					}
					if token == wsToken.Token {
						return wsToken, nil
					}
					return nil, nil // Token not found
				},
			}

			wm := &Manager{
				connRepo:  mockConnRepo,
				tokenRepo: mockTokenRepo,
				logger:    testutil.SilentLogger(),
			}

			ctx := context.Background()
			resp, err := wm.handleConnect(ctx, reqLogger, tt.req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatusCode, resp.StatusCode)

			if tt.expectedSuccess {
				assert.Equal(t, "Connected", resp.Body)
			}
		})
	}
}

func TestHandleConnect_UsesTokenMetadata(t *testing.T) {
	// Verify that the real connection uses metadata from the WebSocket token
	validToken := "token-xyz"
	tokenExpiresAt := int64(1234567890)

	wsToken := &api.WebSocketToken{
		Token:       validToken,
		ExecutionID: "exec-789",
		UserEmail:   "bob@example.com",
		ClientIP:    "172.16.0.1",
		ExpiresAt:   tokenExpiresAt,
		CreatedAt:   1234567800,
	}

	var createdConnection *api.WebSocketConnection

	mockConnRepo := &mockConnectionRepoForWS{
		createConnectionFunc: func(_ context.Context, conn *api.WebSocketConnection) error {
			createdConnection = conn
			return nil
		},
	}

	mockTokenRepo := &mockTokenRepoForWS{
		getTokenFunc: func(_ context.Context, token string) (*api.WebSocketToken, error) {
			if token == validToken {
				return wsToken, nil
			}
			return nil, nil
		},
	}

	reqLogger := testutil.SilentLogger()
	wm := &Manager{
		connRepo:  mockConnRepo,
		tokenRepo: mockTokenRepo,
		logger:    reqLogger,
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
	resp, err := wm.handleConnect(ctx, reqLogger, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, createdConnection)
	// Real connection should use metadata from token
	assert.Equal(t, "bob@example.com", createdConnection.UserEmail)
	assert.Equal(t, "172.16.0.1", createdConnection.TokenRequestClientIP)
	assert.Equal(t, "real-conn-id", createdConnection.ConnectionID)
	assert.Equal(t, "exec-789", createdConnection.ExecutionID)
	assert.Equal(t, validToken, createdConnection.Token) // Token is stored in connection for cleanup
}

type mockAPIGatewayClient struct {
	postToConnectionFunc func(
		context.Context,
		*apigatewaymanagementapi.PostToConnectionInput,
		...func(*apigatewaymanagementapi.Options),
	) (*apigatewaymanagementapi.PostToConnectionOutput, error)
}

func (m *mockAPIGatewayClient) PostToConnection(
	ctx context.Context,
	params *apigatewaymanagementapi.PostToConnectionInput,
	optFns ...func(*apigatewaymanagementapi.Options),
) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
	if m.postToConnectionFunc != nil {
		return m.postToConnectionFunc(ctx, params, optFns...)
	}
	return &apigatewaymanagementapi.PostToConnectionOutput{}, nil
}

func TestSendLogsToExecution(t *testing.T) {
	ctx := context.Background()
	executionID := "exec-123"
	connectionID1 := "conn-1"
	connectionID2 := "conn-2"

	t.Run("successfully sends logs to connections", func(t *testing.T) {
		connections := []*api.WebSocketConnection{
			{ConnectionID: connectionID1, ExecutionID: executionID, LastEventID: "evt-1"},
			{ConnectionID: connectionID2, ExecutionID: executionID},
		}

		buffered := []api.LogEvent{
			{EventID: "evt-1", Timestamp: time.Now().Unix(), Message: "log message 1"},
			{EventID: "evt-2", Timestamp: time.Now().Unix(), Message: "log message 2"},
		}

		var sentMessages []string
		var updatedConnections []string
		mockClient := &mockAPIGatewayClient{
			postToConnectionFunc: func(
				_ context.Context,
				input *apigatewaymanagementapi.PostToConnectionInput,
				_ ...func(*apigatewaymanagementapi.Options),
			) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
				sentMessages = append(sentMessages, string(input.Data))
				return &apigatewaymanagementapi.PostToConnectionOutput{}, nil
			},
		}

		mockConnRepo := &mockConnectionRepoForWS{
			getConnectionsByExecutionIDFunc: func(
				_ context.Context,
				execID string,
			) ([]*api.WebSocketConnection, error) {
				if execID == executionID {
					return connections, nil
				}
				return nil, nil
			},
			updateLastEventIDFunc: func(_ context.Context, connectionID, lastEventID string) error {
				updatedConnections = append(updatedConnections, fmt.Sprintf("%s:%s", connectionID, lastEventID))
				return nil
			},
		}

		mockLogRepo := &mockLogEventRepoForWS{
			listLogEventsFunc: func(_ context.Context, execID string) ([]api.LogEvent, error) {
				if execID == executionID {
					return buffered, nil
				}
				return nil, nil
			},
		}

		m := &Manager{
			connRepo:     mockConnRepo,
			logEventRepo: mockLogRepo,
			apiGwClient:  mockClient,
			logger:       testutil.SilentLogger(),
		}

		err := m.SendLogsToExecution(ctx, &executionID)

		assert.NoError(t, err)
		assert.Len(t, sentMessages, 3) // conn1 gets events after evt-1, conn2 gets all
		require.True(t, messageListContains(sentMessages, "log message 2"))
		assert.ElementsMatch(t, []string{"conn-1:evt-2", "conn-2:evt-2"}, updatedConnections)
	})

	t.Run("handles nil execution ID", func(t *testing.T) {
		m := &Manager{logger: testutil.SilentLogger()}
		err := m.SendLogsToExecution(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "execution ID is nil or empty")
	})

	t.Run("handles empty execution ID", func(t *testing.T) {
		emptyID := ""
		m := &Manager{logger: testutil.SilentLogger()}
		err := m.SendLogsToExecution(ctx, &emptyID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "execution ID is nil or empty")
	})

	t.Run("handles empty buffered logs", func(t *testing.T) {
		mockConnRepo := &mockConnectionRepoForWS{
			getConnectionsByExecutionIDFunc: func(_ context.Context, _ string) ([]*api.WebSocketConnection, error) {
				return []*api.WebSocketConnection{{ConnectionID: connectionID1, ExecutionID: executionID}}, nil
			},
		}

		mockLogRepo := &mockLogEventRepoForWS{
			listLogEventsFunc: func(context.Context, string) ([]api.LogEvent, error) {
				return []api.LogEvent{}, nil
			},
		}

		m := &Manager{
			connRepo:     mockConnRepo,
			logEventRepo: mockLogRepo,
			apiGwClient:  &mockAPIGatewayClient{},
			logger:       testutil.SilentLogger(),
		}

		err := m.SendLogsToExecution(ctx, &executionID)
		assert.NoError(t, err)
	})

	t.Run("handles no connections", func(t *testing.T) {
		mockConnRepo := &mockConnectionRepoForWS{
			getConnectionsByExecutionIDFunc: func(_ context.Context, _ string) ([]*api.WebSocketConnection, error) {
				return []*api.WebSocketConnection{}, nil
			},
		}

		m := &Manager{
			connRepo: mockConnRepo,
			logger:   testutil.SilentLogger(),
		}

		err := m.SendLogsToExecution(ctx, &executionID)
		assert.NoError(t, err)
	})

	t.Run("handles connection repository error", func(t *testing.T) {
		mockConnRepo := &mockConnectionRepoForWS{
			getConnectionsByExecutionIDFunc: func(_ context.Context, _ string) ([]*api.WebSocketConnection, error) {
				return nil, errors.New("database error")
			},
		}

		m := &Manager{
			connRepo: mockConnRepo,
			logger:   testutil.SilentLogger(),
		}

		err := m.SendLogsToExecution(ctx, &executionID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get connections")
	})

	t.Run("handles buffered log retrieval error", func(t *testing.T) {
		mockConnRepo := &mockConnectionRepoForWS{
			getConnectionsByExecutionIDFunc: func(_ context.Context, _ string) ([]*api.WebSocketConnection, error) {
				return []*api.WebSocketConnection{{ConnectionID: connectionID1, ExecutionID: executionID}}, nil
			},
		}

		mockLogRepo := &mockLogEventRepoForWS{
			listLogEventsFunc: func(context.Context, string) ([]api.LogEvent, error) {
				return nil, errors.New("query failed")
			},
		}

		m := &Manager{
			connRepo:     mockConnRepo,
			logEventRepo: mockLogRepo,
			apiGwClient:  &mockAPIGatewayClient{},
			logger:       testutil.SilentLogger(),
		}

		err := m.SendLogsToExecution(ctx, &executionID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to retrieve buffered logs")
	})

	t.Run("handles PostToConnection error", func(t *testing.T) {
		connections := []*api.WebSocketConnection{
			{ConnectionID: connectionID1, ExecutionID: executionID},
		}

		mockClient := &mockAPIGatewayClient{
			postToConnectionFunc: func(
				_ context.Context,
				_ *apigatewaymanagementapi.PostToConnectionInput,
				_ ...func(*apigatewaymanagementapi.Options),
			) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
				return nil, errors.New("connection gone")
			},
		}

		mockConnRepo := &mockConnectionRepoForWS{
			getConnectionsByExecutionIDFunc: func(
				_ context.Context,
				_ string,
			) ([]*api.WebSocketConnection, error) {
				return connections, nil
			},
		}

		mockLogRepo := &mockLogEventRepoForWS{
			listLogEventsFunc: func(context.Context, string) ([]api.LogEvent, error) {
				return []api.LogEvent{{EventID: "evt-1", Message: "test"}}, nil
			},
		}

		m := &Manager{
			connRepo:     mockConnRepo,
			logEventRepo: mockLogRepo,
			apiGwClient:  mockClient,
			logger:       testutil.SilentLogger(),
		}

		err := m.SendLogsToExecution(ctx, &executionID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send logs to some connections")
	})
}

func TestSendLogToConnection(t *testing.T) {
	ctx := context.Background()
	reqLogger := testutil.SilentLogger()
	connectionID := "conn-123"
	logEvent := api.LogEvent{
		Timestamp: time.Now().Unix(),
		Message:   "test log message",
	}

	t.Run("successfully sends log to connection", func(t *testing.T) {
		var sentData []byte
		mockClient := &mockAPIGatewayClient{
			postToConnectionFunc: func(
				_ context.Context,
				input *apigatewaymanagementapi.PostToConnectionInput,
				_ ...func(*apigatewaymanagementapi.Options),
			) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
				assert.Equal(t, connectionID, *input.ConnectionId)
				sentData = input.Data
				return &apigatewaymanagementapi.PostToConnectionOutput{}, nil
			},
		}

		m := &Manager{
			apiGwClient: mockClient,
			logger:      reqLogger,
		}

		err := m.sendLogToConnection(ctx, reqLogger, connectionID, logEvent)

		assert.NoError(t, err)
		assert.NotNil(t, sentData)

		var receivedEvent api.LogEvent
		err = json.Unmarshal(sentData, &receivedEvent)
		assert.NoError(t, err)
		assert.Equal(t, logEvent.Message, receivedEvent.Message)
	})

	t.Run("handles empty connection ID", func(t *testing.T) {
		m := &Manager{logger: reqLogger}
		err := m.sendLogToConnection(ctx, reqLogger, "", logEvent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection ID is empty")
	})

	t.Run("handles PostToConnection error", func(t *testing.T) {
		mockClient := &mockAPIGatewayClient{
			postToConnectionFunc: func(
				_ context.Context,
				_ *apigatewaymanagementapi.PostToConnectionInput,
				_ ...func(*apigatewaymanagementapi.Options),
			) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
				return nil, errors.New("connection gone")
			},
		}

		m := &Manager{
			apiGwClient: mockClient,
			logger:      reqLogger,
		}

		err := m.sendLogToConnection(ctx, reqLogger, connectionID, logEvent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send log to connection")
	})
}

func TestNotifyExecutionCompletion(t *testing.T) {
	ctx := context.Background()
	executionID := "exec-123"
	connectionID1 := "conn-1"
	connectionID2 := "conn-2"

	t.Run("successfully notifies completion", func(t *testing.T) {
		connections := []*api.WebSocketConnection{
			{ConnectionID: connectionID1, ExecutionID: executionID},
			{ConnectionID: connectionID2, ExecutionID: executionID},
		}

		var sentMessages []string
		mockClient := &mockAPIGatewayClient{
			postToConnectionFunc: func(
				_ context.Context,
				input *apigatewaymanagementapi.PostToConnectionInput,
				_ ...func(*apigatewaymanagementapi.Options),
			) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
				sentMessages = append(sentMessages, string(input.Data))
				return &apigatewaymanagementapi.PostToConnectionOutput{}, nil
			},
		}

		mockConnRepo := &mockConnectionRepoForWS{
			getConnectionsByExecutionIDFunc: func(
				_ context.Context,
				_ string,
			) ([]*api.WebSocketConnection, error) {
				return connections, nil
			},
			deleteConnectionsFunc: func(_ context.Context, connIDs []string) (int, error) {
				return len(connIDs), nil
			},
		}

		m := &Manager{
			connRepo:    mockConnRepo,
			apiGwClient: mockClient,
			logger:      testutil.SilentLogger(),
		}

		err := m.NotifyExecutionCompletion(ctx, &executionID)

		assert.NoError(t, err)
		assert.Len(t, sentMessages, 2)

		var disconnectMsg api.WebSocketMessage
		err = json.Unmarshal([]byte(sentMessages[0]), &disconnectMsg)
		assert.NoError(t, err)
		assert.Equal(t, api.WebSocketMessageTypeDisconnect, disconnectMsg.Type)
		assert.NotNil(t, disconnectMsg.Reason)
		assert.Equal(t, api.WebSocketDisconnectReasonExecutionCompleted, *disconnectMsg.Reason)
	})

	t.Run("handles nil execution ID", func(t *testing.T) {
		m := &Manager{logger: testutil.SilentLogger()}
		err := m.NotifyExecutionCompletion(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "execution ID is nil or empty")
	})

	t.Run("handles empty execution ID", func(t *testing.T) {
		emptyID := ""
		m := &Manager{logger: testutil.SilentLogger()}
		err := m.NotifyExecutionCompletion(ctx, &emptyID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "execution ID is nil or empty")
	})

	t.Run("handles no connections", func(t *testing.T) {
		mockConnRepo := &mockConnectionRepoForWS{
			getConnectionsByExecutionIDFunc: func(_ context.Context, _ string) ([]*api.WebSocketConnection, error) {
				return []*api.WebSocketConnection{}, nil
			},
			deleteConnectionsFunc: func(_ context.Context, _ []string) (int, error) {
				return 0, nil
			},
		}

		m := &Manager{
			connRepo: mockConnRepo,
			logger:   testutil.SilentLogger(),
		}

		err := m.NotifyExecutionCompletion(ctx, &executionID)
		assert.NoError(t, err)
	})

	t.Run("handles connection repository error", func(t *testing.T) {
		mockConnRepo := &mockConnectionRepoForWS{
			getConnectionsByExecutionIDFunc: func(_ context.Context, _ string) ([]*api.WebSocketConnection, error) {
				return nil, errors.New("database error")
			},
		}

		m := &Manager{
			connRepo: mockConnRepo,
			logger:   testutil.SilentLogger(),
		}

		err := m.NotifyExecutionCompletion(ctx, &executionID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to notify disconnect")
	})
}

func TestSendDisconnectToConnection(t *testing.T) {
	ctx := context.Background()
	reqLogger := testutil.SilentLogger()
	connectionID := "conn-123"
	message := []byte(`{"type":"disconnect","reason":"execution_completed"}`)

	t.Run("successfully sends disconnect message", func(t *testing.T) {
		var sentData []byte
		mockClient := &mockAPIGatewayClient{
			postToConnectionFunc: func(
				_ context.Context,
				input *apigatewaymanagementapi.PostToConnectionInput,
				_ ...func(*apigatewaymanagementapi.Options),
			) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
				assert.Equal(t, connectionID, *input.ConnectionId)
				sentData = input.Data
				return &apigatewaymanagementapi.PostToConnectionOutput{}, nil
			},
		}

		m := &Manager{
			apiGwClient: mockClient,
			logger:      reqLogger,
		}

		err := m.sendDisconnectToConnection(ctx, reqLogger, connectionID, message)

		assert.NoError(t, err)
		assert.Equal(t, message, sentData)
	})

	t.Run("handles PostToConnection error", func(t *testing.T) {
		mockClient := &mockAPIGatewayClient{
			postToConnectionFunc: func(
				_ context.Context,
				_ *apigatewaymanagementapi.PostToConnectionInput,
				_ ...func(*apigatewaymanagementapi.Options),
			) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
				return nil, errors.New("connection gone")
			},
		}

		m := &Manager{
			apiGwClient: mockClient,
			logger:      reqLogger,
		}

		err := m.sendDisconnectToConnection(ctx, reqLogger, connectionID, message)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send disconnect notification")
	})
}

func TestGenerateWebSocketURL(t *testing.T) {
	ctx := context.Background()
	executionID := "exec-123"
	endpoint := "api.example.com"
	userEmail := "user@example.com"
	clientIP := "10.0.0.1"

	t.Run("successfully generates WebSocket URL", func(t *testing.T) {
		var createdToken *api.WebSocketToken
		mockTokenRepo := &mockTokenRepoForWS{
			createTokenFunc: func(_ context.Context, token *api.WebSocketToken) error {
				createdToken = token
				return nil
			},
		}

		m := &Manager{
			tokenRepo:     mockTokenRepo,
			apiGwEndpoint: &endpoint,
			logger:        testutil.SilentLogger(),
		}

		url := m.GenerateWebSocketURL(ctx, executionID, &userEmail, &clientIP)

		require.NotEmpty(t, url)
		assert.Contains(t, url, "wss://"+endpoint)
		assert.Contains(t, url, "execution_id="+executionID)
		assert.Contains(t, url, "token=")
		require.NotNil(t, createdToken)
		assert.Equal(t, executionID, createdToken.ExecutionID)
		assert.Equal(t, userEmail, createdToken.UserEmail)
		assert.Equal(t, clientIP, createdToken.ClientIP)
		assert.NotEmpty(t, createdToken.Token)
	})

	t.Run("handles nil user email and client IP", func(t *testing.T) {
		mockTokenRepo := &mockTokenRepoForWS{
			createTokenFunc: func(_ context.Context, _ *api.WebSocketToken) error {
				return nil
			},
		}

		m := &Manager{
			tokenRepo:     mockTokenRepo,
			apiGwEndpoint: &endpoint,
			logger:        testutil.SilentLogger(),
		}

		url := m.GenerateWebSocketURL(ctx, executionID, nil, nil)

		require.NotEmpty(t, url)
		assert.Contains(t, url, "wss://"+endpoint)
	})

	t.Run("handles token creation error", func(t *testing.T) {
		mockTokenRepo := &mockTokenRepoForWS{
			createTokenFunc: func(_ context.Context, _ *api.WebSocketToken) error {
				return errors.New("database error")
			},
		}

		m := &Manager{
			tokenRepo:     mockTokenRepo,
			apiGwEndpoint: &endpoint,
			logger:        testutil.SilentLogger(),
		}

		url := m.GenerateWebSocketURL(ctx, executionID, &userEmail, &clientIP)

		assert.Empty(t, url)
	})
}
