package aws

import (
	"context"

	"runvoy/internal/api"
	"runvoy/internal/database"
)

// SecretsManagerAdapter adapts database.SecretsRepository to implement app.SecretsManager.
// This allows the unified repository interface to be used as the secrets manager.
type SecretsManagerAdapter struct {
	repo database.SecretsRepository
}

// NewSecretsManagerAdapter creates a new adapter.
func NewSecretsManagerAdapter(repo database.SecretsRepository) *SecretsManagerAdapter {
	return &SecretsManagerAdapter{repo: repo}
}

// CreateSecret delegates to the repository.
// The secret's CreatedBy field must be pre-populated by the caller.
func (sma *SecretsManagerAdapter) CreateSecret(
	ctx context.Context,
	secret *api.Secret,
) error {
	return sma.repo.CreateSecret(ctx, secret)
}

// GetSecret delegates to the repository.
// If includeValue is true, the secret value will be decrypted and included in the response.
func (sma *SecretsManagerAdapter) GetSecret(ctx context.Context, name string, includeValue bool) (*api.Secret, error) {
	secret, err := sma.repo.GetSecret(ctx, name)
	if err != nil {
		return nil, err
	}
	if !includeValue {
		secret.Value = ""
	}
	return secret, nil
}

// ListSecrets delegates to the repository.
// If includeValue is true, secret values will be decrypted and included in the response.
func (sma *SecretsManagerAdapter) ListSecrets(ctx context.Context, includeValue bool) ([]*api.Secret, error) {
	secrets, err := sma.repo.ListSecrets(ctx)
	if err != nil {
		return nil, err
	}
	if !includeValue {
		for _, secret := range secrets {
			secret.Value = ""
		}
	}
	return secrets, nil
}

// UpdateSecret delegates to the repository.
// The secret's UpdatedBy field must be pre-populated by the caller.
func (sma *SecretsManagerAdapter) UpdateSecret(
	ctx context.Context,
	secret *api.Secret,
) error {
	updateReq := &api.UpdateSecretRequest{
		Description: secret.Description,
		KeyName:     secret.KeyName,
		Value:       secret.Value,
	}
	return sma.repo.UpdateSecret(ctx, secret.Name, updateReq, secret.UpdatedBy)
}

// DeleteSecret delegates to the repository.
func (sma *SecretsManagerAdapter) DeleteSecret(ctx context.Context, name string) error {
	return sma.repo.DeleteSecret(ctx, name)
}
