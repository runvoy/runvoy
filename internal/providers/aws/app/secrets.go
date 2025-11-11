// Package aws provides AWS-specific implementations for the runvoy orchestrator.
// This file contains the comprehensive AWS secrets manager that coordinates
// metadata storage and value encryption/storage.
package aws

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/api"
	"runvoy/internal/database"
	"runvoy/internal/providers/aws/secrets"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// SecretsManager provides a complete secrets management implementation for AWS.
// It handles both metadata storage (DynamoDB) and value encryption/storage (Parameter Store).
// This implementation is provider-specific and only exposed through the app.SecretsManager interface.
type SecretsManager struct {
	metadataRepo database.SecretsRepository
	valueStore   secrets.ValueStore
	logger       *slog.Logger
}

// NewSecretsManager creates a new AWS secrets manager.
func NewSecretsManager(
	metadataRepo database.SecretsRepository,
	ssmClient *ssm.Client,
	secretsPrefix string,
	kmsKeyARN string,
	logger *slog.Logger,
) *SecretsManager {
	valueStore := secrets.NewParameterStoreManager(ssmClient, secretsPrefix, kmsKeyARN, logger)
	return &SecretsManager{
		metadataRepo: metadataRepo,
		valueStore:   valueStore,
		logger:       logger,
	}
}

// populateSecretValue retrieves and populates the actual secret value for a given secret.
// Currently returns the value for all authenticated users (no RBAC yet).
// This is where role-based access control will be added in the future.
func (sm *SecretsManager) populateSecretValue(ctx context.Context, secret *api.Secret) {
	if secret == nil {
		return
	}

	value, err := sm.valueStore.RetrieveSecret(ctx, secret.Name)
	if err != nil {
		sm.logger.Debug("failed to retrieve secret value", "error", err, "name", secret.Name)
		// Don't fail the entire request if value retrieval fails, just return without value
		return
	}

	secret.Value = value
}

