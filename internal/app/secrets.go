// Package app provides business logic for managing execution and resources.
// This file contains the secrets service for managing user secrets.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"runvoy/internal/api"
	"runvoy/internal/database"
	appErrors "runvoy/internal/errors"
)

// valueStore is the internal interface for storing and retrieving secret values.
// Implementations manage the actual secret payloads (e.g., in Parameter Store).
type valueStore interface {
	// StoreSecret saves a secret value and returns any error.
	StoreSecret(ctx context.Context, name, value string) error

	// RetrieveSecret gets a secret value by name.
	RetrieveSecret(ctx context.Context, name string) (string, error)

	// DeleteSecret removes a secret value by name.
	DeleteSecret(ctx context.Context, name string) error
}

// SecretsService provides business logic for managing secrets.
// It implements the SecretsManager interface defined in main.go.
type SecretsService struct {
	repo    database.SecretsRepository
	manager valueStore
	logger  *slog.Logger
}

// NewSecretsService creates a new secrets service with the given repository and value store.
func NewSecretsService(repo database.SecretsRepository, manager valueStore, logger *slog.Logger) *SecretsService {
	return &SecretsService{
		repo:    repo,
		manager: manager,
		logger:  logger,
	}
}

// CreateSecret creates a new secret with the given name, description, and value.
// Returns the created secret metadata and any error.
func (s *SecretsService) CreateSecret(
	ctx context.Context,
	req *api.CreateSecretRequest,
	userEmail string,
) (*api.Secret, error) {
	if req == nil {
		return nil, appErrors.ErrBadRequest("request cannot be nil", nil)
	}

	// Validate input
	if req.Name == "" {
		return nil, appErrors.ErrBadRequest("secret name cannot be empty", nil)
	}
	if req.KeyName == "" {
		return nil, appErrors.ErrBadRequest("secret key_name cannot be empty", nil)
	}
	if req.Value == "" {
		return nil, appErrors.ErrBadRequest("secret value cannot be empty", nil)
	}

	// Check if secret already exists
	exists, err := s.repo.SecretExists(ctx, req.Name)
	if err != nil {
		s.logger.Error("failed to check if secret exists", "error", err, "name", req.Name)
		return nil, appErrors.ErrInternalError("failed to check secret existence", err)
	}
	if exists {
		return nil, appErrors.ErrBadRequest(fmt.Sprintf("secret with name %q already exists", req.Name), nil)
	}

	// Store the secret value
	if err = s.manager.StoreSecret(ctx, req.Name, req.Value); err != nil {
		s.logger.Error("failed to store secret value", "error", err, "name", req.Name)
		return nil, appErrors.ErrInternalError("failed to store secret value", err)
	}

	// Store the metadata
	if err = s.repo.CreateSecret(ctx, req.Name, req.KeyName, req.Description, userEmail); err != nil {
		s.logger.Error("failed to create secret metadata", "error", err, "name", req.Name)
		// Best effort: try to clean up the stored value
		_ = s.manager.DeleteSecret(ctx, req.Name)
		return nil, appErrors.ErrInternalError("failed to create secret metadata", err)
	}

	// Retrieve and return the created secret
	secret, err := s.repo.GetSecret(ctx, req.Name)
	if err != nil {
		s.logger.Error("failed to retrieve created secret", "error", err, "name", req.Name)
		return nil, appErrors.ErrInternalError("failed to retrieve secret after creation", err)
	}

	return secret, nil
}

// GetSecret retrieves a secret's metadata by name.
// Returns the secret metadata (without the value).
func (s *SecretsService) GetSecret(ctx context.Context, name string) (*api.Secret, error) {
	if name == "" {
		return nil, appErrors.ErrBadRequest("secret name cannot be empty", nil)
	}

	secret, err := s.repo.GetSecret(ctx, name)
	if err != nil {
		if errors.Is(err, database.ErrSecretNotFound) {
			return nil, appErrors.ErrNotFound(fmt.Sprintf("secret %q not found", name), nil)
		}
		s.logger.Error("failed to get secret", "error", err, "name", name)
		return nil, appErrors.ErrInternalError("failed to get secret", err)
	}

	return secret, nil
}

