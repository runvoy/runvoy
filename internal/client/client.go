package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"runvoy/internal/config"
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
func (c *Client) Do(req Request) (*Response, error) {
	// Create request body
	var bodyReader io.Reader
	if req.Body != nil {
		jsonData, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	// Create URL
	url, err := url.JoinPath(c.config.APIEndpoint, req.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid API endpoint: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest(req.Method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.config.APIKey)

	// Log request
	if req.Body != nil {
		bodyBytes, _ := json.Marshal(req.Body)
		c.logger.Debug("Making API request", "method", req.Method, "url", url, "body", string(bodyBytes))
	} else {
		c.logger.Debug("Making API request", "method", req.Method, "url", url)
	}

	// Make request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body,
	}, nil
}

// DoJSON makes a request and unmarshals the response into the provided interface
func (c *Client) DoJSON(req Request, result interface{}) error {
	resp, err := c.Do(req)
	if err != nil {
		return err
	}

	// Check for error responses
	if resp.StatusCode >= 400 {
		var errorResp struct {
			Error   string `json:"error"`
			Details string `json:"details"`
		}
		if err := json.Unmarshal(resp.Body, &errorResp); err != nil {
			return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(resp.Body))
		}
		return fmt.Errorf("%s: %s", errorResp.Error, errorResp.Details)
	}

	// Unmarshal successful response
	if err := json.Unmarshal(resp.Body, result); err != nil {
		c.logger.Debug("Response body", "body", string(resp.Body))
		return fmt.Errorf("failed to parse response: %w", err)
	}

	return nil
}
