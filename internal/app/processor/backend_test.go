// Package events provides event processing interfaces and utilities.
package processor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSocketResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal with all fields", func(t *testing.T) {
		resp := WebSocketResponse{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
				"X-Custom":     "value",
			},
			Body: `{"message": "success"}`,
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled WebSocketResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.StatusCode, unmarshaled.StatusCode)
		assert.Equal(t, resp.Headers, unmarshaled.Headers)
		assert.Equal(t, resp.Body, unmarshaled.Body)
	})

	t.Run("marshal and unmarshal with minimal fields", func(t *testing.T) {
		resp := WebSocketResponse{
			StatusCode: 404,
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled WebSocketResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.StatusCode, unmarshaled.StatusCode)
		assert.Nil(t, unmarshaled.Headers)
		assert.Empty(t, unmarshaled.Body)
	})

	t.Run("handles different status codes", func(t *testing.T) {
		testCases := []int{200, 201, 400, 401, 403, 404, 500, 502, 503}

		for _, statusCode := range testCases {
			resp := WebSocketResponse{
				StatusCode: statusCode,
				Body:       "test body",
			}

			data, err := json.Marshal(resp)
			require.NoError(t, err)

			var unmarshaled WebSocketResponse
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			assert.Equal(t, statusCode, unmarshaled.StatusCode)
		}
	})

	t.Run("handles empty headers map", func(t *testing.T) {
		resp := WebSocketResponse{
			StatusCode: 200,
			Headers:    map[string]string{},
			Body:       "test",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled WebSocketResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.StatusCode, unmarshaled.StatusCode)
		assert.NotNil(t, unmarshaled.Headers)
		assert.Empty(t, unmarshaled.Headers)
	})
}
