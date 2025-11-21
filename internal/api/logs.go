// Package api defines the API types and structures used across runvoy.
package api

// LogEvent represents a single log event from either execution or backend logs.
// ExecutionLogs: events from user command execution in containers (ECS tasks)
// BackendLogs: events from backend infrastructure
// Events are ordered by timestamp. Clients should sort by timestamp
// and compute line numbers as needed for display purposes.
type LogEvent struct {
	Timestamp int64  `json:"timestamp"` // Unix timestamp in milliseconds
	Message   string `json:"message"`   // The actual log message text
}

// LogsResponse contains all log events for an execution
type LogsResponse struct {
	ExecutionID string     `json:"execution_id"`
	Events      []LogEvent `json:"events"`

	// Current execution status (RUNNING, SUCCEEDED, FAILED, STOPPED)
	Status string `json:"status"`

	// WebSocket URL for streaming logs (only provided if execution is RUNNING)
	WebSocketURL string `json:"websocket_url,omitempty"`
}
