// Package database provides AWS-specific database implementations.
// This file contains a SecretsRepository that coordinates DynamoDB and Parameter Store.
package database

import (
	"context"
	"log/slog"

	"runvoy/internal/api"
	"runvoy/internal/database"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/providers/aws/secrets"

	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"
)

// SecretsRepository implements database.SecretsRepository for AWS.
// It coordinates DynamoDB (metadata) and Parameter Store (values) to provide a unified interface.
type SecretsRepository struct {
	metadataRepo *dynamoRepo.SecretsRepository
	valueStore   secrets.ValueStore
	logger       *slog.Logger
}

// Ensure SecretsRepository implements database.SecretsRepository
var _ database.SecretsRepository = (*SecretsRepository)(nil)

// NewSecretsRepository creates a new AWS secrets repository.
func NewSecretsRepository(
	metadataRepo *dynamoRepo.SecretsRepository,
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
	// Store the value first
	if err := sr.valueStore.StoreSecret(ctx, secret.Name, secret.Value); err != nil {
		sr.logger.Error("failed to store secret value", "error", err, "name", secret.Name)
		return appErrors.ErrInternalError("failed to store secret value", err)
	}

	// Store the metadata
	if err := sr.metadataRepo.CreateSecret(ctx, secret); err != nil {
		sr.logger.Error("failed to store secret metadata", "error", err, "name", secret.Name)
		// Best effort cleanup: try to remove the stored value
		_ = sr.valueStore.DeleteSecret(ctx, secret.Name)
		return err
	}

	return nil
}

// GetSecret retrieves a secret with its metadata and optionally its value.
func (sr *SecretsRepository) GetSecret(ctx context.Context, name string, includeValue bool) (*api.Secret, error) {
	// Get the metadata
	secret, err := sr.metadataRepo.GetSecret(ctx, name)
	if err != nil {
		return nil, err
	}

	// Get the value if requested
	if includeValue {
		value, valueErr := sr.valueStore.RetrieveSecret(ctx, name)
		if valueErr != nil {
			sr.logger.Debug("failed to retrieve secret value", "error", valueErr, "name", name)
			// Don't fail if value retrieval fails - return metadata only
			return secret, nil
		}
		secret.Value = value
	}

	return secret, nil
}

// ListSecrets retrieves all secrets with optionally their values.
func (sr *SecretsRepository) ListSecrets(ctx context.Context, includeValue bool) ([]*api.Secret, error) {
	// Get all metadata
	secretList, err := sr.metadataRepo.ListSecrets(ctx)
	if err != nil {
		return nil, err
	}

	if secretList == nil {
		secretList = []*api.Secret{}
	}

	// Populate values for each secret if requested
	if includeValue {
		for _, secret := range secretList {
			value, valueErr := sr.valueStore.RetrieveSecret(ctx, secret.Name)
			if valueErr != nil {
				sr.logger.Debug("failed to retrieve secret value", "error", valueErr, "name", secret.Name)
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
	// Update the value if provided
	if secret.Value != "" {
		if err := sr.valueStore.StoreSecret(ctx, secret.Name, secret.Value); err != nil {
			sr.logger.Error("failed to update secret value", "error", err, "name", secret.Name)
			return appErrors.ErrInternalError("failed to update secret value", err)
		}
	}

	// Always update metadata (description, keyName, and timestamp)
	if err := sr.metadataRepo.UpdateSecretMetadata(
		ctx, secret.Name, secret.KeyName, secret.Description, secret.UpdatedBy,
	); err != nil {
		sr.logger.Error("failed to update secret metadata", "error", err, "name", secret.Name)
		return err
	}

	return nil
}

// DeleteSecret removes both the metadata and value of a secret.
func (sr *SecretsRepository) DeleteSecret(ctx context.Context, name string) error {
	// Delete the value (best effort - continue even if it fails)
	if err := sr.valueStore.DeleteSecret(ctx, name); err != nil {
		sr.logger.Debug("failed to delete secret value", "error", err, "name", name)
	}

	// Delete the metadata
	if err := sr.metadataRepo.DeleteSecret(ctx, name); err != nil {
		sr.logger.Error("failed to delete secret metadata", "error", err, "name", name)
		return err
	}

	return nil
}
