package websocket

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"runvoy/internal/api"
	"runvoy/internal/testutil"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConnectionRepoForWS implements database.ConnectionRepository for testing.
type mockConnectionRepoForWS struct {
	createConnectionFunc            func(context.Context, *api.WebSocketConnection) error
	deleteConnectionsFunc           func(context.Context, []string) (int, error)
	getConnectionsByExecutionIDFunc func(context.Context, string) ([]*api.WebSocketConnection, error)
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

// mockTokenRepoForWS implements database.TokenRepository for testing.
type mockTokenRepoForWS struct {
	createTokenFunc func(context.Context, *api.WebSocketToken) error
	getTokenFunc    func(context.Context, string) (*api.WebSocketToken, error)
	deleteTokenFunc func(context.Context, string) error
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
			wm := &WebSocketManager{logger: testutil.SilentLogger()}

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
			mockGetErr:         fmt.Errorf("database error"),
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
			mockCreateErr:      fmt.Errorf("create failed"),
			expectedStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a WebSocketToken from the pending connection data
			wsToken := &api.WebSocketToken{
				Token:                  validToken,
				ExecutionID:            "exec-123",
				UserEmail:              "alice@example.com",
				ClientIPAtCreationTime: "10.0.0.1",
				ExpiresAt:              9999999999,
				CreatedAt:              1234567890,
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

			wm := &WebSocketManager{
				connRepo:  mockConnRepo,
				tokenRepo: mockTokenRepo,
				logger:    testutil.SilentLogger(),
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

func TestHandleConnect_UsesTokenMetadata(t *testing.T) {
	// Verify that the real connection uses metadata from the WebSocket token
	validToken := "token-xyz"
	tokenExpiresAt := int64(1234567890)

	wsToken := &api.WebSocketToken{
		Token:                  validToken,
		ExecutionID:            "exec-789",
		UserEmail:              "bob@example.com",
		ClientIPAtCreationTime: "172.16.0.1",
		ExpiresAt:              tokenExpiresAt,
		CreatedAt:              1234567800,
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

	wm := &WebSocketManager{
		connRepo:  mockConnRepo,
		tokenRepo: mockTokenRepo,
		logger:    testutil.SilentLogger(),
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
	// Real connection should use metadata from token
	assert.Equal(t, "bob@example.com", createdConnection.UserEmail)
	assert.Equal(t, "172.16.0.1", createdConnection.ClientIPAtCreationTime)
	assert.Equal(t, "real-conn-id", createdConnection.ConnectionID)
	assert.Equal(t, "exec-789", createdConnection.ExecutionID)
	assert.Equal(t, validToken, createdConnection.Token) // Token is stored in connection for cleanup
}
