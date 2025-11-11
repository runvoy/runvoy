// Package database defines the repository interfaces for data persistence.
// This file contains the SecretsRepository interface for secret metadata management.
package database

import (
	"context"
	"errors"

	"runvoy/internal/api"
)

// Errors for secrets operations
var (
	ErrSecretNotFound      = errors.New("secret not found")
	ErrSecretAlreadyExists = errors.New("secret already exists")
)

// SecretsRepository defines the interface for managing secret metadata in persistent storage.
// The actual secret values are stored separately in a secrets manager (e.g., AWS Parameter Store).
// This repository only manages the metadata (name, description, created_at, updated_at, etc.)
type SecretsRepository interface {
	// CreateSecret stores a new secret's metadata in persistent storage.
	// Returns an error if a secret with the same name already exists.
	CreateSecret(ctx context.Context, name, description, createdBy string) error

	// GetSecret retrieves a secret's metadata by name.
	// Returns an error if the secret is not found.
	GetSecret(ctx context.Context, name string) (*api.Secret, error)

	// ListSecrets retrieves all secrets (optionally filtered by user).
	// If userEmail is empty, returns all secrets.
	ListSecrets(ctx context.Context, userEmail string) ([]*api.Secret, error)

	// UpdateSecretMetadata updates a secret's description and updated_at timestamp.
	// Returns an error if the secret is not found.
	UpdateSecretMetadata(ctx context.Context, name, description, updatedBy string) error

	// DeleteSecret removes a secret's metadata from persistent storage.
	// Returns an error if the secret is not found.
	DeleteSecret(ctx context.Context, name string) error

	// SecretExists checks if a secret with the given name exists.
	SecretExists(ctx context.Context, name string) (bool, error)
}
