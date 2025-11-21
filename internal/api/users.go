// Package api defines the API types and structures used across runvoy.
package api

import (
	"time"
)

// User represents a user in the system
type User struct {
	Email               string     `json:"email"`
	APIKey              string     `json:"api_key,omitempty"`
	Role                string     `json:"role"`
	CreatedAt           time.Time  `json:"created_at"`
	Revoked             bool       `json:"revoked"`
	LastUsed            *time.Time `json:"last_used,omitempty"`
	CreatedByRequestID  string     `json:"created_by_request_id"`
	ModifiedByRequestID string     `json:"modified_by_request_id"`
}

// CreateUserRequest represents the request to create a new user
type CreateUserRequest struct {
	Email  string `json:"email"`
	APIKey string `json:"api_key,omitempty"` // Optional: if not provided, one will be generated
	Role   string `json:"role"`              // Required: admin, operator, developer, or viewer
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

// ListUsersResponse represents the response containing all users
type ListUsersResponse struct {
	Users []*User `json:"users"`
}
