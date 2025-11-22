package database

import (
	"context"
	"errors"
	"testing"

	"runvoy/internal/api"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMetadataRepository is a mock implementation of the DynamoDB metadata repository
type mockMetadataRepository struct {
	secrets         map[string]*api.Secret
	createErr       error
	getErr          error
	updateErr       error
	deleteErr       error
	listErr         error
	secretExistsErr error
}

func newMockMetadataRepository() *mockMetadataRepository {
	return &mockMetadataRepository{
		secrets: make(map[string]*api.Secret),
	}
}

func (m *mockMetadataRepository) CreateSecret(_ context.Context, secret *api.Secret) error {
	if m.createErr != nil {
		return m.createErr
	}
	// Make a copy to avoid external modifications, and set UpdatedBy to CreatedBy if not set
	secretCopy := *secret
	if secretCopy.UpdatedBy == "" {
		secretCopy.UpdatedBy = secretCopy.CreatedBy
	}
	m.secrets[secret.Name] = &secretCopy
	return nil
}

func (m *mockMetadataRepository) GetSecret(_ context.Context, name string) (*api.Secret, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	secret, ok := m.secrets[name]
	if !ok {
		return nil, appErrors.ErrSecretNotFound("secret not found", nil)
	}
	// Return a copy to prevent external modifications
	secretCopy := *secret
	return &secretCopy, nil
}

func (m *mockMetadataRepository) ListSecrets(_ context.Context) ([]*api.Secret, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	list := make([]*api.Secret, 0, len(m.secrets))
	for _, s := range m.secrets {
		secretCopy := *s
		list = append(list, &secretCopy)
	}
	return list, nil
}

func (m *mockMetadataRepository) UpdateSecretMetadata(
	_ context.Context, name, keyName, description, updatedBy string,
) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	secret, ok := m.secrets[name]
	if !ok {
		return appErrors.ErrSecretNotFound("secret not found", nil)
	}
	secret.KeyName = keyName
	secret.Description = description
	secret.UpdatedBy = updatedBy
	return nil
}

func (m *mockMetadataRepository) DeleteSecret(_ context.Context, name string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.secrets, name)
	return nil
}

func (m *mockMetadataRepository) SecretExists(_ context.Context, name string) (bool, error) {
	if m.secretExistsErr != nil {
		return false, m.secretExistsErr
	}
	_, exists := m.secrets[name]
	return exists, nil
}

func (m *mockMetadataRepository) GetSecretsByRequestID(_ context.Context, _ string) ([]*api.Secret, error) {
	// Return all secrets for testing purposes
	list := make([]*api.Secret, 0, len(m.secrets))
	for _, s := range m.secrets {
		secretCopy := *s
		list = append(list, &secretCopy)
	}
	return list, nil
}

// mockValueStore is a mock implementation of the ValueStore interface
type mockValueStore struct {
	values      map[string]string
	storeErr    error
	retrieveErr error
	deleteErr   error
}

func newMockValueStore() *mockValueStore {
	return &mockValueStore{
		values: make(map[string]string),
	}
}

func (m *mockValueStore) StoreSecret(_ context.Context, name, value string) error {
	if m.storeErr != nil {
		return m.storeErr
	}
	m.values[name] = value
	return nil
}

func (m *mockValueStore) RetrieveSecret(_ context.Context, name string) (string, error) {
	if m.retrieveErr != nil {
		return "", m.retrieveErr
	}
	value, ok := m.values[name]
	if !ok {
		return "", errors.New("secret not found")
	}
	return value, nil
}

func (m *mockValueStore) DeleteSecret(_ context.Context, name string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.values, name)
	return nil
}

