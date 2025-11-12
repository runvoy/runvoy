package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"runvoy/internal/api"
	"runvoy/internal/database"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"
)

// CreateSecret creates a new secret with the given name, description, key name, and value.
func (s *Service) CreateSecret(
	ctx context.Context,
	req *api.CreateSecretRequest,
	userEmail string,
) error {
	if s.secretsRepo == nil {
		return apperrors.ErrInternalError("secrets repository not available", fmt.Errorf("secretsRepo is nil"))
	}
	secret := &api.Secret{
		Name:        req.Name,
		KeyName:     req.KeyName,
		Description: req.Description,
		Value:       req.Value,
		CreatedBy:   userEmail,
	}
	if err := s.secretsRepo.CreateSecret(ctx, secret); err != nil {
		return err
	}
	return nil
}

// GetSecret retrieves a secret's metadata and value by name.
func (s *Service) GetSecret(ctx context.Context, name string) (*api.Secret, error) {
	if s.secretsRepo == nil {
		return nil, apperrors.ErrInternalError("secrets repository not available", fmt.Errorf("secretsRepo is nil"))
	}
	return s.secretsRepo.GetSecret(ctx, name, true)
}

// ListSecrets retrieves all secrets with values.
func (s *Service) ListSecrets(ctx context.Context) ([]*api.Secret, error) {
	if s.secretsRepo == nil {
		return nil, apperrors.ErrInternalError("secrets repository not available", fmt.Errorf("secretsRepo is nil"))
	}
	return s.secretsRepo.ListSecrets(ctx, true)
}

// UpdateSecret updates a secret (metadata and/or value).
func (s *Service) UpdateSecret(
	ctx context.Context,
	name string,
	req *api.UpdateSecretRequest,
	userEmail string,
) error {
	if s.secretsRepo == nil {
		return apperrors.ErrInternalError("secrets repository not available", fmt.Errorf("secretsRepo is nil"))
	}
	secret := &api.Secret{
		Name:        name,
		Description: req.Description,
		KeyName:     req.KeyName,
		Value:       req.Value,
		UpdatedBy:   userEmail,
	}
	if err := s.secretsRepo.UpdateSecret(ctx, secret); err != nil {
		return err
	}
	return nil
}

// DeleteSecret deletes a secret and its value.
func (s *Service) DeleteSecret(ctx context.Context, name string) error {
	if s.secretsRepo == nil {
		return apperrors.ErrInternalError("secrets repository not available", fmt.Errorf("secretsRepo is nil"))
	}
	return s.secretsRepo.DeleteSecret(ctx, name)
}

// resolveSecretsForExecution fetches secret values referenced by name and returns a map of env vars.
// The returned map uses the secret's KeyName as the environment variable key.
// Returns an error if the secrets repository is unavailable or if any requested secret cannot be retrieved.
func (s *Service) resolveSecretsForExecution(
	ctx context.Context,
	secretNames []string,
) (map[string]string, error) {
	if len(secretNames) == 0 {
		return nil, nil
	}

	if s.secretsRepo == nil {
		return nil, apperrors.ErrInternalError("secrets repository not available", fmt.Errorf("secretsRepo is nil"))
	}

	reqLogger := logger.DeriveRequestLogger(ctx, s.Logger)
	secretEnvVars := make(map[string]string, len(secretNames))
	seen := make(map[string]struct{}, len(secretNames))

	for _, rawName := range secretNames {
		name := strings.TrimSpace(rawName)
		if name == "" {
			return nil, apperrors.ErrBadRequest("secret names cannot be empty", nil)
		}
		if _, alreadyProcessed := seen[name]; alreadyProcessed {
			continue
		}
		seen[name] = struct{}{}

		secret, err := s.secretsRepo.GetSecret(ctx, name, true)
		if err != nil {
			if errors.Is(err, database.ErrSecretNotFound) {
				return nil, apperrors.ErrBadRequest(fmt.Sprintf("secret %q not found", name), err)
			}
			return nil, err
		}
		if secret == nil {
			return nil, apperrors.ErrBadRequest(fmt.Sprintf("secret %q not found", name), nil)
		}

		keyName := strings.TrimSpace(secret.KeyName)
		if keyName == "" {
			return nil, apperrors.ErrInternalError(
				fmt.Sprintf("secret %q has no key name configured", name),
				fmt.Errorf("missing key name"))
		}

		secretEnvVars[keyName] = secret.Value
	}

	reqLogger.Debug("resolved secrets for execution", "context", map[string]string{
		"secret_count": fmt.Sprintf("%d", len(secretEnvVars)),
	})

	return secretEnvVars, nil
}

func (s *Service) applyResolvedSecrets(req *api.ExecutionRequest, secretEnvVars map[string]string) {
	if req == nil || len(secretEnvVars) == 0 {
		return
	}

	if req.Env == nil {
		req.Env = make(map[string]string, len(secretEnvVars))
	}
	for key, value := range secretEnvVars {
		if _, exists := req.Env[key]; exists {
			continue
		}
		req.Env[key] = value
	}
}
