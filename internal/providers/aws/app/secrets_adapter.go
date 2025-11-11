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
func (sma *SecretsManagerAdapter) CreateSecret(
	ctx context.Context,
	req *api.CreateSecretRequest,
	userEmail string,
) (*api.Secret, error) {
	if err := sma.repo.CreateSecret(ctx, req.Name, req.KeyName, req.Description, req.Value, userEmail); err != nil {
		return nil, err
	}
	return sma.repo.GetSecret(ctx, req.Name)
}

// GetSecret delegates to the repository.
func (sma *SecretsManagerAdapter) GetSecret(ctx context.Context, name string) (*api.Secret, error) {
	return sma.repo.GetSecret(ctx, name)
}

// ListSecrets delegates to the repository.
func (sma *SecretsManagerAdapter) ListSecrets(ctx context.Context, userEmail string) ([]*api.Secret, error) {
	return sma.repo.ListSecrets(ctx, userEmail)
}

// UpdateSecret delegates to the repository.
func (sma *SecretsManagerAdapter) UpdateSecret(
	ctx context.Context,
	name string,
	req *api.UpdateSecretRequest,
	userEmail string,
) (*api.Secret, error) {
	if err := sma.repo.UpdateSecret(ctx, name, req.KeyName, req.Description, req.Value, userEmail); err != nil {
		return nil, err
	}
	return sma.repo.GetSecret(ctx, name)
}

// DeleteSecret delegates to the repository.
func (sma *SecretsManagerAdapter) DeleteSecret(ctx context.Context, name string) error {
	return sma.repo.DeleteSecret(ctx, name)
}