func TestUpdateSecret_PreservesMetadataWhenNotProvided(t *testing.T) {
	metadataRepo := newMockMetadataRepository()
	valueStore := newMockValueStore()
	logger := testutil.SilentLogger()

	repo := NewSecretsRepository(metadataRepo, valueStore, logger)

	// Create an initial secret with all metadata
	originalSecret := &api.Secret{
		Name:        "test-secret",
		KeyName:     "ORIGINAL_KEY",
		Description: "Original description",
		CreatedBy:   "admin@example.com",
		UpdatedBy:   "admin@example.com",
	}
	err := metadataRepo.CreateSecret(context.Background(), originalSecret)
	require.NoError(t, err)

	// Store the original value
	err = valueStore.StoreSecret(context.Background(), "test-secret", "original-value")
	require.NoError(t, err)

	t.Run("updating only value preserves description and keyName", func(t *testing.T) {
		updateSecret := &api.Secret{
			Name:      "test-secret",
			Value:     "new-value",
			UpdatedBy: "user@example.com",
			// KeyName and Description are empty
		}

		updateErr := repo.UpdateSecret(context.Background(), updateSecret)
		require.NoError(t, updateErr)

		// Verify the metadata was preserved
		retrieved, getErr := metadataRepo.GetSecret(context.Background(), "test-secret")
		require.NoError(t, getErr)
		assert.Equal(t, "ORIGINAL_KEY", retrieved.KeyName, "KeyName should be preserved")
		assert.Equal(t, "Original description", retrieved.Description, "Description should be preserved")
		assert.Equal(t, "user@example.com", retrieved.UpdatedBy, "UpdatedBy should be updated")

		// Verify the value was updated
		value, retrieveErr := valueStore.RetrieveSecret(context.Background(), "test-secret")
		require.NoError(t, retrieveErr)
		assert.Equal(t, "new-value", value)
	})

	t.Run("updating only description preserves keyName", func(t *testing.T) {
		updateSecret := &api.Secret{
			Name:        "test-secret",
			Description: "New description",
			UpdatedBy:   "user2@example.com",
			// KeyName is empty, Value is empty
		}

		updateErr := repo.UpdateSecret(context.Background(), updateSecret)
		require.NoError(t, updateErr)

		// Verify the metadata
		retrieved, getErr := metadataRepo.GetSecret(context.Background(), "test-secret")
		require.NoError(t, getErr)
		assert.Equal(t, "ORIGINAL_KEY", retrieved.KeyName, "KeyName should be preserved")
		assert.Equal(t, "New description", retrieved.Description, "Description should be updated")
		assert.Equal(t, "user2@example.com", retrieved.UpdatedBy)
	})

	t.Run("updating only keyName preserves description", func(t *testing.T) {
		updateSecret := &api.Secret{
			Name:      "test-secret",
			KeyName:   "NEW_KEY",
			UpdatedBy: "user3@example.com",
			// Description is empty, Value is empty
		}

		updateErr := repo.UpdateSecret(context.Background(), updateSecret)
		require.NoError(t, updateErr)

		// Verify the metadata
		retrieved, getErr := metadataRepo.GetSecret(context.Background(), "test-secret")
		require.NoError(t, getErr)
		assert.Equal(t, "NEW_KEY", retrieved.KeyName, "KeyName should be updated")
		assert.Equal(t, "New description", retrieved.Description, "Description should be preserved from previous update")
		assert.Equal(t, "user3@example.com", retrieved.UpdatedBy)
	})

	t.Run("updating all fields works correctly", func(t *testing.T) {
		updateSecret := &api.Secret{
			Name:        "test-secret",
			KeyName:     "COMPLETE_KEY",
			Description: "Complete description",
			Value:       "complete-value",
			UpdatedBy:   "user4@example.com",
		}

		updateErr := repo.UpdateSecret(context.Background(), updateSecret)
		require.NoError(t, updateErr)

		// Verify the metadata
		retrieved, getErr := metadataRepo.GetSecret(context.Background(), "test-secret")
		require.NoError(t, getErr)
		assert.Equal(t, "COMPLETE_KEY", retrieved.KeyName)
		assert.Equal(t, "Complete description", retrieved.Description)
		assert.Equal(t, "user4@example.com", retrieved.UpdatedBy)

		// Verify the value
		value, retrieveErr := valueStore.RetrieveSecret(context.Background(), "test-secret")
		require.NoError(t, retrieveErr)
		assert.Equal(t, "complete-value", value)
	})
}

