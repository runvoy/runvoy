package orchestrator

import (
	"context"
	"errors"
	"testing"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
	"runvoy/internal/constants"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSecret_Success(t *testing.T) {
	secretsRepo := &mockSecretsRepository{}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	if err != nil {
		t.Fatal(err)
	}

	req := &api.CreateSecretRequest{
		Name:        "test-secret",
		KeyName:     "TEST_KEY",
		Description: "Test secret",
		Value:       "secret-value",
	}

	createErr := service.CreateSecret(context.Background(), req, "user@example.com")

	assert.NoError(t, createErr)
}

func TestCreateSecret_NoRepository(t *testing.T) {
	logger := testutil.SilentLogger()

	// Note: secretsRepo is now required for service initialization.
	// This test verifies that CreateSecret works with a valid repository.
	secretsRepo := &mockSecretsRepository{}

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	if err != nil {
		t.Fatal(err)
	}

	req := &api.CreateSecretRequest{
		Name:    "test-secret",
		KeyName: "TEST_KEY",
		Value:   "secret-value",
	}

	createErr := service.CreateSecret(context.Background(), req, "user@example.com")

	// With a valid repository, CreateSecret should succeed
	assert.NoError(t, createErr)
}

func TestCreateSecret_RepositoryError(t *testing.T) {
	secretsRepo := &mockSecretsRepository{
		createSecretFunc: func(_ context.Context, _ *api.Secret) error {
			return appErrors.ErrDatabaseError("test error", errors.New("db error"))
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	if err != nil {
		t.Fatal(err)
	}

	req := &api.CreateSecretRequest{
		Name:    "test-secret",
		KeyName: "TEST_KEY",
		Value:   "secret-value",
	}

	createErr := service.CreateSecret(context.Background(), req, "user@example.com")

	assert.Error(t, createErr)
}

func TestGetSecret_Success(t *testing.T) {
	secretsRepo := &mockSecretsRepository{
		getSecretFunc: func(_ context.Context, name string, _ bool) (*api.Secret, error) {
			return &api.Secret{
				Name:        name,
				KeyName:     "TEST_KEY",
				Description: "Test secret",
				Value:       "secret-value",
				CreatedBy:   "user@example.com",
			}, nil
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	secret, err := service.GetSecret(context.Background(), "test-secret")

	assert.NoError(t, err)
	require.NotNil(t, secret)
	assert.Equal(t, "test-secret", secret.Name)
	assert.Equal(t, "secret-value", secret.Value)
}

func TestGetSecret_NotFound(t *testing.T) {
	secretsRepo := &mockSecretsRepository{
		getSecretFunc: func(_ context.Context, _ string, _ bool) (*api.Secret, error) {
			return nil, nil
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	secret, err := service.GetSecret(context.Background(), "nonexistent")

	assert.NoError(t, err)
	assert.Nil(t, secret)
}

func TestListSecrets_Success(t *testing.T) {
	secretsRepo := &mockSecretsRepository{
		listSecretsFunc: func(_ context.Context, includeValue bool) ([]*api.Secret, error) {
			// During initialization, includeValue is false. Return empty list to avoid ownership loading.
			// During actual ListSecrets call, includeValue is true. Return the test secrets.
			if !includeValue {
				return []*api.Secret{}, nil
			}
			return []*api.Secret{
				{
					Name:        "secret-1",
					KeyName:     "KEY_1",
					Description: "First secret",
					Value:       "value1",
					CreatedBy:   "user@example.com",
				},
				{
					Name:        "secret-2",
					KeyName:     "KEY_2",
					Description: "Second secret",
					Value:       "value2",
					CreatedBy:   "user@example.com",
				},
			}, nil
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	secrets, err := service.ListSecrets(context.Background())

	assert.NoError(t, err)
	assert.Len(t, secrets, 2)
	assert.Equal(t, "secret-1", secrets[0].Name)
	assert.Equal(t, "secret-2", secrets[1].Name)
}

func TestListSecrets_Empty(t *testing.T) {
	secretsRepo := &mockSecretsRepository{
		listSecretsFunc: func(_ context.Context, _ bool) ([]*api.Secret, error) {
			return []*api.Secret{}, nil
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	secrets, err := service.ListSecrets(context.Background())

	assert.NoError(t, err)
	assert.Len(t, secrets, 0)
}

func TestUpdateSecret_Success(t *testing.T) {
	secretsRepo := &mockSecretsRepository{}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	if err != nil {
		t.Fatal(err)
	}

	req := &api.UpdateSecretRequest{
		KeyName:     "UPDATED_KEY",
		Description: "Updated description",
		Value:       "new-value",
	}

	updateErr := service.UpdateSecret(context.Background(), "test-secret", req, "user@example.com")

	assert.NoError(t, updateErr)
}

func TestUpdateSecret_RepositoryError(t *testing.T) {
	secretsRepo := &mockSecretsRepository{
		updateSecretFunc: func(_ context.Context, _ *api.Secret) error {
			return appErrors.ErrDatabaseError("test error", errors.New("db error"))
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	if err != nil {
		t.Fatal(err)
	}

	req := &api.UpdateSecretRequest{
		KeyName: "UPDATED_KEY",
		Value:   "new-value",
	}

	updateErr := service.UpdateSecret(context.Background(), "test-secret", req, "user@example.com")

	assert.Error(t, updateErr)
}

func TestDeleteSecret_Success(t *testing.T) {
	secretsRepo := &mockSecretsRepository{}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	if err != nil {
		t.Fatal(err)
	}

	deleteErr := service.DeleteSecret(context.Background(), "test-secret")

	assert.NoError(t, deleteErr)
}

func TestDeleteSecret_RepositoryError(t *testing.T) {
	secretsRepo := &mockSecretsRepository{
		deleteSecretFunc: func(_ context.Context, _ string) error {
			return appErrors.ErrDatabaseError("test error", errors.New("db error"))
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	if err != nil {
		t.Fatal(err)
	}

	deleteErr := service.DeleteSecret(context.Background(), "test-secret")

	assert.Error(t, deleteErr)
}

func TestDeleteSecret_RemovesOwnershipFromEnforcer(t *testing.T) {
	ctx := context.Background()
	secrets := map[string]*api.Secret{}
	secretsRepo := &mockSecretsRepository{
		createSecretFunc: func(_ context.Context, secret *api.Secret) error {
			secrets[secret.Name] = secret
			return nil
		},
		getSecretFunc: func(_ context.Context, name string, _ bool) (*api.Secret, error) {
			if secret, ok := secrets[name]; ok {
				secretCopy := *secret
				return &secretCopy, nil
			}
			return nil, nil
		},
		deleteSecretFunc: func(_ context.Context, name string) error {
			if _, ok := secrets[name]; !ok {
				return appErrors.ErrNotFound("secret not found", nil)
			}
			delete(secrets, name)
			return nil
		},
	}

	service, enforcer := newTestServiceWithEnforcer(
		&mockUserRepository{},
		&mockExecutionRepository{},
		nil,
		secretsRepo,
	)

	req := &api.CreateSecretRequest{
		Name:    "ownership-secret",
		KeyName: "SECRET_KEY",
		Value:   "super-secret",
	}

	err := service.CreateSecret(ctx, req, "creator@example.com")
	require.NoError(t, err)

	resourceID := authorization.FormatResourceID("secret", req.Name)
	hasOwnership, checkErr := enforcer.HasOwnershipForResource(resourceID, "creator@example.com")
	require.NoError(t, checkErr)
	assert.True(t, hasOwnership)

	deleteErr := service.DeleteSecret(ctx, req.Name)
	require.NoError(t, deleteErr)

	hasOwnership, checkErr = enforcer.HasOwnershipForResource(resourceID, "creator@example.com")
	require.NoError(t, checkErr)
	assert.False(t, hasOwnership)
}

func TestDeleteSecret_RestoresOwnershipOnFailure(t *testing.T) {
	ctx := context.Background()
	secrets := map[string]*api.Secret{}
	secretsRepo := &mockSecretsRepository{
		createSecretFunc: func(_ context.Context, secret *api.Secret) error {
			secrets[secret.Name] = secret
			return nil
		},
		getSecretFunc: func(_ context.Context, name string, _ bool) (*api.Secret, error) {
			if secret, ok := secrets[name]; ok {
				secretCopy := *secret
				return &secretCopy, nil
			}
			return nil, nil
		},
		deleteSecretFunc: func(_ context.Context, name string) error {
			if _, ok := secrets[name]; !ok {
				return appErrors.ErrNotFound("secret not found", nil)
			}
			return appErrors.ErrDatabaseError("delete failed", errors.New("db error"))
		},
	}

	service, enforcer := newTestServiceWithEnforcer(
		&mockUserRepository{},
		&mockExecutionRepository{},
		nil,
		secretsRepo,
	)

	req := &api.CreateSecretRequest{
		Name:    "restore-secret",
		KeyName: "SECRET_KEY",
		Value:   "super-secret",
	}

	err := service.CreateSecret(ctx, req, "creator@example.com")
	require.NoError(t, err)

	resourceID := authorization.FormatResourceID("secret", req.Name)
	hasOwnership, checkErr := enforcer.HasOwnershipForResource(resourceID, "creator@example.com")
	require.NoError(t, checkErr)
	assert.True(t, hasOwnership)

	deleteErr := service.DeleteSecret(ctx, req.Name)
	require.Error(t, deleteErr)

	hasOwnership, checkErr = enforcer.HasOwnershipForResource(resourceID, "creator@example.com")
	require.NoError(t, checkErr)
	assert.True(t, hasOwnership)
}

func TestResolveSecretsForExecution_Success(t *testing.T) {
	secretsRepo := &mockSecretsRepository{
		getSecretFunc: func(_ context.Context, name string, _ bool) (*api.Secret, error) {
			if name == "secret-1" {
				return &api.Secret{
					Name:    "secret-1",
					KeyName: "SECRET_1",
					Value:   "value1",
				}, nil
			}
			if name == "secret-2" {
				return &api.Secret{
					Name:    "secret-2",
					KeyName: "SECRET_2",
					Value:   "value2",
				}, nil
			}
			return nil, nil
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	secretEnvVars, err := service.resolveSecretsForExecution(context.Background(), []string{"secret-1", "secret-2"})

	assert.NoError(t, err)
	assert.Len(t, secretEnvVars, 2)
	assert.Equal(t, "value1", secretEnvVars["SECRET_1"])
	assert.Equal(t, "value2", secretEnvVars["SECRET_2"])
}

func TestResolveSecretsForExecution_Empty(t *testing.T) {
	secretsRepo := &mockSecretsRepository{}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	secretEnvVars, err := service.resolveSecretsForExecution(context.Background(), []string{})

	assert.NoError(t, err)
	assert.Nil(t, secretEnvVars)
}

func TestResolveSecretsForExecution_EmptySecretName(t *testing.T) {
	secretsRepo := &mockSecretsRepository{}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	secretEnvVars, err := service.resolveSecretsForExecution(context.Background(), []string{"  "})

	assert.Error(t, err)
	assert.Nil(t, secretEnvVars)
	assert.Contains(t, err.Error(), "secret names cannot be empty")
}

func TestResolveSecretsForExecution_DuplicateSecrets(t *testing.T) {
	callCount := 0
	secretsRepo := &mockSecretsRepository{
		getSecretFunc: func(_ context.Context, _ string, _ bool) (*api.Secret, error) {
			callCount++
			return &api.Secret{
				Name:    "secret-1",
				KeyName: "SECRET_1",
				Value:   "value1",
			}, nil
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	// Pass the same secret twice
	secretEnvVars, err := service.resolveSecretsForExecution(
		context.Background(),
		[]string{"secret-1", "secret-1", "secret-1"},
	)

	assert.NoError(t, err)
	assert.Len(t, secretEnvVars, 1)
	assert.Equal(t, "value1", secretEnvVars["SECRET_1"])
	// Should only call repository once due to deduplication
	assert.Equal(t, 1, callCount)
}

func TestResolveSecretsForExecution_SecretNotFound(t *testing.T) {
	secretsRepo := &mockSecretsRepository{
		getSecretFunc: func(_ context.Context, _ string, _ bool) (*api.Secret, error) {
			return nil, nil // Not found
		},
	}
	logger := testutil.SilentLogger()

	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil, // wsManager
		secretsRepo,
		nil, // healthManager
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	secretEnvVars, err := service.resolveSecretsForExecution(context.Background(), []string{"secret-1"})

	assert.Error(t, err)
	assert.Nil(t, secretEnvVars)
	assert.Contains(t, err.Error(), "secret \"secret-1\" not found")
}

func TestApplyResolvedSecrets(t *testing.T) {
	tests := []struct {
		name          string
		req           *api.ExecutionRequest
		secretEnvVars map[string]string
		expectedEnv   map[string]string
	}{
		{
			name:          "apply to empty env",
			req:           &api.ExecutionRequest{},
			secretEnvVars: map[string]string{"KEY1": "value1", "KEY2": "value2"},
			expectedEnv:   map[string]string{"KEY1": "value1", "KEY2": "value2"},
		},
		{
			name:          "apply to existing env without conflicts",
			req:           &api.ExecutionRequest{Env: map[string]string{"KEY3": "value3"}},
			secretEnvVars: map[string]string{"KEY1": "value1"},
			expectedEnv:   map[string]string{"KEY1": "value1", "KEY3": "value3"},
		},
		{
			name:          "skip conflicting env vars",
			req:           &api.ExecutionRequest{Env: map[string]string{"KEY1": "existing"}},
			secretEnvVars: map[string]string{"KEY1": "secret"},
			expectedEnv:   map[string]string{"KEY1": "existing"},
		},
		{
			name:          "nil request",
			req:           nil,
			secretEnvVars: map[string]string{"KEY1": "value1"},
			expectedEnv:   nil,
		},
		{
			name:          "empty secrets",
			req:           &api.ExecutionRequest{},
			secretEnvVars: map[string]string{},
			expectedEnv:   nil,
		},
	}

	logger := testutil.SilentLogger()
	service, err := NewService(context.Background(),
		&mockUserRepository{},
		&mockExecutionRepository{},
		&mockConnectionRepository{},
		&mockTokenRepository{},
		&mockRunner{},
		logger,
		constants.AWS,
		nil,                      // wsManager
		&mockSecretsRepository{}, // secretsRepo
		nil,                      // healthManager
		newPermissiveEnforcer(),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service.applyResolvedSecrets(tt.req, tt.secretEnvVars)

			if tt.expectedEnv == nil {
				if tt.req != nil {
					assert.Nil(t, tt.req.Env)
				}
			} else {
				require.NotNil(t, tt.req)
				assert.Equal(t, tt.expectedEnv, tt.req.Env)
			}
		})
	}
}
