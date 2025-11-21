// Package api defines the API types and structures used across runvoy.
// This file contains request and response structures for the secrets API.
package api

import (
	"time"
)

// Secret represents a secret with its metadata and optionally its value
type Secret struct {
	Name                string    `json:"name"`     // Internal identifier for the secret
	KeyName             string    `json:"key_name"` // Environment variable name (e.g., GITHUB_TOKEN)
	Description         string    `json:"description,omitempty"`
	Value               string    `json:"value,omitempty"`
	CreatedBy           string    `json:"created_by"`
	OwnedBy             []string  `json:"owned_by"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	UpdatedBy           string    `json:"updated_by"`
	CreatedByRequestID  string    `json:"created_by_request_id"`
	ModifiedByRequestID string    `json:"modified_by_request_id"`
}

// CreateSecretRequest represents the request to create a new secret
type CreateSecretRequest struct {
	Name        string `json:"name"`     // Internal identifier for the secret
	KeyName     string `json:"key_name"` // Environment variable name (e.g., GITHUB_TOKEN)
	Description string `json:"description,omitempty"`
	Value       string `json:"value"` // The secret value to store
}

// CreateSecretResponse represents the response after creating a secret
// To avoid exposing secret data, only a success message is returned.
type CreateSecretResponse struct {
	Message string `json:"message"`
}

// UpdateSecretRequest represents the request to update a secret (metadata and/or value)
// Users can update: description, key_name (environment variable name), and value.
// Description and KeyName are metadata fields. UpdatedAt is always refreshed.
// If Value is provided, the secret's value will be updated.
type UpdateSecretRequest struct {
	Description string `json:"description,omitempty"` // Environment variable description
	KeyName     string `json:"key_name,omitempty"`    // Environment variable name (e.g., GITHUB_TOKEN)
	Value       string `json:"value,omitempty"`       // If provided, updates the secret value
}

// UpdateSecretResponse represents the response after updating a secret
// To avoid exposing secret data, only a success message is returned.
type UpdateSecretResponse struct {
	Message string `json:"message"`
}

// GetSecretRequest represents the request to retrieve a secret
// The secret name is provided in the URL path parameter
type GetSecretRequest struct {
	Name string `json:"name"` // Secret name from URL path
}

// GetSecretResponse represents the response when retrieving a secret
type GetSecretResponse struct {
	Secret *Secret `json:"secret"`
}

// ListSecretsRequest represents the request to list secrets
// Optionally filters by user email
type ListSecretsRequest struct {
	CreatedBy string `json:"created_by,omitempty"` // Filter by user who created the secret
}

// ListSecretsResponse represents the response containing all secrets
type ListSecretsResponse struct {
	Secrets []*Secret `json:"secrets"`
	Total   int       `json:"total"`
}

// DeleteSecretRequest represents the request to delete a secret
// The secret name is provided in the URL path parameter
type DeleteSecretRequest struct {
	Name string `json:"name"` // Secret name from URL path
}

// DeleteSecretResponse represents the response after deleting a secret
type DeleteSecretResponse struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}
