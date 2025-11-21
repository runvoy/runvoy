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

	// SecretVarNames contains the environment variable names that should be treated as secrets.
	// This is populated by the service layer after resolving secrets from the Secrets field.
	// It includes both explicitly resolved secrets and pattern-detected sensitive variables.
	SecretVarNames []string `json:"-"` // Not serialized in API responses
}

// ExecutionResponse represents the response to an execution request
type ExecutionResponse struct {
	ExecutionID string `json:"execution_id"`
	LogURL      string `json:"log_url"`
	Status      string `json:"status"`
	ImageID     string `json:"image_id"`
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
	ExecutionID         string     `json:"execution_id"`
	CreatedBy           string     `json:"created_by"`
	OwnedBy             []string   `json:"owned_by"`
	Command             string     `json:"command"`
	StartedAt           time.Time  `json:"started_at"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
	Status              string     `json:"status"`
	ExitCode            int        `json:"exit_code"`
	DurationSeconds     int        `json:"duration_seconds,omitempty"`
	LogStreamName       string     `json:"log_stream_name,omitempty"`
	CreatedByRequestID  string     `json:"created_by_request_id"`
	ModifiedByRequestID string     `json:"modified_by_request_id"`
	ComputePlatform     string     `json:"cloud,omitempty"`
}

// BackendLogsRequest represents a request to query backend logs by request ID
type BackendLogsRequest struct {
	RequestID string `json:"request_id"`
}

// BackendLogsResponse represents the response from a backend logs query
// Contains LogEvents from backend infrastructure
// Distinct from ExecutionLogs which come from user command execution in containers
type BackendLogsResponse struct {
	RequestID string     `json:"request_id"`
	Logs      []LogEvent `json:"logs"`
	Status    string     `json:"status"`
}
