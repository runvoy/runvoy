package dynamodb_test

import (
	"context"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/providers/aws/database/dynamodb"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserRepository_CreateUser_WithMock demonstrates how to use the mock client
// to test repository operations without requiring a real DynamoDB instance.
func TestUserRepository_CreateUser_WithMock(t *testing.T) {
	// Setup: Create a mock client and repository
	mockClient := dynamodb.NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := dynamodb.NewUserRepository(mockClient, "test-users-table", "test-pending-table", logger)

	// Test: Create a user
	ctx := context.Background()
	user := &api.User{
		Email:     "test@example.com",
		Role:      "viewer",
		CreatedAt: time.Now(),
		Revoked:   false,
	}
	apiKeyHash := "hashed_key_123"

	err := repo.CreateUser(ctx, user, apiKeyHash, 0)

	// Verify: Operation succeeded and client was called
	require.NoError(t, err)
	assert.Equal(t, 1, mockClient.PutItemCalls, "PutItem should be called once")
}

// TestUserRepository_CreateUser_ErrorHandling demonstrates testing error scenarios.
func TestUserRepository_CreateUser_ErrorHandling(t *testing.T) {
	// Setup: Create a mock client configured to return an error
	mockClient := dynamodb.NewMockDynamoDBClient()
	mockClient.PutItemError = assert.AnError
	logger := testutil.SilentLogger()
	repo := dynamodb.NewUserRepository(mockClient, "test-users-table", "test-pending-table", logger)

	// Test: Attempt to create a user
	ctx := context.Background()
	user := &api.User{
		Email:     "test@example.com",
		Role:      "viewer",
		CreatedAt: time.Now(),
		Revoked:   false,
	}
	apiKeyHash := "hashed_key_123"

	err := repo.CreateUser(ctx, user, apiKeyHash, 0)

	// Verify: Error was propagated
	require.Error(t, err)
	assert.Equal(t, 1, mockClient.PutItemCalls, "PutItem should still be called")
}

// TestUserRepository_GetUserByEmail_WithMock demonstrates testing read operations.
func TestUserRepository_GetUserByEmail_WithMock(t *testing.T) {
	// Setup
	mockClient := dynamodb.NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := dynamodb.NewUserRepository(mockClient, "test-users-table", "test-pending-table", logger)

	// Test: Get a user (will return empty result from mock)
	ctx := context.Background()
	user, err := repo.GetUserByEmail(ctx, "test@example.com")

	// Verify: Operation completed
	// The mock returns empty items by default, which results in nil user
	// Note: A real DynamoDB test with no items would behave similarly
	require.NoError(t, err)
	assert.Nil(t, user, "Should return nil for non-existent user")
	assert.Equal(t, 1, mockClient.QueryCalls, "Query should be called once")
}

// TestExecutionRepository_CreateExecution_WithMock demonstrates testing ExecutionRepository.
func TestExecutionRepository_CreateExecution_WithMock(t *testing.T) {
	// Setup
	mockClient := dynamodb.NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := dynamodb.NewExecutionRepository(mockClient, "test-executions-table", logger)

	// Test: Create an execution
	ctx := context.Background()
	execution := &api.Execution{
		ExecutionID: "exec-123",
		StartedAt:   time.Now(),
		CreatedBy:   "test@example.com",
		OwnedBy:     []string{"user@example.com"},
		Command:     "echo hello",
		Status:      "running",
	}

	err := repo.CreateExecution(ctx, execution)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, 1, mockClient.PutItemCalls, "PutItem should be called once")
}

// TestConnectionRepository_CreateConnection_WithMock demonstrates testing ConnectionRepository.
func TestConnectionRepository_CreateConnection_WithMock(t *testing.T) {
	// Setup
	mockClient := dynamodb.NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := dynamodb.NewConnectionRepository(mockClient, "test-connections-table", logger)

	// Test: Create a connection
	ctx := context.Background()
	connection := &api.WebSocketConnection{
		ConnectionID:  "conn-123",
		ExecutionID:   "exec-123",
		Functionality: "stdio",
		ExpiresAt:     time.Now().Add(1 * time.Hour).Unix(),
	}

	err := repo.CreateConnection(ctx, connection)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, 1, mockClient.PutItemCalls, "PutItem should be called once")
}

// TestTokenRepository_CreateToken_WithMock demonstrates testing TokenRepository.
func TestTokenRepository_CreateToken_WithMock(t *testing.T) {
	// Setup
	mockClient := dynamodb.NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := dynamodb.NewTokenRepository(mockClient, "test-tokens-table", logger)

	// Test: Create a token
	ctx := context.Background()
	token := &api.WebSocketToken{
		Token:       "token-123",
		ExecutionID: "exec-123",
		ExpiresAt:   time.Now().Add(1 * time.Hour).Unix(),
		CreatedAt:   time.Now().Unix(),
	}

	err := repo.CreateToken(ctx, token)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, 1, mockClient.PutItemCalls, "PutItem should be called once")
}

// TestMockClient_CallCounting demonstrates using call counters for assertions.
func TestMockClient_CallCounting(t *testing.T) {
	// Setup
	mockClient := dynamodb.NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := dynamodb.NewUserRepository(mockClient, "test-users-table", "test-pending-table", logger)

	ctx := context.Background()

	// Perform multiple operations
	user := &api.User{
		Email:     "test@example.com",
		Role:      "viewer",
		CreatedAt: time.Now(),
	}
	apiKeyHash := "hashed_key_123"

	_ = repo.CreateUser(ctx, user, apiKeyHash, 0)
	_, _ = repo.GetUserByEmail(ctx, "test@example.com")
	_, _ = repo.GetUserByEmail(ctx, "test2@example.com")

	// Verify call counts
	assert.Equal(t, 1, mockClient.PutItemCalls, "Should have 1 PutItem call")
	assert.Equal(t, 2, mockClient.QueryCalls, "Should have 2 Query calls")

	// Reset counters
	mockClient.ResetCallCounts()
	assert.Equal(t, 0, mockClient.PutItemCalls, "Counters should be reset")
	assert.Equal(t, 0, mockClient.QueryCalls, "Counters should be reset")
}

// TestMockClient_ClearTables demonstrates clearing mock data between tests.
func TestMockClient_ClearTables(t *testing.T) {
	mockClient := dynamodb.NewMockDynamoDBClient()

	// Verify tables are empty initially
	assert.Empty(t, mockClient.Tables)

	// Add some data (via PutItem)
	logger := testutil.SilentLogger()
	repo := dynamodb.NewUserRepository(mockClient, "test-table", "test-pending", logger)
	ctx := context.Background()

	user := &api.User{
		Email:     "test@example.com",
		Role:      "viewer",
		CreatedAt: time.Now(),
	}
	apiKeyHash := "hashed_key_123"
	_ = repo.CreateUser(ctx, user, apiKeyHash, 0)

	// Verify data was added
	assert.NotEmpty(t, mockClient.Tables)

	// Clear tables
	mockClient.ClearTables()

	// Verify tables are empty again
	assert.Empty(t, mockClient.Tables)
}
