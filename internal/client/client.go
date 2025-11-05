// Package client provides HTTP client functionality for the runvoy API.
// It handles authentication, request/response serialization, and error handling.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"runvoy/internal/api"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/logger"
)

// Client provides a generic HTTP client for API operations
type Client struct {
	config *config.Config
	logger *slog.Logger
}

// New creates a new API client
func New(cfg *config.Config, log *slog.Logger) *Client {
	return &Client{
		config: cfg,
		logger: log,
	}
}

// Request represents an API request
type Request struct {
	Method string
	Path   string
	Body   any
}

// Response represents an API response
type Response struct {
	StatusCode int
	Body       []byte
}

// Do makes an HTTP request to the API
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	var bodyReader io.Reader
	if req.Body != nil {
		jsonData, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	apiURL, err := url.JoinPath(c.config.APIEndpoint, req.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid API endpoint: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, apiURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set(constants.ContentTypeHeader, "application/json")
	httpReq.Header.Set(constants.APIKeyHeader, c.config.APIKey)

	// Log before making HTTP request with deadline info
	logArgs := []any{
		"operation", "HTTP.Request",
		"method", req.Method,
		"url", apiURL,
	}
	if req.Body != nil {
		bodyBytes, _ := json.Marshal(req.Body)
		logArgs = append(logArgs, "hasBody", true, "bodySize", len(bodyBytes))
	} else {
		logArgs = append(logArgs, "hasBody", false)
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	c.logger.Debug("calling external service", logArgs...)

	httpClient := &http.Client{}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Log response summary
	c.logger.Debug("received HTTP response",
		"status", resp.StatusCode,
		"bodySize", len(body),
		"method", req.Method,
		"url", apiURL)

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body,
	}, nil
}

// DoJSON makes a request and unmarshals the response into the provided interface
func (c *Client) DoJSON(ctx context.Context, req Request, result any) error {
	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}

	if resp.StatusCode >= constants.HTTPStatusBadRequest {
		var errorResp api.ErrorResponse
		if err = json.Unmarshal(resp.Body, &errorResp); err != nil {
			return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(resp.Body))
		}
		return fmt.Errorf("[%d] %s: %s", resp.StatusCode, errorResp.Error, errorResp.Details)
	}

	if err = json.Unmarshal(resp.Body, result); err != nil {
		c.logger.Debug("response body", "body", string(resp.Body))
		return fmt.Errorf("failed to parse response: %w", err)
	}

	return nil
}

// CreateUser creates a new user using the API
func (c *Client) CreateUser(ctx context.Context, req api.CreateUserRequest) (*api.CreateUserResponse, error) {
	var resp api.CreateUserResponse
	err := c.DoJSON(ctx, Request{
		Method: "POST",
		Path:   "/api/v1/users/create",
		Body:   req,
	}, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// RevokeUser revokes a user's API key
func (c *Client) RevokeUser(ctx context.Context, req api.RevokeUserRequest) (*api.RevokeUserResponse, error) {
	var resp api.RevokeUserResponse
	err := c.DoJSON(ctx, Request{
		Method: "POST",
		Path:   "/api/v1/users/revoke",
		Body:   req,
	}, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// ListUsers lists all users
func (c *Client) ListUsers(ctx context.Context) (*api.ListUsersResponse, error) {
	var resp api.ListUsersResponse
	err := c.DoJSON(ctx, Request{
		Method: "GET",
		Path:   "/api/v1/users/",
	}, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetHealth checks the API health status
func (c *Client) GetHealth(ctx context.Context) (*api.HealthResponse, error) {
	var resp api.HealthResponse
	err := c.DoJSON(ctx, Request{
		Method: "GET",
		Path:   "/api/v1/health",
	}, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// RunCommand executes a command remotely via the runvoy API.
func (c *Client) RunCommand(ctx context.Context, req *api.ExecutionRequest) (*api.ExecutionResponse, error) {
	var resp api.ExecutionResponse
	err := c.DoJSON(ctx, Request{
		Method: "POST",
		Path:   "/api/v1/run",
		Body:   req,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetLogs gets the logs for an execution
// The response includes a WebSocketURL field for streaming logs if WebSocket is configured
func (c *Client) GetLogs(ctx context.Context, executionID string) (*api.LogsResponse, error) {
	var resp api.LogsResponse
	err := c.DoJSON(ctx, Request{
		Method: "GET",
		Path:   fmt.Sprintf("/api/v1/executions/%s/logs", executionID),
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetExecutionStatus gets the status of an execution
func (c *Client) GetExecutionStatus(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error) {
	var resp api.ExecutionStatusResponse
	err := c.DoJSON(ctx, Request{
		Method: "GET",
		Path:   fmt.Sprintf("/api/v1/executions/%s/status", executionID),
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// KillExecution stops a running execution by its ID
func (c *Client) KillExecution(ctx context.Context, executionID string) (*api.KillExecutionResponse, error) {
	var resp api.KillExecutionResponse
	err := c.DoJSON(ctx, Request{
		Method: "POST",
		Path:   fmt.Sprintf("/api/v1/executions/%s/kill", executionID),
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListExecutions fetches all executions
func (c *Client) ListExecutions(ctx context.Context) ([]api.Execution, error) {
	var resp []api.Execution
	err := c.DoJSON(ctx, Request{
		Method: "GET",
		Path:   "/api/v1/executions",
	}, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ClaimAPIKey claims a user's API key
func (c *Client) ClaimAPIKey(ctx context.Context, token string) (*api.ClaimAPIKeyResponse, error) {
	var resp api.ClaimAPIKeyResponse
	err := c.DoJSON(ctx, Request{
		Method: "GET",
		Path:   fmt.Sprintf("/api/v1/claim/%s", token),
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// RegisterImage registers a new container image for execution, optionally marking it as the default
func (c *Client) RegisterImage(ctx context.Context, image string, isDefault *bool) (*api.RegisterImageResponse, error) {
	var resp api.RegisterImageResponse
	err := c.DoJSON(ctx, Request{
		Method: "POST",
		Path:   "/api/v1/images/register",
		Body:   api.RegisterImageRequest{Image: image, IsDefault: isDefault},
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListImages retrieves all registered container images
func (c *Client) ListImages(ctx context.Context) (*api.ListImagesResponse, error) {
	var resp api.ListImagesResponse
	err := c.DoJSON(ctx, Request{
		Method: "GET",
		Path:   "/api/v1/images",
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// UnregisterImage removes a container image from the registry
func (c *Client) UnregisterImage(ctx context.Context, image string) (*api.RemoveImageResponse, error) {
	var resp api.RemoveImageResponse
	err := c.DoJSON(ctx, Request{
		Method: "DELETE",
		Path:   fmt.Sprintf("/api/v1/images/%s", image),
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
