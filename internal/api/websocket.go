package api

// WebSocketConnection represents a WebSocket connection record
type WebSocketConnection struct {
	ConnectionID  string `json:"connection_id"`
	ExecutionID   string `json:"execution_id"`
	Functionality string `json:"functionality"`
	ExpiresAt     int64  `json:"expires_at"`
	LastEventID   string `json:"last_event_id,omitempty"`
	ClientIP      string `json:"client_ip,omitempty"`
	Token         string `json:"token,omitempty"`
	UserEmail     string `json:"user_email,omitempty"`
	// Client IP captured when the websocket token was created (for tracing)
	TokenRequestClientIP string `json:"token_request_client_ip,omitempty"`
}

// WebSocketToken represents a WebSocket authentication token
type WebSocketToken struct {
	Token       string `json:"token"`
	ExecutionID string `json:"execution_id"`
	UserEmail   string `json:"user_email,omitempty"`
	// Client IP captured when the websocket token was created (for tracing)
	ClientIP  string `json:"client_ip,omitempty"`
	ExpiresAt int64  `json:"expires_at"`
	CreatedAt int64  `json:"created_at"`
}

// WebSocketMessageType represents the type of WebSocket message
type WebSocketMessageType string

const (
	// WebSocketMessageTypeLog represents a log event message
	WebSocketMessageTypeLog WebSocketMessageType = "log"
	// WebSocketMessageTypeDisconnect represents a disconnect notification message
	WebSocketMessageTypeDisconnect WebSocketMessageType = "disconnect"
)

// WebSocketDisconnectReason represents the reason for a disconnect
type WebSocketDisconnectReason string

const (
	// WebSocketDisconnectReasonExecutionCompleted indicates the execution has completed
	WebSocketDisconnectReasonExecutionCompleted WebSocketDisconnectReason = "execution_completed"
)

// WebSocketMessage represents a WebSocket message sent to clients
type WebSocketMessage struct {
	Type      WebSocketMessageType       `json:"type"`
	Reason    *WebSocketDisconnectReason `json:"reason,omitempty"`
	Message   *string                    `json:"message,omitempty"`
	Timestamp *int64                     `json:"timestamp,omitempty"`
}
