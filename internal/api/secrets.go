// Package api defines the API types and structures used across runvoy.
// This file contains request and response structures for the secrets API.
package api

import (
	"time"
)

// Secret represents a secret with its metadata and optionally its value
type Secret struct {
	Name        string    `json:"name"`     // Internal identifier for the secret
	KeyName     string    `json:"key_name"` // Environment variable name (e.g., GITHUB_TOKEN)
	Description string    `json:"description,omitempty"`
	Value       string    `json:"value,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   string    `json:"updated_by"`
}

// CreateSecretRequest represents the request to create a new secret
type CreateSecretRequest struct {
	Name        string `json:"name"`     // Internal identifier for the secret
	KeyName     string `json:"key_name"` // Environment variable name (e.g., GITHUB_TOKEN)
	Description string `json:"description,omitempty"`
	Value       string `json:"value"` // The secret value to store
}

// CreateSecretResponse represents the response after creating a secret
type CreateSecretResponse struct {
	Secret  *Secret `json:"secret"`
	Message string  `json:"message"`
}

// UpdateSecretRequest represents the request to update a secret (metadata and/or value)
// If Value is provided, the secret's value will be updated.
// Description can be updated regardless. UpdatedAt is always refreshed.
type UpdateSecretRequest struct {
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"` // If provided, updates the secret value
}

// UpdateSecretResponse represents the response after updating a secret
type UpdateSecretResponse struct {
	Secret  *Secret `json:"secret"`
	Message string  `json:"message"`
}

// GetSecretResponse represents the response when retrieving a secret
type GetSecretResponse struct {
	Secret *Secret `json:"secret"`
}

// ListSecretsResponse represents the response containing all secrets
type ListSecretsResponse struct {
	Secrets []*Secret `json:"secrets"`
	Total   int       `json:"total"`
}

// DeleteSecretResponse represents the response after deleting a secret
type DeleteSecretResponse struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}
