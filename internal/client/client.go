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
)

// Client provides a generic HTTP client for API operations
type Client struct {
	config *config.Config
	logger *slog.Logger
}

// New creates a new API client
func New(cfg *config.Config, logger *slog.Logger) *Client {
	return &Client{
		config: cfg,
		logger: logger,
	}
}

// Request represents an API request
type Request struct {
	Method string
	Path   string
	Body   interface{}
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

	url, err := url.JoinPath(c.config.APIEndpoint, req.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid API endpoint: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set(constants.ContentTypeHeader, "application/json")
	httpReq.Header.Set(constants.ApiKeyHeader, c.config.APIKey)

	if req.Body != nil {
		bodyBytes, _ := json.Marshal(req.Body)
		c.logger.Debug("making API request", "method", req.Method, "url", url, "body", string(bodyBytes))
	} else {
		c.logger.Debug("making API request", "method", req.Method, "url", url)
	}

	c.logger.Debug("request headers", "headers", httpReq.Header)

	httpClient := &http.Client{}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	c.logger.Debug("Response", "status", resp.StatusCode, "body", string(body))

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body,
	}, nil
}

// DoJSON makes a request and unmarshals the response into the provided interface
func (c *Client) DoJSON(ctx context.Context, req Request, result interface{}) error {
	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		var errorResp api.ErrorResponse
		if err := json.Unmarshal(resp.Body, &errorResp); err != nil {
			return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(resp.Body))
		}
		return fmt.Errorf("[%d] %s: %s", resp.StatusCode, errorResp.Error, errorResp.Details)
	}

	if err := json.Unmarshal(resp.Body, result); err != nil {
		c.logger.Debug("Response body", "body", string(resp.Body))
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
