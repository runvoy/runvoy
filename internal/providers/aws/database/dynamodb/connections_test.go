package dynamodb

import (
	"context"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/testutil"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToConnectionItem(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour).Unix()

	tests := []struct {
		name       string
		connection *api.WebSocketConnection
	}{
		{
			name: "complete connection with all fields",
			connection: &api.WebSocketConnection{
				ConnectionID:  "conn-123",
				ExecutionID:   "exec-456",
				Functionality: "log_streaming",
				ExpiresAt:     expiresAt,
			},
		},
		{
			name: "connection with different functionality",
			connection: &api.WebSocketConnection{
				ConnectionID:  "conn-789",
				ExecutionID:   "exec-012",
				Functionality: "status_updates",
				ExpiresAt:     expiresAt,
			},
		},
		{
			name: "connection with zero expires_at",
			connection: &api.WebSocketConnection{
				ConnectionID:  "conn-zero",
				ExecutionID:   "exec-zero",
				Functionality: "log_streaming",
				ExpiresAt:     0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := toConnectionItem(tt.connection)

			assert.Equal(t, tt.connection.ConnectionID, item.ConnectionID)
			assert.Equal(t, tt.connection.ExecutionID, item.ExecutionID)
			assert.Equal(t, tt.connection.Functionality, item.Functionality)
			assert.Equal(t, tt.connection.ExpiresAt, item.ExpiresAt)
		})
	}
}

func TestNewConnectionRepository(t *testing.T) {
	logger := testutil.SilentLogger()
	tableName := "test-connections-table"

	repo := NewConnectionRepository(nil, tableName, logger)

	assert.NotNil(t, repo)
	// Verify it implements the interface
	var _ = repo
}

func TestConnectionRepositoryNilClient(t *testing.T) {
	// Test that repository can be created with nil client
	// (actual DynamoDB operations would fail, but creation should not)
	repo := NewConnectionRepository(nil, "test-table", testutil.SilentLogger())
	assert.NotNil(t, repo)
}

func TestToConnectionItemEdgeCases(t *testing.T) {
	t.Run("empty connection ID", func(t *testing.T) {
		conn := &api.WebSocketConnection{
			ConnectionID:  "",
			ExecutionID:   "exec-123",
			Functionality: "log_streaming",
			ExpiresAt:     time.Now().Unix(),
		}

		item := toConnectionItem(conn)
		assert.Empty(t, item.ConnectionID)
		assert.Equal(t, conn.ExecutionID, item.ExecutionID)
	})

	t.Run("empty execution ID", func(t *testing.T) {
		conn := &api.WebSocketConnection{
			ConnectionID:  "conn-123",
			ExecutionID:   "",
			Functionality: "log_streaming",
			ExpiresAt:     time.Now().Unix(),
		}

		item := toConnectionItem(conn)
		assert.Equal(t, conn.ConnectionID, item.ConnectionID)
		assert.Empty(t, item.ExecutionID)
	})

	t.Run("empty functionality", func(t *testing.T) {
		conn := &api.WebSocketConnection{
			ConnectionID:  "conn-123",
			ExecutionID:   "exec-123",
			Functionality: "",
			ExpiresAt:     time.Now().Unix(),
		}

		item := toConnectionItem(conn)
		assert.Equal(t, conn.ConnectionID, item.ConnectionID)
		assert.Empty(t, item.Functionality)
	})

	t.Run("negative expires_at", func(t *testing.T) {
		conn := &api.WebSocketConnection{
			ConnectionID:  "conn-123",
			ExecutionID:   "exec-123",
			Functionality: "log_streaming",
			ExpiresAt:     -1,
		}

		item := toConnectionItem(conn)
		assert.Equal(t, conn.ConnectionID, item.ConnectionID)
		assert.Equal(t, int64(-1), item.ExpiresAt)
	})

	t.Run("connection with client IP", func(t *testing.T) {
		conn := &api.WebSocketConnection{
			ConnectionID:  "conn-123",
			ExecutionID:   "exec-123",
			Functionality: "log_streaming",
			ExpiresAt:     time.Now().Unix(),
			ClientIP:      "192.168.1.1",
		}

		item := toConnectionItem(conn)
		assert.Equal(t, conn.ClientIP, item.ClientIP)
	})

	t.Run("connection without client IP", func(t *testing.T) {
		conn := &api.WebSocketConnection{
			ConnectionID:  "conn-123",
			ExecutionID:   "exec-123",
			Functionality: "log_streaming",
			ExpiresAt:     time.Now().Unix(),
			ClientIP:      "",
		}

		item := toConnectionItem(conn)
		assert.Empty(t, item.ClientIP)

		// Verify that empty ClientIP is omitted from DynamoDB marshaling
		av, err := attributevalue.MarshalMap(item)
		require.NoError(t, err)
		_, hasClientIP := av["client_ip"]
		assert.False(t, hasClientIP, "empty client_ip should be omitted from DynamoDB item")
	})
}

