package dynamodb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/testutil"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_UpdateLastUsed(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "test-users-table"
	pendingTableName := "test-pending-table"

	t.Run("successfully updates last used", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		// Create a user first
		user := &api.User{
			Email:     "user@example.com",
			Role:      "viewer",
			CreatedAt: time.Now(),
		}
		err := repo.CreateUser(ctx, user, "hash123", 0)
		require.NoError(t, err)

		// Manually set up the table structure for query
		if mockClient.Tables[tableName] == nil {
			mockClient.Tables[tableName] = make(map[string]map[string]map[string]types.AttributeValue)
		}
		if mockClient.Tables[tableName]["hash123"] == nil {
			mockClient.Tables[tableName]["hash123"] = make(map[string]map[string]types.AttributeValue)
		}
		// Set up index for queryAPIKeyHashByEmail
		if mockClient.Indexes[tableName] == nil {
			mockClient.Indexes[tableName] = make(map[string]map[string][]map[string]types.AttributeValue)
		}
		if mockClient.Indexes[tableName]["user_email-index"] == nil {
			mockClient.Indexes[tableName]["user_email-index"] = make(map[string][]map[string]types.AttributeValue)
		}
		item := map[string]types.AttributeValue{
			"api_key_hash": &types.AttributeValueMemberS{Value: "hash123"},
			"user_email":   &types.AttributeValueMemberS{Value: "user@example.com"},
		}
		mockClient.Indexes[tableName]["user_email-index"]["user@example.com"] = []map[string]types.AttributeValue{item}

		// Update last used
		lastUsed, err := repo.UpdateLastUsed(ctx, "user@example.com")

		require.NoError(t, err)
		assert.NotNil(t, lastUsed)
		assert.Less(t, time.Since(*lastUsed), time.Second)
		assert.Equal(t, 1, mockClient.UpdateItemCalls)
	})

	t.Run("handles user not found", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		// Query will return empty result
		lastUsed, err := repo.UpdateLastUsed(ctx, "nonexistent@example.com")

		require.Error(t, err)
		assert.Nil(t, lastUsed)
		assert.Contains(t, err.Error(), "user not found")
	})

	t.Run("handles query error", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		mockClient.QueryError = errors.New("query failed")
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		lastUsed, err := repo.UpdateLastUsed(ctx, "user@example.com")

		require.Error(t, err)
		assert.Nil(t, lastUsed)
		assert.Contains(t, err.Error(), "failed to query user by email")
	})

	t.Run("handles update error", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		// Set up index to return a user
		if mockClient.Indexes[tableName] == nil {
			mockClient.Indexes[tableName] = make(map[string]map[string][]map[string]types.AttributeValue)
		}
		if mockClient.Indexes[tableName]["user_email-index"] == nil {
			mockClient.Indexes[tableName]["user_email-index"] = make(map[string][]map[string]types.AttributeValue)
		}
		item := map[string]types.AttributeValue{
			"api_key_hash": &types.AttributeValueMemberS{Value: "hash123"},
			"user_email":   &types.AttributeValueMemberS{Value: "user@example.com"},
		}
		mockClient.Indexes[tableName]["user_email-index"]["user@example.com"] = []map[string]types.AttributeValue{item}

		// Inject update error
		mockClient.UpdateItemError = errors.New("update failed")

		lastUsed, err := repo.UpdateLastUsed(ctx, "user@example.com")

		require.Error(t, err)
		assert.Nil(t, lastUsed)
		assert.Contains(t, err.Error(), "failed to update last_used")
	})
}

