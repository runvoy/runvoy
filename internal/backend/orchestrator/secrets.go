package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
	"runvoy/internal/database"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"
	"runvoy/internal/secrets"
)

// CreateSecret creates a new secret with the given name, description, key name, and value.
func (s *Service) CreateSecret(
	ctx context.Context,
	req *api.CreateSecretRequest,
	userEmail string,
) error {
	// Extract request ID from context
	requestID := logger.GetRequestID(ctx)

	secret := &api.Secret{
		Name:                req.Name,
		KeyName:             req.KeyName,
		Description:         req.Description,
		Value:               req.Value,
		CreatedBy:           userEmail,
		OwnedBy:             []string{userEmail},
		CreatedByRequestID:  requestID,
		ModifiedByRequestID: requestID,
	}
	if err := s.secretsRepo.CreateSecret(ctx, secret); err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			return err // Already wrapped, pass through
		}
		return apperrors.ErrInternalError("failed to create secret", fmt.Errorf("create secret: %w", err))
	}

	enforcer := s.GetEnforcer()
	resourceID := authorization.FormatResourceID("secret", req.Name)
	for _, owner := range secret.OwnedBy {
		if err := enforcer.AddOwnershipForResource(resourceID, owner); err != nil {
			// Rollback secret creation if enforcer update fails
			if deleteErr := s.secretsRepo.DeleteSecret(ctx, req.Name); deleteErr != nil {
				s.Logger.Error("failed to rollback secret creation after enforcer error",
					"error", deleteErr,
					"resource", resourceID,
					"owner", owner,
				)
			}
			return apperrors.ErrInternalError("failed to add secret ownership to authorization enforcer", err)
		}
	}

	return nil
}

// GetSecret retrieves a secret's metadata and value by name.
func (s *Service) GetSecret(ctx context.Context, name string) (*api.Secret, error) {
	return s.secretsRepo.GetSecret(ctx, name, true)
}

// ListSecrets retrieves all secrets with values.
func (s *Service) ListSecrets(ctx context.Context) ([]*api.Secret, error) {
	return s.secretsRepo.ListSecrets(ctx, true)
}

// UpdateSecret updates a secret (metadata and/or value).
func (s *Service) UpdateSecret(
	ctx context.Context,
	name string,
	req *api.UpdateSecretRequest,
	userEmail string,
) error {
	// Extract request ID from context
	requestID := logger.GetRequestID(ctx)

	secret := &api.Secret{
		Name:                name,
		Description:         req.Description,
		KeyName:             req.KeyName,
		Value:               req.Value,
		UpdatedBy:           userEmail,
		ModifiedByRequestID: requestID,
	}
	if err := s.secretsRepo.UpdateSecret(ctx, secret); err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			return err // Already wrapped, pass through
		}
		return apperrors.ErrInternalError("failed to update secret", fmt.Errorf("update secret: %w", err))
	}
	return nil
}

// DeleteSecret deletes a secret and its value.
func (s *Service) DeleteSecret(ctx context.Context, name string) error {
	resourceID := authorization.FormatResourceID("secret", name)
	secret, fetchErr := s.secretsRepo.GetSecret(ctx, name, false)
	if fetchErr != nil {
		var appErr *apperrors.AppError
		if errors.As(fetchErr, &appErr) {
			return fetchErr
		}
		return apperrors.ErrInternalError("failed to load secret metadata", fmt.Errorf("get secret: %w", fetchErr))
	}

	var ownerEmails []string
	if secret != nil && len(secret.OwnedBy) > 0 {
		ownerEmails = secret.OwnedBy
		for _, ownerEmail := range ownerEmails {
			if removeErr := s.enforcer.RemoveOwnershipForResource(resourceID, ownerEmail); removeErr != nil {
				return apperrors.ErrInternalError("failed to remove secret ownership from authorization enforcer", removeErr)
			}
		}
	}

	if deleteErr := s.secretsRepo.DeleteSecret(ctx, name); deleteErr != nil {
		// Rollback: restore ownership if delete failed
		for _, ownerEmail := range ownerEmails {
			if addErr := s.enforcer.AddOwnershipForResource(resourceID, ownerEmail); addErr != nil {
				return apperrors.ErrInternalError("failed to restore secret ownership after delete error", addErr)
			}
		}
		var appErr *apperrors.AppError
		if errors.As(deleteErr, &appErr) {
			return deleteErr // Already wrapped, pass through
		}
		return apperrors.ErrInternalError("failed to delete secret", fmt.Errorf("delete secret: %w", deleteErr))
	}

	return nil
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
			return nil, apperrors.ErrInternalError("failed to retrieve secret", fmt.Errorf("get secret %q: %w", name, err))
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

// applyResolvedSecrets merges resolved secrets into the request environment and populates
// SecretVarNames with both explicitly resolved and pattern-detected secret variable names.
func (s *Service) applyResolvedSecrets(req *api.ExecutionRequest, secretEnvVars map[string]string) {
	if req == nil {
		return
	}

	knownSecretVarNames := make([]string, 0, len(secretEnvVars))
	for key := range secretEnvVars {
		knownSecretVarNames = append(knownSecretVarNames, key)
	}

	if len(secretEnvVars) > 0 {
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

	detectedSecretVarNames := secrets.GetSecretVariableNames(req.Env)
	req.SecretVarNames = secrets.MergeSecretVarNames(knownSecretVarNames, detectedSecretVarNames)
}
