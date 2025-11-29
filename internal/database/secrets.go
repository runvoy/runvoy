package database

import (
	"context"

	"github.com/runvoy/runvoy/internal/api"
	appErrors "github.com/runvoy/runvoy/internal/errors"
)

// Errors for secrets operations.
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
	// The secret's UpdatedBy field must be set by the caller.
	// The Name field identifies which secret to update.
	// The updatedAt timestamp is always refreshed.
	// Returns an error if the secret is not found.
	UpdateSecret(ctx context.Context, secret *api.Secret) error

	// DeleteSecret removes a secret from storage.
	// Returns an error if the secret is not found.
	DeleteSecret(ctx context.Context, name string) error

	// GetSecretsByRequestID retrieves all secrets created or modified by a specific request ID.
	GetSecretsByRequestID(ctx context.Context, requestID string) ([]*api.Secret, error)
}
