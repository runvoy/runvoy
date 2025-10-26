package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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

// Request types
type ExecRequest struct {
	Action         string            `json:"action"`
	Repo           string            `json:"repo"`
	Branch         string            `json:"branch,omitempty"`
	Command        string            `json:"command"`
	Image          string            `json:"image,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
}

type StatusRequest struct {
	Action  string `json:"action"`
	TaskArn string `json:"task_arn"`
}

type LogsRequest struct {
	Action      string `json:"action"`
	ExecutionID string `json:"execution_id"`
}

// Response types
type ExecResponse struct {
	ExecutionID string `json:"execution_id"`
	TaskArn     string `json:"task_arn"`
	Status      string `json:"status"`
	LogStream   string `json:"log_stream,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	Error       string `json:"error,omitempty"`
}

type StatusResponse struct {
	Status        string `json:"status"`
	DesiredStatus string `json:"desired_status"`
	CreatedAt     string `json:"created_at"`
	Error         string `json:"error,omitempty"`
}

type LogsResponse struct {
	Logs  string `json:"logs"`
	Error string `json:"error,omitempty"`
}

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
