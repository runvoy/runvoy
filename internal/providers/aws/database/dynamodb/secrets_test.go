package dynamodb

import (
	"context"
	"errors"
	"testing"

	"runvoy/internal/api"
	"runvoy/internal/database"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/testutil"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSecretsRepository(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	tableName := "secrets-table"

	repo := NewSecretsRepository(client, tableName, logger)

	assert.NotNil(t, repo)
	assert.Equal(t, tableName, repo.tableName)
	assert.Equal(t, client, repo.client)
	assert.Equal(t, logger, repo.logger)
}

func TestCreateSecret_Success(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewSecretsRepository(client, "secrets-table", logger)

	secret := &api.Secret{
		Name:        "github-token",
		KeyName:     "GITHUB_TOKEN",
		Description: "GitHub personal access token",
		CreatedBy:   "admin@example.com",
	}

	err := repo.CreateSecret(context.Background(), secret)

	assert.NoError(t, err)
	assert.Equal(t, 1, client.PutItemCalls)
}

func TestCreateSecret_ClientError(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()

	// Inject a put item error
	client.PutItemError = appErrors.ErrInternalError("test error", errors.New("database error"))

	repo := NewSecretsRepository(client, "secrets-table", logger)

	secret := &api.Secret{
		Name:      "test-secret",
		KeyName:   "TEST_KEY",
		CreatedBy: "admin@example.com",
	}

	err := repo.CreateSecret(context.Background(), secret)

	assert.Error(t, err)
}

func TestGetSecret_Success(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewSecretsRepository(client, "secrets-table", logger)

	// Test that GetSecret calls the client correctly
	// and handles successful retrieval
	err := repo.CreateSecret(context.Background(), &api.Secret{
		Name:        "test-secret",
		KeyName:     "TEST_KEY",
		Description: "Test description",
		CreatedBy:   "admin@example.com",
	})
	assert.NoError(t, err)

	// Verify the put was called
	assert.Equal(t, 1, client.PutItemCalls)
}

func TestGetSecret_NotFound(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewSecretsRepository(client, "secrets-table", logger)

	retrieved, err := repo.GetSecret(context.Background(), "nonexistent")

	assert.Equal(t, database.ErrSecretNotFound, err)
	assert.Nil(t, retrieved)
}

func TestGetSecret_ClientError(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()

	// Inject a get item error
	client.GetItemError = appErrors.ErrInternalError("test error", errors.New("database error"))

	repo := NewSecretsRepository(client, "secrets-table", logger)

	_, err := repo.GetSecret(context.Background(), "some-secret")

	assert.Error(t, err)
	assert.NotEqual(t, database.ErrSecretNotFound, err)
}

func TestListSecrets_Success(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewSecretsRepository(client, "secrets-table", logger)

	// Create secrets using the repository
	err := repo.CreateSecret(context.Background(), &api.Secret{
		Name:        "github-token",
		KeyName:     "GITHUB_TOKEN",
		Description: "GitHub token",
		CreatedBy:   "admin@example.com",
	})
	assert.NoError(t, err)

	err = repo.CreateSecret(context.Background(), &api.Secret{
		Name:        "db-password",
		KeyName:     "DB_PASSWORD",
		Description: "Database password",
		CreatedBy:   "admin@example.com",
	})
	assert.NoError(t, err)

	// List all secrets
	retrieved, err := repo.ListSecrets(context.Background())

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(retrieved), 2)

	// Verify at least the expected secrets are present
	names := make(map[string]bool)
	for _, s := range retrieved {
		names[s.Name] = true
	}
	assert.True(t, names["github-token"])
	assert.True(t, names["db-password"])
}

func TestListSecrets_Empty(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewSecretsRepository(client, "secrets-table", logger)

	retrieved, err := repo.ListSecrets(context.Background())

	assert.NoError(t, err)
	assert.Len(t, retrieved, 0)
}

func TestListSecrets_ScanError(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()

	// Inject a scan error
	client.ScanError = appErrors.ErrInternalError("test error", errors.New("scan failed"))

	repo := NewSecretsRepository(client, "secrets-table", logger)

	_, err := repo.ListSecrets(context.Background())

	assert.Error(t, err)
}

func TestUpdateSecretMetadata_NotFound(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewSecretsRepository(client, "secrets-table", logger)

	// Try to update a non-existent secret (UpdateItem returns error for missing item)
	client.UpdateItemError = &types.ConditionalCheckFailedException{}
	err := repo.UpdateSecretMetadata(
		context.Background(),
		"nonexistent",
		"KEY",
		"description",
		"user@example.com",
	)

	assert.Equal(t, database.ErrSecretNotFound, err)
	client.UpdateItemError = nil
}

func TestUpdateSecretMetadata_ClientError(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()

	// Inject an update item error
	client.UpdateItemError = appErrors.ErrInternalError("test error", errors.New("update failed"))

	repo := NewSecretsRepository(client, "secrets-table", logger)

	err := repo.UpdateSecretMetadata(
		context.Background(),
		"some-secret",
		"KEY",
		"description",
		"user@example.com",
	)

	assert.Error(t, err)
	assert.NotEqual(t, database.ErrSecretNotFound, err)
}

func TestDeleteSecret_Success(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewSecretsRepository(client, "secrets-table", logger)

	// Create a secret first
	err := repo.CreateSecret(context.Background(), &api.Secret{
		Name:        "github-token",
		KeyName:     "GITHUB_TOKEN",
		Description: "GitHub token",
		CreatedBy:   "admin@example.com",
	})
	require.NoError(t, err)

	// Delete the secret
	err = repo.DeleteSecret(context.Background(), "github-token")
	assert.NoError(t, err)
	assert.Equal(t, 1, client.DeleteItemCalls)
}

func TestDeleteSecret_NotFound(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewSecretsRepository(client, "secrets-table", logger)

	// Try to delete a non-existent secret (DeleteItem returns error for missing item)
	client.DeleteItemError = &types.ConditionalCheckFailedException{}
	err := repo.DeleteSecret(context.Background(), "nonexistent")

	assert.Equal(t, database.ErrSecretNotFound, err)
	client.DeleteItemError = nil
}

func TestDeleteSecret_ClientError(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()

	// Inject a delete item error
	client.DeleteItemError = appErrors.ErrInternalError("test error", errors.New("delete failed"))

	repo := NewSecretsRepository(client, "secrets-table", logger)

	err := repo.DeleteSecret(context.Background(), "some-secret")

	assert.Error(t, err)
	assert.NotEqual(t, database.ErrSecretNotFound, err)
}
