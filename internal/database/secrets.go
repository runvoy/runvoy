// Package database defines the repository interfaces for data persistence.
// This file contains the SecretsRepository interface for secret metadata management.
package database

import (
	"context"

	"runvoy/internal/api"
	appErrors "runvoy/internal/errors"
)

// Errors for secrets operations
var (
	ErrSecretNotFound      = appErrors.ErrSecretNotFound("secret not found", nil)
	ErrSecretAlreadyExists = appErrors.ErrSecretAlreadyExists("secret already exists", nil)
)

// SecretsRepository defines the interface for persisting secret data.
// Implementations handle storing and retrieving secrets in their preferred storage backend.
type SecretsRepository interface {
	// CreateSecret stores a new secret.
	// The secret's CreatedBy field must be set by the caller.
	// The repository sets CreatedAt and UpdatedAt timestamps.
	// Returns an error if a secret with the same name already exists.
	CreateSecret(ctx context.Context, secret *api.Secret) error

	// GetSecret retrieves a secret by name.
	// If includeValue is true, the secret value will be decrypted and included in the response.
	// Returns an error if the secret is not found.
	GetSecret(ctx context.Context, name string, includeValue bool) (*api.Secret, error)

	// ListSecrets retrieves all secrets.
	// If includeValue is true, secret values will be decrypted and included in the response.
	ListSecrets(ctx context.Context, includeValue bool) ([]*api.Secret, error)

	// UpdateSecret updates a secret's value and/or editable properties.
	// The updatedAt timestamp is always refreshed.
	// Returns an error if the secret is not found.
	UpdateSecret(ctx context.Context, name string, updates *api.UpdateSecretRequest, updatedBy string) error

	// DeleteSecret removes a secret from storage.
	// Returns an error if the secret is not found.
	DeleteSecret(ctx context.Context, name string) error
}
