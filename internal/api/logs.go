package api

// LogEvent represents a single log event.
// Events are ordered by timestamp. Clients should sort by timestamp
// and compute line numbers as needed for display purposes.
type LogEvent struct {
	EventID   string `json:"event_id"`  // Unique identifier for the log event
	Timestamp int64  `json:"timestamp"` // Unix timestamp in milliseconds
	Message   string `json:"message"`   // The actual log message text
}

// LogsResponse contains all log events for an execution.
// Contract: For running executions, events is nil and websocket_url is provided.
// For terminal executions (SUCCEEDED, FAILED, STOPPED), events is an array
// (never nil, may be empty) and websocket_url is omitted.
type LogsResponse struct {
	ExecutionID string `json:"execution_id"`
	// Events is nil for running executions, and a non-nil array (possibly empty) for terminal executions.
	Events []LogEvent `json:"events"`

	// Current execution status (RUNNING, SUCCEEDED, FAILED, STOPPED)
	Status string `json:"status"`

	// WebSocket URL for streaming logs (only provided when execution is running).
	// Omitted for terminal executions.
	WebSocketURL string `json:"websocket_url,omitempty"`
}

// TraceResponse contains logs and related resources for a request ID
type TraceResponse struct {
	// Logs retrieved from backend infrastructure
	Logs []LogEvent `json:"logs"`

	// Related resources associated with this request ID
	RelatedResources RelatedResources `json:"related_resources"`
}

// RelatedResources contains all resources associated with a request ID
type RelatedResources struct {
	Executions []*Execution `json:"executions,omitempty"`
	Secrets    []*Secret    `json:"secrets,omitempty"`
	Users      []*User      `json:"users,omitempty"`
	Images     []ImageInfo  `json:"images,omitempty"`
}