// ListSecrets retrieves all secrets (optionally filtered by user).
// If userEmail is empty, returns all secrets.
func (s *SecretsService) ListSecrets(ctx context.Context, userEmail string) ([]*api.Secret, error) {
	secrets, err := s.repo.ListSecrets(ctx, userEmail)
	if err != nil {
		s.logger.Error("failed to list secrets", "error", err)
		return nil, appErrors.ErrInternalError("failed to list secrets", err)
	}

	if secrets == nil {
		secrets = []*api.Secret{}
	}

	return secrets, nil
}

// UpdateSecretMetadata updates a secret's metadata (description, updated_at, updated_by).
// Returns the updated secret metadata and any error.
func (s *SecretsService) UpdateSecretMetadata(
	ctx context.Context,
	name string,
	req *api.UpdateSecretMetadataRequest,
	userEmail string,
) (*api.Secret, error) {
	if req == nil {
		return nil, appErrors.ErrBadRequest("request cannot be nil", nil)
	}
	if name == "" {
		return nil, appErrors.ErrBadRequest("secret name cannot be empty", nil)
	}

	// Check if secret exists
	exists, err := s.repo.SecretExists(ctx, name)
	if err != nil {
		s.logger.Error("failed to check if secret exists", "error", err, "name", name)
		return nil, appErrors.ErrInternalError("failed to check secret existence", err)
	}
	if !exists {
		return nil, appErrors.ErrNotFound(fmt.Sprintf("secret %q not found", name), nil)
	}

	// Update metadata
	if err = s.repo.UpdateSecretMetadata(ctx, name, req.Description, userEmail); err != nil {
		s.logger.Error("failed to update secret metadata", "error", err, "name", name)
		return nil, appErrors.ErrInternalError("failed to update secret metadata", err)
	}

	// Retrieve and return the updated secret
	secret, err := s.repo.GetSecret(ctx, name)
	if err != nil {
		s.logger.Error("failed to retrieve updated secret", "error", err, "name", name)
		return nil, appErrors.ErrInternalError("failed to retrieve secret after update", err)
	}

	return secret, nil
}

// SetSecretValue updates a secret's value without changing its metadata.
// Returns any error.
func (s *SecretsService) SetSecretValue(ctx context.Context, name, value string) error {
	if name == "" {
		return appErrors.ErrBadRequest("secret name cannot be empty", nil)
	}
	if value == "" {
		return appErrors.ErrBadRequest("secret value cannot be empty", nil)
	}

	// Check if secret exists
	exists, err := s.repo.SecretExists(ctx, name)
	if err != nil {
		s.logger.Error("failed to check if secret exists", "error", err, "name", name)
		return appErrors.ErrInternalError("failed to check secret existence", err)
	}
	if !exists {
		return appErrors.ErrNotFound(fmt.Sprintf("secret %q not found", name), nil)
	}

	// Update the value
	if err = s.manager.StoreSecret(ctx, name, value); err != nil {
		s.logger.Error("failed to store secret value", "error", err, "name", name)
		return appErrors.ErrInternalError("failed to update secret value", err)
	}

	return nil
}

// DeleteSecret deletes a secret and its value.
// Returns any error.
func (s *SecretsService) DeleteSecret(ctx context.Context, name string) error {
	if name == "" {
		return appErrors.ErrBadRequest("secret name cannot be empty", nil)
	}

	// Check if secret exists
	exists, err := s.repo.SecretExists(ctx, name)
	if err != nil {
		s.logger.Error("failed to check if secret exists", "error", err, "name", name)
		return appErrors.ErrInternalError("failed to check secret existence", err)
	}
	if !exists {
		return appErrors.ErrNotFound(fmt.Sprintf("secret %q not found", name), nil)
	}

	// Delete the secret value
	if err = s.manager.DeleteSecret(ctx, name); err != nil {
		s.logger.Error("failed to delete secret value", "error", err, "name", name)
		// Continue to delete metadata even if value deletion fails
	}

	// Delete the metadata
	if err = s.repo.DeleteSecret(ctx, name); err != nil {
		s.logger.Error("failed to delete secret metadata", "error", err, "name", name)
		return appErrors.ErrInternalError("failed to delete secret metadata", err)
	}

	return nil
}
