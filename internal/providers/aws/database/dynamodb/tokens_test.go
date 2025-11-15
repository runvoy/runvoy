package dynamodb

import (
	"context"
	"errors"
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