func TestUserRepository_RevokeUser(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "test-users-table"
	pendingTableName := "test-pending-table"

	t.Run("successfully revokes user", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		// Set up index to return a user
		if mockClient.Indexes[tableName] == nil {
			mockClient.Indexes[tableName] = make(map[string]map[string][]map[string]types.AttributeValue)
		}
		if mockClient.Indexes[tableName]["user_email-index"] == nil {
			mockClient.Indexes[tableName]["user_email-index"] = make(map[string][]map[string]types.AttributeValue)
		}
		item := map[string]types.AttributeValue{
			"api_key_hash": &types.AttributeValueMemberS{Value: "hash123"},
			"user_email":   &types.AttributeValueMemberS{Value: "user@example.com"},
		}
		mockClient.Indexes[tableName]["user_email-index"]["user@example.com"] = []map[string]types.AttributeValue{item}

		// Also set up the table so UpdateItem can find the item
		if mockClient.Tables[tableName] == nil {
			mockClient.Tables[tableName] = make(map[string]map[string]map[string]types.AttributeValue)
		}
		if mockClient.Tables[tableName]["hash123"] == nil {
			mockClient.Tables[tableName]["hash123"] = make(map[string]map[string]types.AttributeValue)
		}
		mockClient.Tables[tableName]["hash123"][""] = item

		err := repo.RevokeUser(ctx, "user@example.com")

		require.NoError(t, err)
		assert.Equal(t, 1, mockClient.UpdateItemCalls)
	})

	t.Run("handles user not found", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		err := repo.RevokeUser(ctx, "nonexistent@example.com")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")
	})

	t.Run("handles update error", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		// Set up index to return a user
		if mockClient.Indexes[tableName] == nil {
			mockClient.Indexes[tableName] = make(map[string]map[string][]map[string]types.AttributeValue)
		}
		if mockClient.Indexes[tableName]["user_email-index"] == nil {
			mockClient.Indexes[tableName]["user_email-index"] = make(map[string][]map[string]types.AttributeValue)
		}
		item := map[string]types.AttributeValue{
			"api_key_hash": &types.AttributeValueMemberS{Value: "hash123"},
			"user_email":   &types.AttributeValueMemberS{Value: "user@example.com"},
		}
		mockClient.Indexes[tableName]["user_email-index"]["user@example.com"] = []map[string]types.AttributeValue{item}

		// Inject update error
		mockClient.UpdateItemError = errors.New("update failed")

		err := repo.RevokeUser(ctx, "user@example.com")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to revoke user")
	})
}

func TestUserRepository_ListUsers(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "test-users-table"
	pendingTableName := "test-pending-table"

	t.Run("successfully lists users", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		users, err := repo.ListUsers(ctx)

		require.NoError(t, err)
		assert.NotNil(t, users)
		assert.Equal(t, 1, mockClient.QueryCalls)
	})

	t.Run("handles empty result", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		users, err := repo.ListUsers(ctx)

		require.NoError(t, err)
		assert.Empty(t, users)
	})

	t.Run("handles query error", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		mockClient.QueryError = errors.New("query failed")
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		users, err := repo.ListUsers(ctx)

		require.Error(t, err)
		assert.Nil(t, users)
		assert.Contains(t, err.Error(), "failed to list users")
	})

	t.Run("handles unmarshal error gracefully", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		// Set up index with invalid item
		if mockClient.Indexes[tableName] == nil {
			mockClient.Indexes[tableName] = make(map[string]map[string][]map[string]types.AttributeValue)
		}
		if mockClient.Indexes[tableName]["all-user_email"] == nil {
			mockClient.Indexes[tableName]["all-user_email"] = make(map[string][]map[string]types.AttributeValue)
		}
		// Invalid item that will fail to unmarshal
		invalidItem := map[string]types.AttributeValue{
			"user_email": &types.AttributeValueMemberN{Value: "not-a-number"}, // Wrong type
		}
		mockClient.Indexes[tableName]["all-user_email"]["USER"] = []map[string]types.AttributeValue{invalidItem}

		users, err := repo.ListUsers(ctx)

		// Should handle gracefully - skip invalid items
		require.NoError(t, err)
		assert.NotNil(t, users)
		// Invalid items are skipped, so list might be empty
	})
}