func TestUpdateSecret_ValueUpdateError(t *testing.T) {
	metadataRepo := newMockMetadataRepository()
	valueStore := newMockValueStore()
	logger := testutil.SilentLogger()

	repo := NewSecretsRepository(metadataRepo, valueStore, logger)

	// Create an initial secret
	originalSecret := &api.Secret{
		Name:        "test-secret",
		KeyName:     "TEST_KEY",
		Description: "Test description",
		CreatedBy:   "admin@example.com",
	}
	err := metadataRepo.CreateSecret(context.Background(), originalSecret)
	require.NoError(t, err)

	// Inject a store error
	valueStore.storeErr = errors.New("store failed")

	updateSecret := &api.Secret{
		Name:      "test-secret",
		Value:     "new-value",
		UpdatedBy: "user@example.com",
	}

	err = repo.UpdateSecret(context.Background(), updateSecret)
	assert.Error(t, err)

	// Metadata should not be updated when value update fails
	retrieved, err := metadataRepo.GetSecret(context.Background(), "test-secret")
	require.NoError(t, err)
	assert.Equal(t, "admin@example.com", retrieved.UpdatedBy, "Metadata should not be updated when value update fails")
}

func TestUpdateSecret_GetExistingSecretError(t *testing.T) {
	metadataRepo := newMockMetadataRepository()
	valueStore := newMockValueStore()
	logger := testutil.SilentLogger()

	repo := NewSecretsRepository(metadataRepo, valueStore, logger)

	// Inject a get error
	metadataRepo.getErr = errors.New("get failed")

	updateSecret := &api.Secret{
		Name:        "test-secret",
		Description: "New description",
		UpdatedBy:   "user@example.com",
	}

	err := repo.UpdateSecret(context.Background(), updateSecret)
	assert.Error(t, err)
}

func TestUpdateSecret_SecretNotFound(t *testing.T) {
	metadataRepo := newMockMetadataRepository()
	valueStore := newMockValueStore()
	logger := testutil.SilentLogger()

	repo := NewSecretsRepository(metadataRepo, valueStore, logger)

	updateSecret := &api.Secret{
		Name:        "nonexistent-secret",
		Description: "New description",
		UpdatedBy:   "user@example.com",
	}

	err := repo.UpdateSecret(context.Background(), updateSecret)
	assert.Error(t, err)
}

func TestUpdateSecret_MetadataUpdateError(t *testing.T) {
	metadataRepo := newMockMetadataRepository()
	valueStore := newMockValueStore()
	logger := testutil.SilentLogger()

	repo := NewSecretsRepository(metadataRepo, valueStore, logger)

	// Create an initial secret
	originalSecret := &api.Secret{
		Name:        "test-secret",
		KeyName:     "TEST_KEY",
		Description: "Test description",
		CreatedBy:   "admin@example.com",
	}
	err := metadataRepo.CreateSecret(context.Background(), originalSecret)
	require.NoError(t, err)

	// Inject an update error
	metadataRepo.updateErr = errors.New("update failed")

	updateSecret := &api.Secret{
		Name:        "test-secret",
		Description: "New description",
		UpdatedBy:   "user@example.com",
	}

	err = repo.UpdateSecret(context.Background(), updateSecret)
	assert.Error(t, err)
}

func TestUpdateSecret_EmptyValueDoesNotUpdateValue(t *testing.T) {
	metadataRepo := newMockMetadataRepository()
	valueStore := newMockValueStore()
	logger := testutil.SilentLogger()

	repo := NewSecretsRepository(metadataRepo, valueStore, logger)

	// Create an initial secret with a value
	originalSecret := &api.Secret{
		Name:        "test-secret",
		KeyName:     "TEST_KEY",
		Description: "Test description",
		CreatedBy:   "admin@example.com",
	}
	err := metadataRepo.CreateSecret(context.Background(), originalSecret)
	require.NoError(t, err)

	err = valueStore.StoreSecret(context.Background(), "test-secret", "original-value")
	require.NoError(t, err)

	// Update with empty value
	updateSecret := &api.Secret{
		Name:        "test-secret",
		Description: "New description",
		UpdatedBy:   "user@example.com",
		// Value is empty
	}

	err = repo.UpdateSecret(context.Background(), updateSecret)
	require.NoError(t, err)

	// Verify the value was not changed
	value, err := valueStore.RetrieveSecret(context.Background(), "test-secret")
	require.NoError(t, err)
	assert.Equal(t, "original-value", value, "Value should remain unchanged when empty value is provided")
}
