package api

// LogEvent represents a single log event.
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

	// WebSocket URL for streaming logs (provided when execution is running and returned
	// from /run so clients can connect immediately)
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
