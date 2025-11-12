// Package api defines the API types and structures used across runvoy.
package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		resp := ErrorResponse{
			Error:   "test error",
			Code:    "TEST_CODE",
			Details: "detailed information",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled ErrorResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.Error, unmarshaled.Error)
		assert.Equal(t, resp.Code, unmarshaled.Code)
		assert.Equal(t, resp.Details, unmarshaled.Details)
	})

	t.Run("omit empty fields", func(t *testing.T) {
		resp := ErrorResponse{
			Error: "test error",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		// Code and Details should be omitted when empty
		assert.NotContains(t, string(data), "code")
		assert.NotContains(t, string(data), "details")
	})
}

func TestHealthResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		resp := HealthResponse{
			Status:  "healthy",
			Version: "1.0.0",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled HealthResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.Status, unmarshaled.Status)
		assert.Equal(t, resp.Version, unmarshaled.Version)
	})
}
