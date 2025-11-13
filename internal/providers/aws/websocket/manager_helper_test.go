package aws

import (
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
)

func TestGetClientIPFromWebSocketRequest(t *testing.T) {
	tests := []struct {
		name     string
		req      *events.APIGatewayWebsocketProxyRequest
		expected string
	}{
		{
			name: "request with source IP",
			req: &events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					Identity: events.APIGatewayRequestIdentity{
						SourceIP: "192.168.1.1",
					},
				},
			},
			expected: "192.168.1.1",
		},
		{
			name: "request without source IP",
			req: &events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					Identity: events.APIGatewayRequestIdentity{
						SourceIP: "",
					},
				},
			},
			expected: "",
		},
		{
			name: "request with IPv6 address",
			req: &events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					Identity: events.APIGatewayRequestIdentity{
						SourceIP: "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
					},
				},
			},
			expected: "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getClientIPFromWebSocketRequest(tt.req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewWebSocketConnection(t *testing.T) {
	req := &events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "conn-123",
			Identity: events.APIGatewayRequestIdentity{
				SourceIP: "10.0.0.1",
			},
		},
		QueryStringParameters: map[string]string{
			"execution_id": "exec-456",
		},
	}

	token := "test-token-abc"
	wsToken := &api.WebSocketToken{
		Token:       token,
		ExecutionID: "exec-456",
		UserEmail:   "user@example.com",
		ClientIP:    "192.168.1.1",
		ExpiresAt:   time.Now().Add(1 * time.Hour).Unix(),
		CreatedAt:   time.Now().Unix(),
	}

	m := &Manager{}
	connection := m.newWebSocketConnection(req, token, wsToken)

	assert.NotNil(t, connection)
	assert.Equal(t, "conn-123", connection.ConnectionID)
	assert.Equal(t, "exec-456", connection.ExecutionID)
	assert.Equal(t, constants.FunctionalityLogStreaming, connection.Functionality)
	assert.Equal(t, token, connection.Token)
	assert.Equal(t, "10.0.0.1", connection.ClientIP)
	assert.Equal(t, "user@example.com", connection.UserEmail)
	assert.Equal(t, "192.168.1.1", connection.TokenRequestClientIP)
	assert.Greater(t, connection.ExpiresAt, time.Now().Unix())
}

func TestNewWebSocketConnection_WithNilUserEmail(t *testing.T) {
	req := &events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "conn-789",
			Identity: events.APIGatewayRequestIdentity{
				SourceIP: "10.0.0.2",
			},
		},
		QueryStringParameters: map[string]string{
			"execution_id": "exec-999",
		},
	}

	token := "test-token-xyz"
	wsToken := &api.WebSocketToken{
		Token:       token,
		ExecutionID: "exec-999",
		UserEmail:   "", // Empty email
		ClientIP:    "",
		ExpiresAt:   time.Now().Add(1 * time.Hour).Unix(),
		CreatedAt:   time.Now().Unix(),
	}

	m := &Manager{}
	connection := m.newWebSocketConnection(req, token, wsToken)

	assert.NotNil(t, connection)
	assert.Equal(t, "conn-789", connection.ConnectionID)
	assert.Equal(t, "exec-999", connection.ExecutionID)
	assert.Equal(t, "", connection.UserEmail)
	assert.Equal(t, "", connection.TokenRequestClientIP)
}

func TestNewWebSocketConnection_ExpiresAt(t *testing.T) {
	req := &events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "conn-expiry",
		},
		QueryStringParameters: map[string]string{
			"execution_id": "exec-expiry",
		},
	}

	wsToken := &api.WebSocketToken{
		Token:       "token",
		ExecutionID: "exec-expiry",
		ExpiresAt:   time.Now().Add(1 * time.Hour).Unix(),
		CreatedAt:   time.Now().Unix(),
	}

	m := &Manager{}
	connection := m.newWebSocketConnection(req, "token", wsToken)

	// ExpiresAt should be approximately TTL hours from now
	expectedExpiry := time.Now().Add(constants.ConnectionTTLHours * time.Hour).Unix()
	// Allow 5 second tolerance for test execution time
	assert.InDelta(t, expectedExpiry, connection.ExpiresAt, 5, "ExpiresAt should be approximately TTL hours from now")
}
