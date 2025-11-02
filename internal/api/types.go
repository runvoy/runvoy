// Package api defines the API types and structures used across runvoy.
// It contains request and response structures for the orchestrator API.
package api

import (
	"time"
)

// Request/Response types for the API

// ExecutionRequest represents a request to execute a command
type ExecutionRequest struct {
	Command string            `json:"command"`
	Lock    string            `json:"lock,omitempty"`
	Image   string            `json:"image,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Timeout int               `json:"timeout,omitempty"`

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

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// User represents a user in the system
type User struct {
	Email     string    `json:"email"`
	APIKey    string    `json:"api_key,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `json:"revoked"`
	LastUsed  time.Time `json:"last_used,omitempty"`
}

// Execution represents an execution record
type Execution struct {
	ExecutionID     string     `json:"execution_id"`
	UserEmail       string     `json:"user_email"`
	Command         string     `json:"command"`
	LockName        string     `json:"lock_name,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	Status          string     `json:"status"`
	ExitCode        int        `json:"exit_code"`
	DurationSeconds int        `json:"duration_seconds,omitempty"`
	LogStreamName   string     `json:"log_stream_name,omitempty"`
	RequestID       string     `json:"request_id,omitempty"`
	ComputePlatform string     `json:"cloud,omitempty"`
}

// LogEvent represents a single log event line
type LogEvent struct {
	Line      int    `json:"line"`
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
}

// LogsResponse contains all log events for an execution
type LogsResponse struct {
	ExecutionID string     `json:"execution_id"`
	Events      []LogEvent `json:"events"`
}

// Lock represents a lock record
type Lock struct {
	LockName    string    `json:"lock_name"`
	ExecutionID string    `json:"execution_id"`
	UserEmail   string    `json:"user_email"`
	AcquiredAt  time.Time `json:"acquired_at"`
	TTL         int64     `json:"ttl"`
}

// CreateUserRequest represents the request to create a new user
type CreateUserRequest struct {
	Email  string `json:"email"`
	APIKey string `json:"api_key,omitempty"` // Optional: if not provided, one will be generated
}

// CreateUserResponse represents the response after creating a user
type CreateUserResponse struct {
	User       *User  `json:"user"`
	ClaimToken string `json:"claim_token"`
}

// PendingAPIKey represents a pending API key awaiting claim
type PendingAPIKey struct {
	SecretToken  string     `json:"secret_token"`
	APIKey       string     `json:"api_key"`
	UserEmail    string     `json:"user_email"`
	CreatedBy    string     `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    int64      `json:"expires_at"` // Unix timestamp for TTL
	Viewed       bool       `json:"viewed"`
	ViewedAt     *time.Time `json:"viewed_at,omitempty"`
	ViewedFromIP string     `json:"viewed_from_ip,omitempty"`
}

// ClaimAPIKeyResponse represents the response when claiming an API key
type ClaimAPIKeyResponse struct {
	APIKey    string `json:"api_key"`
	UserEmail string `json:"user_email"`
	Message   string `json:"message,omitempty"`
}

// RevokeUserRequest represents the request to revoke a user's API key
type RevokeUserRequest struct {
	Email string `json:"email"`
}

// RevokeUserResponse represents the response after revoking a user
type RevokeUserResponse struct {
	Message string `json:"message"`
	Email   string `json:"email"`
}

// HealthResponse represents the response to a health check request
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// RegisterImageRequest represents the request to register a new Docker image
type RegisterImageRequest struct {
	Image     string `json:"image"`
	IsDefault *bool  `json:"is_default,omitempty"`
}

// RegisterImageResponse represents the response after registering an image
type RegisterImageResponse struct {
	Image   string `json:"image"`
	Message string `json:"message"`
}

// RemoveImageRequest represents the request to remove a Docker image
type RemoveImageRequest struct {
	Image string `json:"image"` // Docker image name (e.g., "hashicorp/terraform:1.6")
}

// RemoveImageResponse represents the response after removing an image
type RemoveImageResponse struct {
	Image   string `json:"image"`
	Message string `json:"message"`
}

// ImageInfo represents information about a registered image
type ImageInfo struct {
	Image              string `json:"image"`
	TaskDefinitionARN  string `json:"task_definition_arn,omitempty"`
	TaskDefinitionName string `json:"task_definition_name,omitempty"`
	IsDefault          bool   `json:"is_default,omitempty"` // True if this is the default image
}

// ListImagesResponse represents the response containing all registered images
type ListImagesResponse struct {
	Images []ImageInfo `json:"images"`
}