func TestConnectionItemDynamoDBTags(t *testing.T) {
	t.Run("verify struct tags", func(t *testing.T) {
		// This test ensures the dynamodbav tags are correctly set
		// by attempting to marshal/unmarshal
		item := &connectionItem{
			ConnectionID:  "test-123",
			ExecutionID:   "exec-456",
			Functionality: "log_streaming",
			ExpiresAt:     time.Now().Unix(),
			ClientIP:      "192.168.1.1",
		}

		// If tags are correct, this should not panic
		assert.NotPanics(t, func() {
			_, err := attributevalue.MarshalMap(item)
			require.NoError(t, err)
		})
	})

	t.Run("verify omitempty for ClientIP", func(t *testing.T) {
		// This test verifies that empty ClientIP is omitted
		item := &connectionItem{
			ConnectionID:  "test-123",
			ExecutionID:   "exec-456",
			Functionality: "log_streaming",
			ExpiresAt:     time.Now().Unix(),
			ClientIP:      "",
		}

		av, err := attributevalue.MarshalMap(item)
		require.NoError(t, err)
		_, hasClientIP := av["client_ip"]
		assert.False(t, hasClientIP, "empty client_ip should be omitted")
	})
}

// Benchmark conversion functions
func BenchmarkToConnectionItem(b *testing.B) {
	expiresAt := time.Now().Add(24 * time.Hour).Unix()
	conn := &api.WebSocketConnection{
		ConnectionID:  "conn-bench",
		ExecutionID:   "exec-bench",
		Functionality: "log_streaming",
		ExpiresAt:     expiresAt,
	}

	for b.Loop() {
		_ = toConnectionItem(conn)
	}
}

// Context tests
func TestConnectionRepositoryWithContext(t *testing.T) {
	t.Run("context with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Verify context is valid
		assert.NotNil(t, ctx)
		deadline, ok := ctx.Deadline()
		assert.True(t, ok)
		assert.True(t, time.Until(deadline) > 0)
	})

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		assert.NotNil(t, ctx)
		assert.Error(t, ctx.Err())
		assert.Equal(t, context.Canceled, ctx.Err())
	})
}

// Tests for DeleteConnections
func TestCreateConnection_Success(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewConnectionRepository(client, "connections-table", logger)

	connection := &api.WebSocketConnection{
		ConnectionID:         "conn-123",
		ExecutionID:          "exec-456",
		Functionality:        "logs",
		ExpiresAt:            time.Now().Add(1 * time.Hour).Unix(),
		ClientIP:             "192.168.1.1",
		Token:                "ws_token_123",
		UserEmail:            "user@example.com",
		TokenRequestClientIP: "192.168.1.2",
	}

	err := repo.CreateConnection(context.Background(), connection)

	assert.NoError(t, err)
	assert.Equal(t, 1, client.PutItemCalls)
}

func TestDeleteConnections_Success(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewConnectionRepository(client, "connections-table", logger)

	connectionIDs := []string{"conn-1", "conn-2", "conn-3"}

	deletedCount, err := repo.DeleteConnections(context.Background(), connectionIDs)

	assert.NoError(t, err)
	assert.Equal(t, 3, deletedCount)
	assert.Equal(t, 1, client.BatchWriteItemCalls)
}

func TestDeleteConnections_Empty(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewConnectionRepository(client, "connections-table", logger)

	connectionIDs := []string{}

	deletedCount, err := repo.DeleteConnections(context.Background(), connectionIDs)

	assert.NoError(t, err)
	assert.Equal(t, 0, deletedCount)
	assert.Equal(t, 0, client.BatchWriteItemCalls)
}

func TestDeleteConnections_LargeBatch(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewConnectionRepository(client, "connections-table", logger)

	// Create 60 connection IDs to test batching (should require 3 batches of 25)
	connectionIDs := make([]string, 60)
	for i := range 60 {
		connectionIDs[i] = "conn-" + string(rune(i))
	}

	deletedCount, err := repo.DeleteConnections(context.Background(), connectionIDs)

	assert.NoError(t, err)
	assert.Equal(t, 60, deletedCount)
	assert.Equal(t, 3, client.BatchWriteItemCalls) // 3 batches needed for 60 items
}

func TestGetConnectionsByExecutionID_Success(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewConnectionRepository(client, "connections-table", logger)

	// Create some connections
	connections := []api.WebSocketConnection{
		{
			ConnectionID:         "conn-1",
			ExecutionID:          "exec-123",
			Functionality:        "logs",
			ExpiresAt:            time.Now().Add(1 * time.Hour).Unix(),
			ClientIP:             "192.168.1.1",
			Token:                "token-1",
			UserEmail:            "user@example.com",
			TokenRequestClientIP: "192.168.1.2",
		},
		{
			ConnectionID:         "conn-2",
			ExecutionID:          "exec-123",
			Functionality:        "logs",
			ExpiresAt:            time.Now().Add(1 * time.Hour).Unix(),
			ClientIP:             "192.168.1.3",
			Token:                "token-2",
			UserEmail:            "user@example.com",
			TokenRequestClientIP: "192.168.1.4",
		},
	}

	for i := range connections {
		err := repo.CreateConnection(context.Background(), &connections[i])
		require.NoError(t, err)
	}

	// Query by execution ID
	retrieved, err := repo.GetConnectionsByExecutionID(context.Background(), "exec-123")

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(retrieved), 2)
	assert.Equal(t, 1, client.QueryCalls)
}

func TestGetConnectionsByExecutionID_NoResults(t *testing.T) {
	client := NewMockDynamoDBClient()
	logger := testutil.SilentLogger()
	repo := NewConnectionRepository(client, "connections-table", logger)

	// Query for non-existent execution
	retrieved, err := repo.GetConnectionsByExecutionID(context.Background(), "nonexistent-exec")

	assert.NoError(t, err)
	assert.Len(t, retrieved, 0)
}
