package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	sharedAPI "mycli/pkg/api"
)

type Client struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

func NewClient(endpoint, apiKey string) *Client {
	return &Client{
		endpoint: endpoint,
		apiKey:   apiKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Type aliases for convenience - expose shared types
type ExecRequest = sharedAPI.ExecRequest
type StatusRequest = sharedAPI.StatusRequest
type LogsRequest = sharedAPI.LogsRequest
type ExecResponse = sharedAPI.ExecResponse
type StatusResponse = sharedAPI.StatusResponse
type LogsResponse = sharedAPI.LogsResponse

func (c *Client) Exec(ctx context.Context, req ExecRequest) (*ExecResponse, error) {
	req.Action = "exec"

	var resp ExecResponse
	if err := c.doRequest(ctx, req, &resp); err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("API error: %s", resp.Error)
	}

	return &resp, nil
}

func (c *Client) GetStatus(ctx context.Context, taskArn string) (*StatusResponse, error) {
	req := StatusRequest{
		Action:  "status",
		TaskArn: taskArn,
	}

	var resp StatusResponse
	if err := c.doRequest(ctx, req, &resp); err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("API error: %s", resp.Error)
	}

	return &resp, nil
}

func (c *Client) GetLogs(ctx context.Context, executionID string) (*LogsResponse, error) {
	req := LogsRequest{
		Action:      "logs",
		ExecutionID: executionID,
	}

	var resp LogsResponse
	if err := c.doRequest(ctx, req, &resp); err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("API error: %s", resp.Error)
	}

	return &resp, nil
}

func (c *Client) GetLogsByTaskArn(ctx context.Context, taskArn string) (*LogsResponse, error) {
	req := LogsRequest{
		Action:  "logs",
		TaskArn: taskArn,
	}

	var resp LogsResponse
	if err := c.doRequest(ctx, req, &resp); err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("API error: %s", resp.Error)
	}

	return &resp, nil
}

func (c *Client) doRequest(ctx context.Context, reqBody interface{}, respBody interface{}) error {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, respBody); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}