// CreateSecret creates a new secret with the given name, description, and value.
func (sm *SecretsManager) CreateSecret(
	ctx context.Context,
	req *api.CreateSecretRequest,
	userEmail string,
) (*api.Secret, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Validate input
	if req.Name == "" {
		return nil, fmt.Errorf("secret name cannot be empty")
	}
	if req.KeyName == "" {
		return nil, fmt.Errorf("secret key_name cannot be empty")
	}
	if req.Value == "" {
		return nil, fmt.Errorf("secret value cannot be empty")
	}

	// Check if secret already exists
	exists, err := sm.metadataRepo.SecretExists(ctx, req.Name)
	if err != nil {
		sm.logger.Error("failed to check if secret exists", "error", err, "name", req.Name)
		return nil, fmt.Errorf("failed to check secret existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("secret with name %q already exists", req.Name)
	}

	// Store the secret value
	if err = sm.valueStore.StoreSecret(ctx, req.Name, req.Value); err != nil {
		sm.logger.Error("failed to store secret value", "error", err, "name", req.Name)
		return nil, fmt.Errorf("failed to store secret value: %w", err)
	}

	// Store the metadata
	if err = sm.metadataRepo.CreateSecret(ctx, req.Name, req.KeyName, req.Description, userEmail); err != nil {
		sm.logger.Error("failed to create secret metadata", "error", err, "name", req.Name)
		// Best effort: try to clean up the stored value
		_ = sm.valueStore.DeleteSecret(ctx, req.Name)
		return nil, fmt.Errorf("failed to create secret metadata: %w", err)
	}

	// Retrieve and return the created secret
	secret, err := sm.metadataRepo.GetSecret(ctx, req.Name)
	if err != nil {
		sm.logger.Error("failed to retrieve created secret", "error", err, "name", req.Name)
		return nil, fmt.Errorf("failed to retrieve secret after creation: %w", err)
	}

	// Populate the secret value
	sm.populateSecretValue(ctx, secret)

	return secret, nil
}

// GetSecret retrieves a secret's metadata and value by name.
func (sm *SecretsManager) GetSecret(ctx context.Context, name string) (*api.Secret, error) {
	if name == "" {
		return nil, fmt.Errorf("secret name cannot be empty")
	}

	secret, err := sm.metadataRepo.GetSecret(ctx, name)
	if err != nil {
		sm.logger.Error("failed to get secret", "error", err, "name", name)
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	// Populate the secret value
	sm.populateSecretValue(ctx, secret)

	return secret, nil
}

// ListSecrets retrieves all secrets with their values, optionally filtered by user.
func (sm *SecretsManager) ListSecrets(ctx context.Context, userEmail string) ([]*api.Secret, error) {
	secretList, err := sm.metadataRepo.ListSecrets(ctx, userEmail)
	if err != nil {
		sm.logger.Error("failed to list secrets", "error", err)
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	if secretList == nil {
		secretList = []*api.Secret{}
	}

	// Populate the secret values for all secrets
	for _, secret := range secretList {
		sm.populateSecretValue(ctx, secret)
	}

	return secretList, nil
}

// UpdateSecret updates a secret (metadata and/or value).
// Users can update: description, keyName, and value.
// Always updates the UpdatedAt timestamp.
// If Value is provided (non-empty), also updates the secret value in the value store.
func (sm *SecretsManager) UpdateSecret(
	ctx context.Context,
	name string,
	req *api.UpdateSecretRequest,
	userEmail string,
) (*api.Secret, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if name == "" {
		return nil, fmt.Errorf("secret name cannot be empty")
	}

	// Check if secret exists
	exists, err := sm.metadataRepo.SecretExists(ctx, name)
	if err != nil {
		sm.logger.Error("failed to check if secret exists", "error", err, "name", name)
		return nil, fmt.Errorf("failed to check secret existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("secret %q not found", name)
	}

	// Update value if provided
	if req.Value != "" {
		if err = sm.valueStore.StoreSecret(ctx, name, req.Value); err != nil {
			sm.logger.Error("failed to store secret value", "error", err, "name", name)
			return nil, fmt.Errorf("failed to update secret value: %w", err)
		}
	}

	// Always update metadata (description, keyName, and timestamp)
	if err = sm.metadataRepo.UpdateSecretMetadata(ctx, name, req.KeyName, req.Description, userEmail); err != nil {
		sm.logger.Error("failed to update secret metadata", "error", err, "name", name)
		return nil, fmt.Errorf("failed to update secret metadata: %w", err)
	}

	// Retrieve and return the updated secret
	secret, err := sm.metadataRepo.GetSecret(ctx, name)
	if err != nil {
		sm.logger.Error("failed to retrieve updated secret", "error", err, "name", name)
		return nil, fmt.Errorf("failed to retrieve secret after update: %w", err)
	}

	// Populate the secret value
	sm.populateSecretValue(ctx, secret)

	return secret, nil
}

// DeleteSecret deletes a secret and its value.
func (sm *SecretsManager) DeleteSecret(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("secret name cannot be empty")
	}

	// Check if secret exists
	exists, err := sm.metadataRepo.SecretExists(ctx, name)
	if err != nil {
		sm.logger.Error("failed to check if secret exists", "error", err, "name", name)
		return fmt.Errorf("failed to check secret existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("secret %q not found", name)
	}

	// Delete the secret value
	if err = sm.valueStore.DeleteSecret(ctx, name); err != nil {
		sm.logger.Error("failed to delete secret value", "error", err, "name", name)
		// Continue to delete metadata even if value deletion fails
	}

	// Delete the metadata
	if err = sm.metadataRepo.DeleteSecret(ctx, name); err != nil {
		sm.logger.Error("failed to delete secret metadata", "error", err, "name", name)
		return fmt.Errorf("failed to delete secret metadata: %w", err)
	}

	return nil
}
