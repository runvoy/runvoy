package api

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// HealthResponse represents the response to a health check request
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}
