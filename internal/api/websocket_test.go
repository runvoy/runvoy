package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSocketConnectionJSON(t *testing.T) {
	t.Run("marshal and unmarshal with all fields", func(t *testing.T) {
		conn := WebSocketConnection{
			ConnectionID:         "conn-123",
			ExecutionID:          "exec-456",
			Functionality:        "logs",
			ExpiresAt:            1234567890,
			LastEventID:          "event-1",
			ClientIP:             "192.168.1.1",
			Token:                "token-abc",
			UserEmail:            "user@example.com",
			TokenRequestClientIP: "192.168.1.2",
		}

		data, err := json.Marshal(conn)
		require.NoError(t, err)

		var unmarshaled WebSocketConnection
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, conn.ConnectionID, unmarshaled.ConnectionID)
		assert.Equal(t, conn.ExecutionID, unmarshaled.ExecutionID)
		assert.Equal(t, conn.Functionality, unmarshaled.Functionality)
		assert.Equal(t, conn.ExpiresAt, unmarshaled.ExpiresAt)
		assert.Equal(t, conn.LastEventID, unmarshaled.LastEventID)
		assert.Equal(t, conn.ClientIP, unmarshaled.ClientIP)
		assert.Equal(t, conn.Token, unmarshaled.Token)
		assert.Equal(t, conn.UserEmail, unmarshaled.UserEmail)
		assert.Equal(t, conn.TokenRequestClientIP, unmarshaled.TokenRequestClientIP)
	})

	t.Run("omit optional fields", func(t *testing.T) {
		conn := WebSocketConnection{
			ConnectionID:  "conn-123",
			ExecutionID:   "exec-456",
			Functionality: "logs",
			ExpiresAt:     1234567890,
		}

		data, err := json.Marshal(conn)
		require.NoError(t, err)

		jsonStr := string(data)
		assert.NotContains(t, jsonStr, "client_ip")
		assert.NotContains(t, jsonStr, "token")
		assert.NotContains(t, jsonStr, "user_email")
		assert.NotContains(t, jsonStr, "last_event_id")
	})
}

func TestWebSocketTokenJSON(t *testing.T) {
	t.Run("marshal and unmarshal with all fields", func(t *testing.T) {
		token := WebSocketToken{
			Token:       "token-123",
			ExecutionID: "exec-456",
			UserEmail:   "user@example.com",
			ClientIP:    "192.168.1.1",
			ExpiresAt:   1234567890,
			CreatedAt:   1234567000,
		}

		data, err := json.Marshal(token)
		require.NoError(t, err)

		var unmarshaled WebSocketToken
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, token.Token, unmarshaled.Token)
		assert.Equal(t, token.ExecutionID, unmarshaled.ExecutionID)
		assert.Equal(t, token.UserEmail, unmarshaled.UserEmail)
		assert.Equal(t, token.ClientIP, unmarshaled.ClientIP)
		assert.Equal(t, token.ExpiresAt, unmarshaled.ExpiresAt)
		assert.Equal(t, token.CreatedAt, unmarshaled.CreatedAt)
	})

	t.Run("omit optional fields", func(t *testing.T) {
		token := WebSocketToken{
			Token:       "token-123",
			ExecutionID: "exec-456",
			ExpiresAt:   1234567890,
			CreatedAt:   1234567000,
		}

		data, err := json.Marshal(token)
		require.NoError(t, err)

		jsonStr := string(data)
		assert.NotContains(t, jsonStr, "user_email")
		assert.NotContains(t, jsonStr, "client_ip")
	})
}

func TestWebSocketMessageTypes(t *testing.T) {
	t.Run("message type constants", func(t *testing.T) {
		assert.Equal(t, WebSocketMessageType("log"), WebSocketMessageTypeLog)
		assert.Equal(t, WebSocketMessageType("disconnect"), WebSocketMessageTypeDisconnect)
	})

	t.Run("disconnect reason constants", func(t *testing.T) {
		assert.Equal(t, WebSocketDisconnectReason("execution_completed"), WebSocketDisconnectReasonExecutionCompleted)
	})
}

func TestWebSocketMessageJSON(t *testing.T) {
	t.Run("marshal and unmarshal log message", func(t *testing.T) {
		timestamp := int64(1234567890)
		message := "Test log message"
		msg := WebSocketMessage{
			Type:      WebSocketMessageTypeLog,
			Message:   &message,
			Timestamp: &timestamp,
		}

		data, err := json.Marshal(msg)
		require.NoError(t, err)

		var unmarshaled WebSocketMessage
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, msg.Type, unmarshaled.Type)
		require.NotNil(t, unmarshaled.Message)
		assert.Equal(t, *msg.Message, *unmarshaled.Message)
		require.NotNil(t, unmarshaled.Timestamp)
		assert.Equal(t, *msg.Timestamp, *unmarshaled.Timestamp)
		assert.Nil(t, unmarshaled.Reason)
	})

	t.Run("marshal and unmarshal disconnect message", func(t *testing.T) {
		reason := WebSocketDisconnectReasonExecutionCompleted
		msg := WebSocketMessage{
			Type:   WebSocketMessageTypeDisconnect,
			Reason: &reason,
		}

		data, err := json.Marshal(msg)
		require.NoError(t, err)

		var unmarshaled WebSocketMessage
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, msg.Type, unmarshaled.Type)
		require.NotNil(t, unmarshaled.Reason)
		assert.Equal(t, *msg.Reason, *unmarshaled.Reason)
		assert.Nil(t, unmarshaled.Message)
		assert.Nil(t, unmarshaled.Timestamp)
	})

	t.Run("omit optional fields", func(t *testing.T) {
		msg := WebSocketMessage{
			Type: WebSocketMessageTypeLog,
		}

		data, err := json.Marshal(msg)
		require.NoError(t, err)

		jsonStr := string(data)
		assert.NotContains(t, jsonStr, "reason")
		assert.NotContains(t, jsonStr, "message")
		assert.NotContains(t, jsonStr, "timestamp")
	})
}
