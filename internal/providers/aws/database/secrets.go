// Package database provides AWS-specific database implementations.
// This file contains a SecretsRepository that coordinates DynamoDB and Parameter Store.
package database

import (
	"context"
	"errors"
	"log/slog"

	"runvoy/internal/api"
	"runvoy/internal/database"
	appErrors "runvoy/internal/errors"
	loggerPkg "runvoy/internal/logger"
	"runvoy/internal/providers/aws/secrets"
)

// MetadataRepository defines the interface for secret metadata operations.
type MetadataRepository interface {
	CreateSecret(ctx context.Context, secret *api.Secret) error
	GetSecret(ctx context.Context, name string, includeValue bool) (*api.Secret, error)
	ListSecrets(ctx context.Context, includeValue bool) ([]*api.Secret, error)
	UpdateSecretMetadata(ctx context.Context, name, keyName, description, updatedBy string) error
	DeleteSecret(ctx context.Context, name string) error
	SecretExists(ctx context.Context, name string) (bool, error)
}

// SecretsRepository implements database.SecretsRepository for AWS.
// It coordinates DynamoDB (metadata) and Parameter Store (values) to provide a unified interface.
type SecretsRepository struct {
	metadataRepo MetadataRepository
	valueStore   secrets.ValueStore
	logger       *slog.Logger
}

// Ensure SecretsRepository implements database.SecretsRepository
var _ database.SecretsRepository = (*SecretsRepository)(nil)

// NewSecretsRepository creates a new AWS secrets repository.
func NewSecretsRepository(
	metadataRepo MetadataRepository,
	valueStore secrets.ValueStore,
	logger *slog.Logger,
) *SecretsRepository {
	return &SecretsRepository{
		metadataRepo: metadataRepo,
		valueStore:   valueStore,
		logger:       logger,
	}
}

// CreateSecret stores a new secret with both metadata and value.
func (sr *SecretsRepository) CreateSecret(
	ctx context.Context,
	secret *api.Secret,
) error {
	reqLogger := loggerPkg.DeriveRequestLogger(ctx, sr.logger)

	// Store the value first
	if err := sr.valueStore.StoreSecret(ctx, secret.Name, secret.Value); err != nil {
		reqLogger.Error("failed to store secret value", "error", err, "name", secret.Name)
		return appErrors.ErrInternalError("failed to store secret value", err)
	}

	// Store the metadata
	if err := sr.metadataRepo.CreateSecret(ctx, secret); err != nil {
		reqLogger.Error("failed to store secret metadata", "error", err, "name", secret.Name)
		// Best effort cleanup: try to remove the stored value
		_ = sr.valueStore.DeleteSecret(ctx, secret.Name)
		return appErrors.ErrInternalError("failed to store secret metadata", err)
	}

	return nil
}

// GetSecret retrieves a secret with its metadata and optionally its value.
func (sr *SecretsRepository) GetSecret(ctx context.Context, name string, includeValue bool) (*api.Secret, error) {
	reqLogger := loggerPkg.DeriveRequestLogger(ctx, sr.logger)

	// Get the metadata
	secret, err := sr.metadataRepo.GetSecret(ctx, name, false)
	if err != nil {
		// Check if it's a not found error (expected) or a real error
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) && appErr.Code == appErrors.ErrCodeSecretNotFound {
			return nil, err // Pass through not found errors as-is
		}
		return nil, appErrors.ErrInternalError("failed to get secret", err)
	}

	// Get the value if requested
	if includeValue {
		value, valueErr := sr.valueStore.RetrieveSecret(ctx, name)
		if valueErr != nil {
			reqLogger.Debug("failed to retrieve secret value", "error", valueErr, "name", name)
			// Don't fail if value retrieval fails - return metadata only
			return secret, nil
		}
		secret.Value = value
	}

	return secret, nil
}

// ListSecrets retrieves all secrets with optionally their values.
func (sr *SecretsRepository) ListSecrets(ctx context.Context, includeValue bool) ([]*api.Secret, error) {
	reqLogger := loggerPkg.DeriveRequestLogger(ctx, sr.logger)

	// Get all metadata
	secretList, err := sr.metadataRepo.ListSecrets(ctx, false)
	if err != nil {
		return nil, appErrors.ErrInternalError("failed to list secrets", err)
	}

	if secretList == nil {
		secretList = []*api.Secret{}
	}

	// Populate values for each secret if requested
	if includeValue {
		for _, secret := range secretList {
			value, valueErr := sr.valueStore.RetrieveSecret(ctx, secret.Name)
			if valueErr != nil {
				reqLogger.Debug("failed to retrieve secret value", "error", valueErr, "name", secret.Name)
				// Don't fail - continue with other secrets
				continue
			}
			secret.Value = value
		}
	}

	return secretList, nil
}

// UpdateSecret updates a secret's metadata and/or value.
func (sr *SecretsRepository) UpdateSecret(
	ctx context.Context,
	secret *api.Secret,
) error {
	reqLogger := loggerPkg.DeriveRequestLogger(ctx, sr.logger)

	// Update the value if provided
	if secret.Value != "" {
		if err := sr.valueStore.StoreSecret(ctx, secret.Name, secret.Value); err != nil {
			reqLogger.Error("failed to update secret value", "error", err, "name", secret.Name)
			return appErrors.ErrInternalError("failed to update secret value", err)
		}
	}

	// Get existing secret to preserve metadata that wasn't provided
	existingSecret, err := sr.metadataRepo.GetSecret(ctx, secret.Name, false)
	if err != nil {
		reqLogger.Error("failed to get existing secret", "error", err, "name", secret.Name)
		return appErrors.ErrInternalError("failed to get existing secret", err)
	}

	// Use provided values, or fall back to existing values if not provided
	keyName := secret.KeyName
	if keyName == "" {
		keyName = existingSecret.KeyName
	}

	description := secret.Description
	if description == "" {
		description = existingSecret.Description
	}

	// Update metadata with merged values
	if updateErr := sr.metadataRepo.UpdateSecretMetadata(
		ctx, secret.Name, keyName, description, secret.UpdatedBy,
	); updateErr != nil {
		reqLogger.Error("failed to update secret metadata", "error", updateErr, "name", secret.Name)
		return appErrors.ErrInternalError("failed to update secret metadata", updateErr)
	}

	return nil
}

// DeleteSecret removes both the metadata and value of a secret.
func (sr *SecretsRepository) DeleteSecret(ctx context.Context, name string) error {
	reqLogger := loggerPkg.DeriveRequestLogger(ctx, sr.logger)

	// Delete the value (best effort - continue even if it fails)
	if err := sr.valueStore.DeleteSecret(ctx, name); err != nil {
		reqLogger.Debug("failed to delete secret value", "error", err, "name", name)
	}

	// Delete the metadata
	if err := sr.metadataRepo.DeleteSecret(ctx, name); err != nil {
		reqLogger.Error("failed to delete secret metadata", "error", err, "name", name)
		return appErrors.ErrInternalError("failed to delete secret metadata", err)
	}

	return nil
}

// GetSecretsByRequestID retrieves all secrets created or modified by a specific request ID.
func (sr *SecretsRepository) GetSecretsByRequestID(_ context.Context, _ string) ([]*api.Secret, error) {
	// This will be implemented by the metadata repository
	// For now, return an empty list since the metadata repo may not have this capability yet
	return []*api.Secret{}, nil
}
