package api

// Request types for different actions
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

// Unified request type for Lambda handler (can represent any action)
type Request struct {
	Action      string            `json:"action"`
	TaskArn     string            `json:"task_arn,omitempty"`
	ExecutionID string            `json:"execution_id,omitempty"`

	// Execution parameters (for "exec" action)
	Repo           string            `json:"repo,omitempty"`
	Branch         string            `json:"branch,omitempty"`
	Command        string            `json:"command,omitempty"`
	Image          string            `json:"image,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
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

// Unified response type for Lambda handler (can represent any action)
type Response struct {
	ExecutionID   string `json:"execution_id,omitempty"`
	TaskArn       string `json:"task_arn,omitempty"`
	Status        string `json:"status,omitempty"`
	DesiredStatus string `json:"desired_status,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	LogStream     string `json:"log_stream,omitempty"`
	Logs          string `json:"logs,omitempty"`
	Error         string `json:"error,omitempty"`
}
