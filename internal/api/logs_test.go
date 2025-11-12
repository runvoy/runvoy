// Package api defines the API types and structures used across runvoy.
package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogEventJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		event := LogEvent{
			Timestamp: 1234567890,
			Message:   "Test log message",
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		var unmarshaled LogEvent
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, event.Timestamp, unmarshaled.Timestamp)
		assert.Equal(t, event.Message, unmarshaled.Message)
	})

	t.Run("handle empty message", func(t *testing.T) {
		event := LogEvent{
			Timestamp: 1234567890,
			Message:   "",
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		var unmarshaled LogEvent
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, event.Timestamp, unmarshaled.Timestamp)
		assert.Empty(t, unmarshaled.Message)
	})
}

func TestLogsResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal with WebSocketURL", func(t *testing.T) {
		resp := LogsResponse{
			ExecutionID: "exec-123",
			Events: []LogEvent{
				{Timestamp: 1000, Message: "Log 1"},
				{Timestamp: 2000, Message: "Log 2"},
			},
			Status:       "RUNNING",
			WebSocketURL: "wss://example.com/ws",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled LogsResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.ExecutionID, unmarshaled.ExecutionID)
		assert.Len(t, unmarshaled.Events, 2)
		assert.Equal(t, resp.Status, unmarshaled.Status)
		assert.Equal(t, resp.WebSocketURL, unmarshaled.WebSocketURL)
	})

	t.Run("omit WebSocketURL when empty", func(t *testing.T) {
		resp := LogsResponse{
			ExecutionID: "exec-123",
			Events:      []LogEvent{},
			Status:      "SUCCEEDED",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		jsonStr := string(data)
		assert.NotContains(t, jsonStr, "websocket_url")
	})

	t.Run("handle empty events", func(t *testing.T) {
		resp := LogsResponse{
			ExecutionID: "exec-123",
			Events:      []LogEvent{},
			Status:      "SUCCEEDED",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled LogsResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.ExecutionID, unmarshaled.ExecutionID)
		assert.Empty(t, unmarshaled.Events)
		assert.Equal(t, resp.Status, unmarshaled.Status)
	})

	t.Run("handle nil events", func(t *testing.T) {
		resp := LogsResponse{
			ExecutionID: "exec-123",
			Events:      nil,
			Status:      "SUCCEEDED",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled LogsResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.ExecutionID, unmarshaled.ExecutionID)
		assert.Nil(t, unmarshaled.Events)
	})
}
