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
) (*api.Secret, error) {
	if err := sma.repo.CreateSecret(ctx, secret); err != nil {
		return nil, err
	}
	return sma.repo.GetSecret(ctx, secret.Name)
}

// GetSecret delegates to the repository.
func (sma *SecretsManagerAdapter) GetSecret(ctx context.Context, name string) (*api.Secret, error) {
	return sma.repo.GetSecret(ctx, name)
}

// ListSecrets delegates to the repository.
func (sma *SecretsManagerAdapter) ListSecrets(ctx context.Context) ([]*api.Secret, error) {
	return sma.repo.ListSecrets(ctx)
}

// UpdateSecret delegates to the repository.
func (sma *SecretsManagerAdapter) UpdateSecret(
	ctx context.Context,
	name string,
	req *api.UpdateSecretRequest,
	userEmail string,
) (*api.Secret, error) {
	if err := sma.repo.UpdateSecret(ctx, name, req, userEmail); err != nil {
		return nil, err
	}
	return sma.repo.GetSecret(ctx, name)
}

// DeleteSecret delegates to the repository.
func (sma *SecretsManagerAdapter) DeleteSecret(ctx context.Context, name string) error {
	return sma.repo.DeleteSecret(ctx, name)
}
