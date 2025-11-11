// Package api defines the API types and structures used across runvoy.
// This file contains request and response structures for the secrets API.
package api

import (
	"time"
)

// Secret represents a secret with its metadata (but not its value)
type Secret struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   string    `json:"updated_by"`
}

// CreateSecretRequest represents the request to create a new secret
type CreateSecretRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value"` // The secret value to store
}

// CreateSecretResponse represents the response after creating a secret
type CreateSecretResponse struct {
	Secret  *Secret `json:"secret"`
	Message string  `json:"message"`
}

// UpdateSecretMetadataRequest represents the request to update secret metadata
type UpdateSecretMetadataRequest struct {
	Description string `json:"description,omitempty"`
}

// UpdateSecretMetadataResponse represents the response after updating metadata
type UpdateSecretMetadataResponse struct {
	Secret  *Secret `json:"secret"`
	Message string  `json:"message"`
}

// SetSecretValueRequest represents the request to update just a secret's value
type SetSecretValueRequest struct {
	Value string `json:"value"`
}

// SetSecretValueResponse represents the response after updating secret value
type SetSecretValueResponse struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// GetSecretResponse represents the response when retrieving secret metadata
type GetSecretResponse struct {
	Secret *Secret `json:"secret"`
}

// DeleteSecretResponse represents the response after deleting a secret
type DeleteSecretResponse struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// ListSecretsResponse represents the response containing all secrets
type ListSecretsResponse struct {
	Secrets []*Secret `json:"secrets"`
	Total   int       `json:"total"`
}
