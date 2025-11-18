// Package api defines the API types and structures used across runvoy.
package api

import (
	"time"
)

// ExecutionRequest represents a request to execute a command
type ExecutionRequest struct {
	Command string            `json:"command"`
	Image   string            `json:"image,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Timeout int               `json:"timeout,omitempty"`
	Secrets []string          `json:"secrets,omitempty"`

	// Git repository configuration (optional sidecar pattern)
	GitRepo string `json:"git_repo,omitempty"` // Git repository URL (e.g., "https://github.com/user/repo.git")
	GitRef  string `json:"git_ref,omitempty"`  // Git branch, tag, or commit SHA (default: "main")
	GitPath string `json:"git_path,omitempty"` // Working directory within the cloned repo (default: ".")
}

// ExecutionResponse represents the response to an execution request
type ExecutionResponse struct {
	ExecutionID string `json:"execution_id"`
	LogURL      string `json:"log_url"`
	Status      string `json:"status"`
	ImageID     string `json:"image_id,omitempty"`
}

// ExecutionStatusResponse represents the current status of an execution
type ExecutionStatusResponse struct {
	ExecutionID string     `json:"execution_id"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	ExitCode    *int       `json:"exit_code"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// KillExecutionResponse represents the response after killing an execution
type KillExecutionResponse struct {
	ExecutionID string `json:"execution_id"`
	Message     string `json:"message"`
}

// Execution represents an execution record
type Execution struct {
	ExecutionID     string     `json:"execution_id"`
	CreatedBy       string     `json:"created_by"`
	OwnedBy         []string   `json:"owned_by"`
	Command         string     `json:"command"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	Status          string     `json:"status"`
	ExitCode        int        `json:"exit_code"`
	DurationSeconds int        `json:"duration_seconds,omitempty"`
	LogStreamName   string     `json:"log_stream_name,omitempty"`
	RequestID       string     `json:"request_id,omitempty"`
	ComputePlatform string     `json:"cloud,omitempty"`
}