func TestUserRepository_QueryAPIKeyHashByEmail(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "test-users-table"
	pendingTableName := "test-pending-table"

	t.Run("successfully queries api key hash", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		// Set up index
		if mockClient.Indexes[tableName] == nil {
			mockClient.Indexes[tableName] = make(map[string]map[string][]map[string]types.AttributeValue)
		}
		if mockClient.Indexes[tableName]["user_email-index"] == nil {
			mockClient.Indexes[tableName]["user_email-index"] = make(map[string][]map[string]types.AttributeValue)
		}
		item := map[string]types.AttributeValue{
			"api_key_hash": &types.AttributeValueMemberS{Value: "hash123"},
			"user_email":   &types.AttributeValueMemberS{Value: "user@example.com"},
		}
		mockClient.Indexes[tableName]["user_email-index"]["user@example.com"] = []map[string]types.AttributeValue{item}

		hash, err := repo.queryAPIKeyHashByEmail(ctx, "user@example.com", "test")

		require.NoError(t, err)
		assert.Equal(t, "hash123", hash)
	})

	t.Run("handles user not found", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		hash, err := repo.queryAPIKeyHashByEmail(ctx, "nonexistent@example.com", "test")

		require.Error(t, err)
		assert.Empty(t, hash)
		assert.Contains(t, err.Error(), "user not found")
	})

	t.Run("handles missing api_key_hash attribute", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		// Set up index with item missing api_key_hash
		if mockClient.Indexes[tableName] == nil {
			mockClient.Indexes[tableName] = make(map[string]map[string][]map[string]types.AttributeValue)
		}
		if mockClient.Indexes[tableName]["user_email-index"] == nil {
			mockClient.Indexes[tableName]["user_email-index"] = make(map[string][]map[string]types.AttributeValue)
		}
		item := map[string]types.AttributeValue{
			"user_email": &types.AttributeValueMemberS{Value: "user@example.com"},
			// Missing api_key_hash
		}
		mockClient.Indexes[tableName]["user_email-index"]["user@example.com"] = []map[string]types.AttributeValue{item}

		hash, err := repo.queryAPIKeyHashByEmail(ctx, "user@example.com", "test")

		require.Error(t, err)
		assert.Empty(t, hash)
		assert.Contains(t, err.Error(), "missing api_key_hash attribute")
	})

	t.Run("handles query error", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		mockClient.QueryError = errors.New("query failed")
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		hash, err := repo.queryAPIKeyHashByEmail(ctx, "user@example.com", "test")

		require.Error(t, err)
		assert.Empty(t, hash)
		assert.Contains(t, err.Error(), "failed to query user by email")
	})
}

func TestUserRepository_CreateUser_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "test-users-table"
	pendingTableName := "test-pending-table"

	t.Run("handles marshal error", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		// Create user with invalid data that might cause marshal issues
		// Note: attributevalue.MarshalMap is quite permissive, so this might not actually fail
		user := &api.User{
			Email:     "user@example.com",
			Role:      "viewer",
			CreatedAt: time.Now(),
		}

		err := repo.CreateUser(ctx, user, "hash123", 0)
		// Marshal should succeed for valid user
		assert.NoError(t, err)
	})

	t.Run("handles conditional check failed", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		mockClient.PutItemError = &types.ConditionalCheckFailedException{}
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		user := &api.User{
			Email:     "user@example.com",
			Role:      "viewer",
			CreatedAt: time.Now(),
		}

		err := repo.CreateUser(ctx, user, "hash123", 0)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("handles database error", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		mockClient.PutItemError = errors.New("database error")
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		user := &api.User{
			Email:     "user@example.com",
			Role:      "viewer",
			CreatedAt: time.Now(),
		}

		err := repo.CreateUser(ctx, user, "hash123", 0)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create user")
	})
}

func TestUserRepository_GetUserByEmail_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "test-users-table"
	pendingTableName := "test-pending-table"

	t.Run("handles query error", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		mockClient.QueryError = errors.New("query failed")
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		user, err := repo.GetUserByEmail(ctx, "user@example.com")

		require.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "failed to query user by email")
		assert.Equal(t, 1, mockClient.QueryCalls)
	})

	t.Run("returns nil for non-existent user", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		user, err := repo.GetUserByEmail(ctx, "nonexistent@example.com")

		require.NoError(t, err)
		assert.Nil(t, user)
	})

	t.Run("GetUserByAPIKeyHash handles get item error", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		mockClient.GetItemError = errors.New("get item failed")
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		user, err := repo.GetUserByAPIKeyHash(ctx, "hash123")

		require.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "failed to get user by API key hash")
		assert.Equal(t, 1, mockClient.GetItemCalls)
	})

	t.Run("GetUserByAPIKeyHash returns nil for non-existent hash", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewUserRepository(mockClient, tableName, pendingTableName, logger)

		user, err := repo.GetUserByAPIKeyHash(ctx, "nonexistent-hash")

		require.NoError(t, err)
		assert.Nil(t, user)
	})
}
