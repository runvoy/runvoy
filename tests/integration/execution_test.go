package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"runvoy/internal/api"
	"runvoy/internal/testing"
)

func TestExecutionAPI(t *testing.T) {
	// Create test server with mocks
	ts := testing.NewTestServer(t)
	defer ts.Close()

	// Setup mock expectations
	user := &api.User{
		Email: "test@example.com",
		APIKey: "test-key",
		CreatedAt: time.Now(),
		Revoked: false,
	}

	ts.Mocks.Auth.On("ValidateAPIKey", mock.Anything, "test-key").Return(user, nil)
	ts.Mocks.ECS.On("StartTask", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("arn:aws:ecs:test", nil)
	ts.Mocks.Storage.On("CreateExecution", mock.Anything, mock.Anything).Return(nil)
	ts.Mocks.Lock.On("AcquireLock", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ts.Mocks.Log.On("GenerateLogURL", mock.Anything, mock.Anything).Return("http://test.com/logs", nil)

	// Test execution request
	req := api.ExecutionRequest{
		Command: "echo hello world",
		Lock: "test-lock",
	}

	body, err := json.Marshal(req)
	require.NoError(t, err)

	// Make request
	resp, err := ts.Client().Post(ts.URL()+"/executions", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert response
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var executionResp api.ExecutionResponse
	err = json.NewDecoder(resp.Body).Decode(&executionResp)
	require.NoError(t, err)

	assert.NotEmpty(t, executionResp.ExecutionID)
	assert.NotEmpty(t, executionResp.TaskARN)
	assert.Equal(t, "starting", executionResp.Status)

	// Verify all mocks were called
	ts.Mocks.Auth.AssertExpectations(t)
	ts.Mocks.ECS.AssertExpectations(t)
	ts.Mocks.Storage.AssertExpectations(t)
	ts.Mocks.Lock.AssertExpectations(t)
	ts.Mocks.Log.AssertExpectations(t)
}

func TestExecutionAPI_InvalidAPIKey(t *testing.T) {
	ts := testing.NewTestServer(t)
	defer ts.Close()

	// Setup mock to return error for invalid API key
	ts.Mocks.Auth.On("ValidateAPIKey", mock.Anything, "invalid-key").Return(nil, assert.AnError)

	req := api.ExecutionRequest{
		Command: "echo hello world",
	}

	body, err := json.Marshal(req)
	require.NoError(t, err)

	// Make request with invalid API key
	httpReq, err := http.NewRequest("POST", ts.URL()+"/executions", bytes.NewReader(body))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", "invalid-key")

	resp, err := ts.Client().Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert error response
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var errorResp api.ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	require.NoError(t, err)

	assert.Equal(t, "Invalid API key", errorResp.Error)
	assert.Equal(t, "INVALID_API_KEY", errorResp.Code)
}

func TestHealthEndpoint(t *testing.T) {
	ts := testing.NewTestServer(t)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL() + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health map[string]string
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)

	assert.Equal(t, "healthy", health["status"])
}