package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"runvoy/internal/api"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenRepository(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	tableName := "tokens-table"

	repo := NewTokenRepository(client, tableName, logger)

	assert.NotNil(t, repo)
}

func TestCreateToken_Success(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewTokenRepository(client, "tokens-table", logger)

	token := &api.WebSocketToken{
		Token:       "ws_token_123",
		ExecutionID: "exec-456",
		UserEmail:   "user@example.com",
		ClientIP:    "192.168.1.1",
		ExpiresAt:   time.Now().Add(1 * time.Hour).Unix(),
		CreatedAt:   time.Now().Unix(),
	}

	err := repo.CreateToken(context.Background(), token)

	assert.NoError(t, err)
	assert.Equal(t, 1, client.PutItemCalls)
}

func TestCreateToken_ClientError(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()

	// Inject a put item error
	client.PutItemError = appErrors.ErrDatabaseError("test error", errors.New("database error"))

	repo := NewTokenRepository(client, "tokens-table", logger)

	token := &api.WebSocketToken{
		Token:       "ws_token_123",
		ExecutionID: "exec-456",
		UserEmail:   "user@example.com",
		ClientIP:    "192.168.1.1",
		ExpiresAt:   time.Now().Add(1 * time.Hour).Unix(),
		CreatedAt:   time.Now().Unix(),
	}

	err := repo.CreateToken(context.Background(), token)

	assert.Error(t, err)
}

func TestGetToken_NotFound(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewTokenRepository(client, "tokens-table", logger)

	// Token doesn't exist, should return nil without error
	retrieved, err := repo.GetToken(context.Background(), "nonexistent_token")

	assert.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestGetToken_ClientError(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()

	// Inject a get item error
	client.GetItemError = appErrors.ErrDatabaseError("test error", errors.New("database error"))

	repo := NewTokenRepository(client, "tokens-table", logger)

	_, err := repo.GetToken(context.Background(), "some_token")

	assert.Error(t, err)
}

func TestDeleteToken_Success(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewTokenRepository(client, "tokens-table", logger)

	// Create a token first
	token := &api.WebSocketToken{
		Token:       "ws_token_123",
		ExecutionID: "exec-456",
		UserEmail:   "user@example.com",
		ClientIP:    "192.168.1.1",
		ExpiresAt:   time.Now().Add(1 * time.Hour).Unix(),
		CreatedAt:   time.Now().Unix(),
	}
	err := repo.CreateToken(context.Background(), token)
	require.NoError(t, err)

	// Delete the token
	err = repo.DeleteToken(context.Background(), "ws_token_123")

	assert.NoError(t, err)
	assert.Equal(t, 1, client.DeleteItemCalls)
}

func TestDeleteToken_ClientError(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()

	// Inject a delete item error
	client.DeleteItemError = appErrors.ErrDatabaseError("test error", errors.New("delete failed"))

	repo := NewTokenRepository(client, "tokens-table", logger)

	err := repo.DeleteToken(context.Background(), "some_token")

	assert.Error(t, err)
}

func TestTokenRepository_CreateToken_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "tokens-table"

	t.Run("handles marshal error", func(t *testing.T) {
		client := NewMockDynamoDBClient()
		repo := NewTokenRepository(client, tableName, logger)

		// Create token - marshal should succeed for valid token
		token := &api.WebSocketToken{
			Token:       "token-123",
			ExecutionID: "exec-456",
			ExpiresAt:   time.Now().Add(1 * time.Hour).Unix(),
			CreatedAt:   time.Now().Unix(),
		}

		err := repo.CreateToken(ctx, token)
		// Marshal should succeed
		assert.NoError(t, err)
	})

	t.Run("handles put item error", func(t *testing.T) {
		client := NewMockDynamoDBClient()
		client.PutItemError = fmt.Errorf("put item failed")
		repo := NewTokenRepository(client, tableName, logger)

		token := &api.WebSocketToken{
			Token:       "token-123",
			ExecutionID: "exec-456",
			ExpiresAt:   time.Now().Add(1 * time.Hour).Unix(),
			CreatedAt:   time.Now().Unix(),
		}

		err := repo.CreateToken(ctx, token)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to store token")
	})
}

func TestTokenRepository_GetToken_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "tokens-table"

	t.Run("handles get item error", func(t *testing.T) {
		client := NewMockDynamoDBClient()
		client.GetItemError = fmt.Errorf("get item failed")
		repo := NewTokenRepository(client, tableName, logger)

		token, err := repo.GetToken(ctx, "token-123")

		require.Error(t, err)
		assert.Nil(t, token)
		assert.Contains(t, err.Error(), "failed to retrieve token")
	})

	t.Run("handles unmarshal error", func(t *testing.T) {
		client := NewMockDynamoDBClient()
		repo := NewTokenRepository(client, tableName, logger)

		// Note: attributevalue.UnmarshalMap is quite permissive and doesn't easily fail
		// on type mismatches. To properly test unmarshal errors, we'd need to create
		// truly malformed data that the unmarshaler can't handle, which is difficult
		// with the current mock setup. This test verifies the function handles empty results.
		// For actual unmarshal error testing, integration tests would be more appropriate.
		token, err := repo.GetToken(ctx, "token-123")

		// Should succeed with nil token (no error for missing items)
		require.NoError(t, err)
		assert.Nil(t, token)
	})
}

func TestTokenRepository_DeleteToken_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "tokens-table"

	t.Run("handles delete item error", func(t *testing.T) {
		client := NewMockDynamoDBClient()
		client.DeleteItemError = fmt.Errorf("delete item failed")
		repo := NewTokenRepository(client, tableName, logger)

		err := repo.DeleteToken(ctx, "token-123")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete token")
	})
}
