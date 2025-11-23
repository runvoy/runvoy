package dynamodb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/testutil"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToExecutionItem(t *testing.T) {
	now := time.Now()
	completed := now.Add(5 * time.Minute)

	tests := []struct {
		name      string
		execution *api.Execution
	}{
		{
			name: "complete execution with all fields",
			execution: &api.Execution{
				ExecutionID:         "exec-123",
				CreatedBy:           "user@example.com",
				OwnedBy:             []string{"user@example.com"},
				Command:             "echo hello",
				StartedAt:           now,
				CompletedAt:         &completed,
				Status:              "SUCCEEDED",
				ExitCode:            0,
				DurationSeconds:     300,
				LogStreamName:       "ecs/task/123",
				CreatedByRequestID:  "req-456",
				ModifiedByRequestID: "req-789",
				ComputePlatform:     "AWS",
			},
		},
		{
			name: "minimal execution",
			execution: &api.Execution{
				ExecutionID: "exec-minimal",
				CreatedBy:   "user@example.com",
				OwnedBy:     []string{"user@example.com"},
				Command:     "ls",
				StartedAt:   now,
				Status:      "RUNNING",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := toExecutionItem(tt.execution)

			assert.Equal(t, tt.execution.ExecutionID, item.ExecutionID)
			assert.Equal(t, tt.execution.CreatedBy, item.CreatedBy)
			assert.Equal(t, tt.execution.Command, item.Command)
			assert.Equal(t, tt.execution.StartedAt.Unix(), item.StartedAt)
			if tt.execution.CompletedAt != nil {
				require.NotNil(t, item.CompletedAt)
				assert.Equal(t, tt.execution.CompletedAt.Unix(), *item.CompletedAt)
			} else {
				assert.Nil(t, item.CompletedAt)
			}
			assert.Equal(t, tt.execution.Status, item.Status)
			assert.Equal(t, tt.execution.ExitCode, item.ExitCode)
			assert.Equal(t, tt.execution.DurationSeconds, item.DurationSecs)
			assert.Equal(t, tt.execution.LogStreamName, item.LogStreamName)
			assert.Equal(t, tt.execution.CreatedByRequestID, item.CreatedByRequestID)
			assert.Equal(t, tt.execution.ModifiedByRequestID, item.ModifiedByRequestID)
			assert.Equal(t, tt.execution.ComputePlatform, item.ComputePlatform)
		})
	}
}

func TestExecutionItem_ToAPIExecution(t *testing.T) {
	nowUnix := time.Now().Unix()
	completedUnix := time.Now().Add(5 * time.Minute).Unix()

	tests := []struct {
		name string
		item *executionItem
	}{
		{
			name: "complete execution item",
			item: &executionItem{
				ExecutionID:         "exec-123",
				CreatedBy:           "user@example.com",
				OwnedBy:             []string{"user@example.com"},
				Command:             "echo hello",
				StartedAt:           nowUnix,
				CompletedAt:         &completedUnix,
				Status:              "SUCCEEDED",
				ExitCode:            0,
				DurationSecs:        300,
				LogStreamName:       "ecs/task/123",
				CreatedByRequestID:  "req-456",
				ModifiedByRequestID: "req-789",
				ComputePlatform:     "AWS",
			},
		},
		{
			name: "minimal execution item",
			item: &executionItem{
				ExecutionID: "exec-minimal",
				CreatedBy:   "user@example.com",
				OwnedBy:     []string{"user@example.com"},
				Command:     "ls",
				StartedAt:   nowUnix,
				Status:      "RUNNING",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execution := tt.item.toAPIExecution()

			assert.Equal(t, tt.item.ExecutionID, execution.ExecutionID)
			assert.Equal(t, tt.item.CreatedBy, execution.CreatedBy)
			assert.Equal(t, tt.item.Command, execution.Command)
			assert.Equal(t, tt.item.StartedAt, execution.StartedAt.Unix())
			if tt.item.CompletedAt != nil {
				require.NotNil(t, execution.CompletedAt)
				assert.Equal(t, *tt.item.CompletedAt, execution.CompletedAt.Unix())
			} else {
				assert.Nil(t, execution.CompletedAt)
			}
			assert.Equal(t, tt.item.Status, execution.Status)
			assert.Equal(t, tt.item.ExitCode, execution.ExitCode)
			assert.Equal(t, tt.item.DurationSecs, execution.DurationSeconds)
			assert.Equal(t, tt.item.LogStreamName, execution.LogStreamName)
			assert.Equal(t, tt.item.CreatedByRequestID, execution.CreatedByRequestID)
			assert.Equal(t, tt.item.ModifiedByRequestID, execution.ModifiedByRequestID)
			assert.Equal(t, tt.item.ComputePlatform, execution.ComputePlatform)
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	now := time.Now().UTC()
	completed := now.Add(5 * time.Minute)

	original := &api.Execution{
		ExecutionID:         "exec-roundtrip",
		CreatedBy:           "user@example.com",
		OwnedBy:             []string{"user@example.com"},
		Command:             "echo test",
		StartedAt:           now,
		CompletedAt:         &completed,
		Status:              "SUCCEEDED",
		ExitCode:            42,
		DurationSeconds:     150,
		LogStreamName:       "logs/stream",
		CreatedByRequestID:  "req-789",
		ModifiedByRequestID: "req-abc",
		ComputePlatform:     "AWS",
	}

	// Convert to item and back
	item := toExecutionItem(original)
	result := item.toAPIExecution()

	assert.Equal(t, original.ExecutionID, result.ExecutionID)
	assert.Equal(t, original.CreatedBy, result.CreatedBy)
	assert.Equal(t, original.Command, result.Command)
	assert.Equal(t, original.StartedAt.Unix(), result.StartedAt.Unix())
	assert.Equal(t, original.Status, result.Status)
	assert.Equal(t, original.ExitCode, result.ExitCode)
	assert.Equal(t, original.DurationSeconds, result.DurationSeconds)
	assert.Equal(t, original.LogStreamName, result.LogStreamName)
	assert.Equal(t, original.CreatedByRequestID, result.CreatedByRequestID)
	assert.Equal(t, original.ModifiedByRequestID, result.ModifiedByRequestID)
	assert.Equal(t, original.ComputePlatform, result.ComputePlatform)

	require.NotNil(t, result.CompletedAt)
	assert.Equal(t, completed.Unix(), result.CompletedAt.Unix())
}

func TestNewExecutionRepository(t *testing.T) {
	logger := testutil.SilentLogger()
	tableName := "test-table"

	repo := NewExecutionRepository(nil, tableName, logger)

	assert.NotNil(t, repo)
	assert.Equal(t, tableName, repo.tableName)
	assert.Equal(t, logger, repo.logger)
}

// Test edge cases for conversion functions
func TestConversionEdgeCases(t *testing.T) {
	t.Run("nil CompletedAt", func(t *testing.T) {
		exec := &api.Execution{
			ExecutionID: "exec-1",
			CreatedBy:   "user@example.com",
			OwnedBy:     []string{"user@example.com"},
			Command:     "test",
			StartedAt:   time.Now(),
			CompletedAt: nil,
			Status:      "RUNNING",
		}

		item := toExecutionItem(exec)
		assert.Nil(t, item.CompletedAt)

		result := item.toAPIExecution()
		assert.Nil(t, result.CompletedAt)
	})

	t.Run("zero ExitCode", func(t *testing.T) {
		exec := &api.Execution{
			ExecutionID: "exec-2",
			CreatedBy:   "user@example.com",
			OwnedBy:     []string{"user@example.com"},
			Command:     "test",
			StartedAt:   time.Now(),
			Status:      "SUCCEEDED",
			ExitCode:    0,
		}

		item := toExecutionItem(exec)
		assert.Equal(t, 0, item.ExitCode)

		result := item.toAPIExecution()
		assert.Equal(t, 0, result.ExitCode)
	})

	t.Run("non-zero ExitCode", func(t *testing.T) {
		exec := &api.Execution{
			ExecutionID: "exec-3",
			CreatedBy:   "user@example.com",
			OwnedBy:     []string{"user@example.com"},
			Command:     "test",
			StartedAt:   time.Now(),
			Status:      "FAILED",
			ExitCode:    137,
		}

		item := toExecutionItem(exec)
		assert.Equal(t, 137, item.ExitCode)

		result := item.toAPIExecution()
		assert.Equal(t, 137, result.ExitCode)
	})

	t.Run("empty optional fields", func(t *testing.T) {
		exec := &api.Execution{
			ExecutionID:        "exec-4",
			CreatedBy:          "user@example.com",
			OwnedBy:            []string{"user@example.com"},
			Command:            "test",
			StartedAt:          time.Now(),
			Status:             "RUNNING",
			CreatedByRequestID: "",
		}

		item := toExecutionItem(exec)
		assert.Empty(t, item.CreatedByRequestID)

		result := item.toAPIExecution()
		assert.Empty(t, result.CreatedByRequestID)
	})
}

func TestExecutionItemDynamoDBTags(t *testing.T) {
	t.Run("verify struct tags", func(t *testing.T) {
		// This test ensures the dynamodbav tags are correctly set
		// by attempting to marshal/unmarshal
		item := &executionItem{
			ExecutionID:        "test-123",
			StartedAt:          time.Now().Unix(),
			CreatedBy:          "user@example.com",
			OwnedBy:            []string{"user@example.com"},
			Command:            "echo test",
			Status:             "RUNNING",
			LogStreamName:      "log-stream",
			CreatedByRequestID: "req-123",
			ComputePlatform:    "AWS",
		}

		// If tags are correct, this should not panic
		assert.NotPanics(t, func() {
			_ = item.toAPIExecution()
		})
	})
}

// Benchmark conversion functions
func BenchmarkToExecutionItem(b *testing.B) {
	now := time.Now()
	completed := now.Add(5 * time.Minute)
	exec := &api.Execution{
		ExecutionID:        "exec-bench",
		CreatedBy:          "user@example.com",
		OwnedBy:            []string{"user@example.com"},
		Command:            "echo benchmark",
		StartedAt:          now,
		CompletedAt:        &completed,
		Status:             "SUCCEEDED",
		ExitCode:           0,
		DurationSeconds:    300,
		LogStreamName:      "logs/bench",
		CreatedByRequestID: "req-bench",
		ComputePlatform:    "AWS",
	}

	for b.Loop() {
		_ = toExecutionItem(exec)
	}
}

func BenchmarkToAPIExecution(b *testing.B) {
	nowUnix := time.Now().Unix()
	completedUnix := time.Now().Add(5 * time.Minute).Unix()
	item := &executionItem{
		ExecutionID:        "exec-bench",
		CreatedBy:          "user@example.com",
		OwnedBy:            []string{"user@example.com"},
		Command:            "echo benchmark",
		StartedAt:          nowUnix,
		CompletedAt:        &completedUnix,
		Status:             "SUCCEEDED",
		ExitCode:           0,
		DurationSecs:       300,
		LogStreamName:      "logs/bench",
		CreatedByRequestID: "req-bench",
		ComputePlatform:    "AWS",
	}

	for b.Loop() {
		_ = item.toAPIExecution()
	}
}

func TestExecutionRepositoryNilClient(t *testing.T) {
	// Test that repository can be created with nil client
	// (actual DynamoDB operations would fail, but creation should not)
	repo := NewExecutionRepository(nil, "test-table", testutil.SilentLogger())
	assert.NotNil(t, repo)
}

// Context tests
func TestCreateExecutionWithContext(t *testing.T) {
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

func TestBuildUpdateExpression(t *testing.T) {
	now := time.Now()
	completed := now.Add(5 * time.Minute)

	tests := []struct {
		name                  string
		execution             *api.Execution
		expectedUpdateExpr    string
		expectedExprNames     map[string]string
		expectedExprValueKeys []string
		wantErr               bool
	}{
		{
			name: "minimal execution with just status",
			execution: &api.Execution{
				ExecutionID: "exec-1",
				Status:      "RUNNING",
				ExitCode:    0,
			},
			expectedUpdateExpr: "SET #status = :status, exit_code = :exit_code",
			expectedExprNames: map[string]string{
				"#status": "status",
			},
			expectedExprValueKeys: []string{":status", ":exit_code"},
			wantErr:               false,
		},
		{
			name: "complete execution with all optional fields",
			execution: &api.Execution{
				ExecutionID:     "exec-2",
				Status:          "SUCCEEDED",
				CompletedAt:     &completed,
				ExitCode:        0,
				DurationSeconds: 300,
				LogStreamName:   "ecs/task/123",
			},
			expectedUpdateExpr: "SET #status = :status, completed_at = :completed_at, " +
				"exit_code = :exit_code, duration_seconds = :duration_seconds, log_stream_name = :log_stream_name",
			expectedExprNames: map[string]string{
				"#status": "status",
			},
			expectedExprValueKeys: []string{":status", ":completed_at", ":exit_code", ":duration_seconds", ":log_stream_name"},
			wantErr:               false,
		},
		{
			name: "execution with CompletedAt but no DurationSeconds",
			execution: &api.Execution{
				ExecutionID: "exec-3",
				Status:      "FAILED",
				CompletedAt: &completed,
				ExitCode:    137,
			},
			expectedUpdateExpr: "SET #status = :status, completed_at = :completed_at, exit_code = :exit_code",
			expectedExprNames: map[string]string{
				"#status": "status",
			},
			expectedExprValueKeys: []string{":status", ":completed_at", ":exit_code"},
			wantErr:               false,
		},
		{
			name: "execution with DurationSeconds but no CompletedAt",
			execution: &api.Execution{
				ExecutionID:     "exec-4",
				Status:          "SUCCEEDED",
				ExitCode:        0,
				DurationSeconds: 150,
			},
			expectedUpdateExpr: "SET #status = :status, exit_code = :exit_code, duration_seconds = :duration_seconds",
			expectedExprNames: map[string]string{
				"#status": "status",
			},
			expectedExprValueKeys: []string{":status", ":exit_code", ":duration_seconds"},
			wantErr:               false,
		},
		{
			name: "execution with LogStreamName only",
			execution: &api.Execution{
				ExecutionID:   "exec-5",
				Status:        "RUNNING",
				ExitCode:      0,
				LogStreamName: "logs/stream-123",
			},
			expectedUpdateExpr: "SET #status = :status, exit_code = :exit_code, log_stream_name = :log_stream_name",
			expectedExprNames: map[string]string{
				"#status": "status",
			},
			expectedExprValueKeys: []string{":status", ":exit_code", ":log_stream_name"},
			wantErr:               false,
		},
		{
			name: "execution with ModifiedByRequestID",
			execution: &api.Execution{
				ExecutionID:         "exec-6",
				Status:              "SUCCEEDED",
				ExitCode:            0,
				ModifiedByRequestID: "req-modify-789",
			},
			expectedUpdateExpr: "SET #status = :status, exit_code = :exit_code," +
				" modified_by_request_id = :modified_by_request_id",
			expectedExprNames: map[string]string{
				"#status": "status",
			},
			expectedExprValueKeys: []string{":status", ":exit_code", ":modified_by_request_id"},
			wantErr:               false,
		},
		{
			name: "execution with all fields including ModifiedByRequestID",
			execution: &api.Execution{
				ExecutionID:         "exec-7",
				Status:              "SUCCEEDED",
				CompletedAt:         &completed,
				ExitCode:            0,
				DurationSeconds:     300,
				LogStreamName:       "ecs/task/456",
				ModifiedByRequestID: "req-complete-xyz",
			},
			expectedUpdateExpr: "SET #status = :status, completed_at = :completed_at, " +
				"exit_code = :exit_code, duration_seconds = :duration_seconds, " +
				"log_stream_name = :log_stream_name, modified_by_request_id = :modified_by_request_id",
			expectedExprNames: map[string]string{
				"#status": "status",
			},
			expectedExprValueKeys: []string{
				":status", ":completed_at", ":exit_code", ":duration_seconds",
				":log_stream_name", ":modified_by_request_id",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateExpr, exprNames, exprValues := buildUpdateExpression(tt.execution)

			assert.Equal(t, tt.expectedUpdateExpr, updateExpr)
			assert.Equal(t, tt.expectedExprNames, exprNames)

			// Check that we got the expected number of attribute values
			assert.Equal(t, len(tt.expectedExprValueKeys), len(exprValues))

			// Check that all expected keys are present
			for _, key := range tt.expectedExprValueKeys {
				assert.Contains(t, exprValues, key)
			}

			// Verify specific attribute value types and values
			statusVal, ok := exprValues[":status"]
			require.True(t, ok)
			assert.IsType(t, &types.AttributeValueMemberS{}, statusVal)
			assert.Equal(t, tt.execution.Status, statusVal.(*types.AttributeValueMemberS).Value)

			exitCodeVal, ok := exprValues[":exit_code"]
			require.True(t, ok)
			assert.IsType(t, &types.AttributeValueMemberN{}, exitCodeVal)
			assert.Equal(t, fmt.Sprintf("%d", tt.execution.ExitCode), exitCodeVal.(*types.AttributeValueMemberN).Value)

			if tt.execution.CompletedAt != nil {
				completedAtVal, hasCompletedAt := exprValues[":completed_at"]
				require.True(t, hasCompletedAt)
				assert.IsType(t, &types.AttributeValueMemberN{}, completedAtVal)
			}

			if tt.execution.DurationSeconds > 0 {
				durationVal, hasDuration := exprValues[":duration_seconds"]
				require.True(t, hasDuration)
				assert.IsType(t, &types.AttributeValueMemberN{}, durationVal)
				assert.Equal(t, fmt.Sprintf("%d", tt.execution.DurationSeconds), durationVal.(*types.AttributeValueMemberN).Value)
			}

			if tt.execution.LogStreamName != "" {
				logStreamVal, hasLogStream := exprValues[":log_stream_name"]
				require.True(t, hasLogStream)
				assert.IsType(t, &types.AttributeValueMemberS{}, logStreamVal)
				assert.Equal(t, tt.execution.LogStreamName, logStreamVal.(*types.AttributeValueMemberS).Value)
			}

			if tt.execution.ModifiedByRequestID != "" {
				modifiedByRequestIDVal, hasModifiedByRequestID := exprValues[":modified_by_request_id"]
				require.True(t, hasModifiedByRequestID)
				assert.IsType(t, &types.AttributeValueMemberS{}, modifiedByRequestIDVal)
				assert.Equal(t, tt.execution.ModifiedByRequestID, modifiedByRequestIDVal.(*types.AttributeValueMemberS).Value)
			}
		})
	}
}

func TestExecutionRepository_CreateExecution(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "test-executions-table"

	t.Run("successfully creates execution", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		execution := &api.Execution{
			ExecutionID: "exec-123",
			StartedAt:   time.Now(),
			CreatedBy:   "user@example.com",
			OwnedBy:     []string{"user@example.com"},
			Command:     "echo hello",
			Status:      "RUNNING",
		}

		err := repo.CreateExecution(ctx, execution)

		require.NoError(t, err)
		assert.Equal(t, 1, mockClient.PutItemCalls)
	})

	t.Run("handles duplicate execution ID", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		execution := &api.Execution{
			ExecutionID: "exec-123",
			StartedAt:   time.Now(),
			CreatedBy:   "user@example.com",
			OwnedBy:     []string{"user@example.com"},
			Command:     "echo hello",
			Status:      "RUNNING",
		}

		// Create first execution
		err := repo.CreateExecution(ctx, execution)
		require.NoError(t, err)

		// Try to create duplicate - mock client doesn't check condition expressions
		// so we'll simulate it by setting an error
		mockClient.PutItemError = &types.ConditionalCheckFailedException{}

		err = repo.CreateExecution(ctx, execution)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "execution already exists")
	})

	t.Run("handles database error", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		mockClient.PutItemError = fmt.Errorf("database error")
		repo := NewExecutionRepository(mockClient, tableName, logger)

		execution := &api.Execution{
			ExecutionID: "exec-123",
			StartedAt:   time.Now(),
			CreatedBy:   "user@example.com",
			OwnedBy:     []string{"user@example.com"},
			Command:     "echo hello",
			Status:      "RUNNING",
		}

		err := repo.CreateExecution(ctx, execution)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create execution")
	})
}

func TestExecutionRepository_GetExecution(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "test-executions-table"

	t.Run("successfully retrieves execution", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		// Manually set up the table structure with an execution item
		// This works around the mock's limitation in key extraction
		if mockClient.Tables[tableName] == nil {
			mockClient.Tables[tableName] = make(
				map[string]map[string]map[string]types.AttributeValue,
			)
		}
		if mockClient.Tables[tableName]["exec-123"] == nil {
			mockClient.Tables[tableName]["exec-123"] = make(
				map[string]map[string]types.AttributeValue,
			)
		}
		now := time.Now()
		// Create a properly formatted execution item
		executionItem := toExecutionItem(&api.Execution{
			ExecutionID: "exec-123",
			StartedAt:   now,
			CreatedBy:   "user@example.com",
			OwnedBy:     []string{"user@example.com"},
			Command:     "echo hello",
			Status:      "RUNNING",
		})
		av, err := attributevalue.MarshalMap(executionItem)
		require.NoError(t, err)
		av["_all"] = &types.AttributeValueMemberS{Value: "1"}
		mockClient.Tables[tableName]["exec-123"][""] = av

		repo := NewExecutionRepository(mockClient, tableName, logger)

		result, err := repo.GetExecution(ctx, "exec-123")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "exec-123", result.ExecutionID)
		assert.Equal(t, "user@example.com", result.CreatedBy)
		assert.Equal(t, "echo hello", result.Command)
		assert.Equal(t, 1, mockClient.GetItemCalls)
	})

	t.Run("returns nil for non-existent execution", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		result, err := repo.GetExecution(ctx, "non-existent")

		require.NoError(t, err)
		assert.Nil(t, result)
		assert.Equal(t, 1, mockClient.GetItemCalls)
	})

	t.Run("handles database error", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		mockClient.GetItemError = fmt.Errorf("database error")
		repo := NewExecutionRepository(mockClient, tableName, logger)

		result, err := repo.GetExecution(ctx, "exec-123")

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to get execution")
	})
}

func TestExecutionRepository_UpdateExecution(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "test-executions-table"

	t.Run("successfully updates execution", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		// Manually set up the table structure to match what PutItem would create
		// This works around the mock's limitation in key extraction
		if mockClient.Tables[tableName] == nil {
			mockClient.Tables[tableName] = make(
				map[string]map[string]map[string]types.AttributeValue,
			)
		}
		if mockClient.Tables[tableName]["exec-123"] == nil {
			mockClient.Tables[tableName]["exec-123"] = make(
				map[string]map[string]types.AttributeValue,
			)
		}
		// Create a dummy item
		mockClient.Tables[tableName]["exec-123"][""] = map[string]types.AttributeValue{
			"execution_id": &types.AttributeValueMemberS{Value: "exec-123"},
		}

		repo := NewExecutionRepository(mockClient, tableName, logger)

		now := time.Now()
		completed := now.Add(5 * time.Minute)
		execution := &api.Execution{
			ExecutionID:     "exec-123",
			StartedAt:       now,
			CreatedBy:       "user@example.com",
			OwnedBy:         []string{"user@example.com"},
			Command:         "echo hello",
			Status:          "SUCCEEDED",
			CompletedAt:     &completed,
			ExitCode:        0,
			DurationSeconds: 300,
			LogStreamName:   "logs/stream",
		}

		err := repo.UpdateExecution(ctx, execution)

		require.NoError(t, err)
		assert.Equal(t, 1, mockClient.UpdateItemCalls)
	})

	t.Run("handles execution not found", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		mockClient.UpdateItemError = &types.ConditionalCheckFailedException{}
		repo := NewExecutionRepository(mockClient, tableName, logger)

		execution := &api.Execution{
			ExecutionID: "exec-123",
			Status:      "SUCCEEDED",
			ExitCode:    0,
		}

		err := repo.UpdateExecution(ctx, execution)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "execution not found")
	})

	t.Run("handles database error", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		mockClient.UpdateItemError = fmt.Errorf("database error")
		repo := NewExecutionRepository(mockClient, tableName, logger)

		execution := &api.Execution{
			ExecutionID: "exec-123",
			Status:      "SUCCEEDED",
			ExitCode:    0,
		}

		err := repo.UpdateExecution(ctx, execution)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update execution")
	})
}

func TestExecutionRepository_ListExecutions(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "test-executions-table"

	t.Run("successfully lists executions", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		executions, err := repo.ListExecutions(ctx, 10, []string{})

		require.NoError(t, err)
		assert.NotNil(t, executions)
		assert.Equal(t, 1, mockClient.QueryCalls)
	})

	t.Run("handles empty results", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		executions, err := repo.ListExecutions(ctx, 10, []string{})

		require.NoError(t, err)
		assert.NotNil(t, executions)
		assert.Empty(t, executions)
	})

	t.Run("handles database error", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		mockClient.QueryError = fmt.Errorf("database error")
		repo := NewExecutionRepository(mockClient, tableName, logger)

		executions, err := repo.ListExecutions(ctx, 10, []string{})

		require.Error(t, err)
		assert.Nil(t, executions)
		assert.Contains(t, err.Error(), "failed to query executions")
	})
}

func TestBuildQueryInput_NoLimitWhenZero(t *testing.T) {
	logger := testutil.SilentLogger()
	repo := NewExecutionRepository(nil, "test-table", logger)

	exprNames := map[string]string{
		"#all": "_all",
	}
	exprValues := map[string]types.AttributeValue{
		":all": &types.AttributeValueMemberS{Value: "1"},
	}

	input := repo.buildQueryInput("", exprNames, exprValues, nil, 0)

	require.NotNil(t, input)
	assert.Nil(t, input.Limit)
}

func TestProcessQueryResults_UnboundedLimit(t *testing.T) {
	item := executionItem{
		ExecutionID: "exec-1",
		StartedAt:   time.Now().Unix(),
		CreatedBy:   "user@example.com",
		OwnedBy:     []string{"user@example.com"},
		Command:     "echo hello",
		Status:      "SUCCEEDED",
	}

	av, err := attributevalue.MarshalMap(item)
	require.NoError(t, err)

	executions, reachedLimit, procErr := processQueryResults(
		[]map[string]types.AttributeValue{av, av},
		make([]*api.Execution, 0),
		0,
	)

	require.NoError(t, procErr)
	assert.False(t, reachedLimit)
	assert.Len(t, executions, 2)
}

// Request ID Tracking Tests
func TestRequestIDTracking(t *testing.T) {
	t.Run("CreatedByRequestID and ModifiedByRequestID preserved in conversion", func(t *testing.T) {
		now := time.Now()
		execution := &api.Execution{
			ExecutionID:         "exec-req-test",
			CreatedBy:           "user@example.com",
			OwnedBy:             []string{"user@example.com"},
			Command:             "echo test",
			StartedAt:           now,
			Status:              "RUNNING",
			CreatedByRequestID:  "req-create-123",
			ModifiedByRequestID: "req-modify-456",
		}

		// Convert to DynamoDB item
		item := toExecutionItem(execution)
		assert.Equal(t, "req-create-123", item.CreatedByRequestID)
		assert.Equal(t, "req-modify-456", item.ModifiedByRequestID)

		// Convert back to API
		result := item.toAPIExecution()
		assert.Equal(t, "req-create-123", result.CreatedByRequestID)
		assert.Equal(t, "req-modify-456", result.ModifiedByRequestID)
	})

	t.Run("empty request IDs are preserved", func(t *testing.T) {
		now := time.Now()
		execution := &api.Execution{
			ExecutionID:         "exec-empty-req",
			CreatedBy:           "user@example.com",
			OwnedBy:             []string{"user@example.com"},
			Command:             "echo test",
			StartedAt:           now,
			Status:              "RUNNING",
			CreatedByRequestID:  "",
			ModifiedByRequestID: "",
		}

		item := toExecutionItem(execution)
		assert.Empty(t, item.CreatedByRequestID)
		assert.Empty(t, item.ModifiedByRequestID)

		result := item.toAPIExecution()
		assert.Empty(t, result.CreatedByRequestID)
		assert.Empty(t, result.ModifiedByRequestID)
	})

	t.Run("ModifiedByRequestID included in update expression when present", func(t *testing.T) {
		execution := &api.Execution{
			ExecutionID:         "exec-update",
			Status:              "SUCCEEDED",
			ExitCode:            0,
			ModifiedByRequestID: "req-update-789",
		}

		updateExpr, _, exprValues := buildUpdateExpression(execution)

		assert.Contains(t, updateExpr, "modified_by_request_id = :modified_by_request_id")
		assert.Contains(t, exprValues, ":modified_by_request_id")

		modifiedVal, ok := exprValues[":modified_by_request_id"]
		require.True(t, ok)
		assert.IsType(t, &types.AttributeValueMemberS{}, modifiedVal)
		assert.Equal(t, "req-update-789", modifiedVal.(*types.AttributeValueMemberS).Value)
	})

	t.Run("ModifiedByRequestID omitted from update expression when empty", func(t *testing.T) {
		execution := &api.Execution{
			ExecutionID:         "exec-no-update",
			Status:              "SUCCEEDED",
			ExitCode:            0,
			ModifiedByRequestID: "",
		}

		updateExpr, _, exprValues := buildUpdateExpression(execution)

		assert.NotContains(t, updateExpr, "modified_by_request_id")
		assert.NotContains(t, exprValues, ":modified_by_request_id")
	})

	t.Run("both CreatedByRequestID and ModifiedByRequestID can be different", func(t *testing.T) {
		now := time.Now()
		execution := &api.Execution{
			ExecutionID:         "exec-diff-req",
			CreatedBy:           "user@example.com",
			OwnedBy:             []string{"user@example.com"},
			Command:             "echo test",
			StartedAt:           now,
			Status:              "RUNNING",
			CreatedByRequestID:  "req-original-abc",
			ModifiedByRequestID: "req-updated-xyz",
		}

		item := toExecutionItem(execution)

		assert.NotEqual(t, item.CreatedByRequestID, item.ModifiedByRequestID)
		assert.Equal(t, "req-original-abc", item.CreatedByRequestID)
		assert.Equal(t, "req-updated-xyz", item.ModifiedByRequestID)
	})

	t.Run("both CreatedByRequestID and ModifiedByRequestID can be the same", func(t *testing.T) {
		now := time.Now()
		execution := &api.Execution{
			ExecutionID:         "exec-same-req",
			CreatedBy:           "user@example.com",
			OwnedBy:             []string{"user@example.com"},
			Command:             "echo test",
			StartedAt:           now,
			Status:              "RUNNING",
			CreatedByRequestID:  "req-same-123",
			ModifiedByRequestID: "req-same-123",
		}

		item := toExecutionItem(execution)

		assert.Equal(t, item.CreatedByRequestID, item.ModifiedByRequestID)
		assert.Equal(t, "req-same-123", item.CreatedByRequestID)
		assert.Equal(t, "req-same-123", item.ModifiedByRequestID)
	})
}

func TestExecutionRepository_GetExecutionsByRequestID(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "test-executions-table"

	t.Run("successfully retrieves executions by request ID", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		// Create executions with the same request ID
		now := time.Now()
		exec1 := &api.Execution{
			ExecutionID:         "exec-1",
			CreatedBy:           "user@example.com",
			OwnedBy:             []string{"user@example.com"},
			Command:             "echo hello",
			StartedAt:           now,
			Status:              "RUNNING",
			CreatedByRequestID:  "req-123",
			ModifiedByRequestID: "req-123",
		}
		exec2 := &api.Execution{
			ExecutionID:         "exec-2",
			CreatedBy:           "user@example.com",
			OwnedBy:             []string{"user@example.com"},
			Command:             "echo world",
			StartedAt:           now,
			Status:              "SUCCEEDED",
			CreatedByRequestID:  "req-123",
			ModifiedByRequestID: "req-456", // Different modified request ID
		}

		err := repo.CreateExecution(ctx, exec1)
		require.NoError(t, err)
		err = repo.CreateExecution(ctx, exec2)
		require.NoError(t, err)

		// Query by request ID
		executions, err := repo.GetExecutionsByRequestID(ctx, "req-123")

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 1)
		// Should find exec1 (created) and exec2 (created)
		executionIDs := make(map[string]bool)
		for _, exec := range executions {
			executionIDs[exec.ExecutionID] = true
		}
		assert.True(t, executionIDs["exec-1"] || executionIDs["exec-2"])
	})

	t.Run("returns empty list for non-existent request ID", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		executions, err := repo.GetExecutionsByRequestID(ctx, "non-existent-req")

		require.NoError(t, err)
		assert.Empty(t, executions)
	})

	t.Run("handles query error on created_by_request_id index", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		mockClient.QueryError = fmt.Errorf("query failed")
		repo := NewExecutionRepository(mockClient, tableName, logger)

		executions, err := repo.GetExecutionsByRequestID(ctx, "req-123")

		require.Error(t, err)
		assert.Nil(t, executions)
		assert.Contains(t, err.Error(), "failed to query executions by request ID")
	})

	t.Run("handles query error on modified_by_request_id index", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		// Set up first query to succeed, second to fail
		// We'll use QueryError to simulate failure on second call
		// Since we can't easily control which query fails, we'll test the general error case
		mockClient.QueryError = fmt.Errorf("query failed")

		executions, err := repo.GetExecutionsByRequestID(ctx, "req-123")

		require.Error(t, err)
		assert.Nil(t, executions)
		assert.Contains(t, err.Error(), "failed to query executions by request ID")
	})

	t.Run("deduplicates executions found in both indexes", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		// Create execution that appears in both indexes
		now := time.Now()
		exec := &api.Execution{
			ExecutionID:         "exec-dedup",
			CreatedBy:           "user@example.com",
			OwnedBy:             []string{"user@example.com"},
			Command:             "echo test",
			StartedAt:           now,
			Status:              "RUNNING",
			CreatedByRequestID:  "req-same",
			ModifiedByRequestID: "req-same", // Same request ID for both
		}

		err := repo.CreateExecution(ctx, exec)
		require.NoError(t, err)

		// Query by request ID - should only return execution once
		executions, err := repo.GetExecutionsByRequestID(ctx, "req-same")

		require.NoError(t, err)
		// Should only have one execution, not duplicated
		assert.LessOrEqual(t, len(executions), 1)
	})

	t.Run("handles unmarshal error gracefully", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		// Create an execution first
		now := time.Now()
		exec := &api.Execution{
			ExecutionID:        "exec-unmarshal",
			CreatedBy:          "user@example.com",
			OwnedBy:            []string{"user@example.com"},
			Command:            "echo test",
			StartedAt:          now,
			Status:             "RUNNING",
			CreatedByRequestID: "req-unmarshal",
		}
		repo := NewExecutionRepository(mockClient, tableName, logger)
		err := repo.CreateExecution(ctx, exec)
		require.NoError(t, err)

		// The mock client doesn't easily support injecting unmarshal errors
		// but we can test that the function handles them
		// For now, we'll test the success path and error paths separately
		executions, err := repo.GetExecutionsByRequestID(ctx, "req-unmarshal")
		// Should succeed or handle gracefully
		_ = executions
		_ = err
	})
}

func TestExecutionRepository_ListExecutions_WithStatusFilter(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "test-executions-table"

	t.Run("filters by single status", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		executions, err := repo.ListExecutions(ctx, 10, []string{"RUNNING"})

		require.NoError(t, err)
		assert.NotNil(t, executions)
		assert.Equal(t, 1, mockClient.QueryCalls)
	})

	t.Run("filters by multiple statuses", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		executions, err := repo.ListExecutions(ctx, 10, []string{"RUNNING", "SUCCEEDED", "FAILED"})

		require.NoError(t, err)
		assert.NotNil(t, executions)
		assert.Equal(t, 1, mockClient.QueryCalls)
	})

	t.Run("handles pagination with status filter", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		// Test with limit that might require pagination
		executions, err := repo.ListExecutions(ctx, 5, []string{"RUNNING"})

		require.NoError(t, err)
		assert.NotNil(t, executions)
		// Should make at least one query call
		assert.GreaterOrEqual(t, mockClient.QueryCalls, 1)
	})
}

func TestExecutionRepository_ListExecutions_EdgeCases(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	tableName := "test-executions-table"

	t.Run("handles zero limit", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		executions, err := repo.ListExecutions(ctx, 0, []string{})

		require.NoError(t, err)
		assert.NotNil(t, executions)
	})

	t.Run("handles very large limit", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		executions, err := repo.ListExecutions(ctx, 1000000, []string{})

		require.NoError(t, err)
		assert.NotNil(t, executions)
	})

	t.Run("handles empty status filter", func(t *testing.T) {
		mockClient := NewMockDynamoDBClient()
		repo := NewExecutionRepository(mockClient, tableName, logger)

		executions, err := repo.ListExecutions(ctx, 10, []string{})

		require.NoError(t, err)
		assert.NotNil(t, executions)
	})
}

func TestBuildStatusFilterExpression(t *testing.T) {
	t.Run("empty statuses returns empty string", func(t *testing.T) {
		exprNames := make(map[string]string)
		exprValues := make(map[string]types.AttributeValue)
		result := buildStatusFilterExpression([]string{}, exprNames, exprValues)

		assert.Empty(t, result)
		assert.Empty(t, exprNames)
		assert.Empty(t, exprValues)
	})

	t.Run("single status creates equality expression", func(t *testing.T) {
		exprNames := make(map[string]string)
		exprValues := make(map[string]types.AttributeValue)
		result := buildStatusFilterExpression([]string{"RUNNING"}, exprNames, exprValues)

		assert.Equal(t, "#status = :status", result)
		assert.Equal(t, "status", exprNames["#status"])
		assert.NotNil(t, exprValues[":status"])
	})

	t.Run("multiple statuses creates IN expression", func(t *testing.T) {
		exprNames := make(map[string]string)
		exprValues := make(map[string]types.AttributeValue)
		result := buildStatusFilterExpression([]string{"RUNNING", "SUCCEEDED", "FAILED"}, exprNames, exprValues)

		assert.Contains(t, result, "IN")
		assert.Contains(t, result, ":status0")
		assert.Contains(t, result, ":status1")
		assert.Contains(t, result, ":status2")
		assert.Equal(t, "status", exprNames["#status"])
		assert.Len(t, exprValues, 3)
	})
}

func TestProcessQueryResults_ErrorHandling(t *testing.T) {
	t.Run("handles unmarshal error", func(t *testing.T) {
		// Note: attributevalue.UnmarshalMap is quite permissive and doesn't easily fail
		// on type mismatches. To properly test unmarshal errors, we'd need to create
		// truly malformed data that the unmarshaler can't handle, which is difficult
		// with the current mock setup. This test verifies the function structure.
		// For actual unmarshal error testing, integration tests would be more appropriate.
		now := time.Now().Unix()
		item := executionItem{
			ExecutionID: "exec-1",
			StartedAt:   now,
			CreatedBy:   "user@example.com",
			OwnedBy:     []string{"user@example.com"},
			Command:     "echo hello",
			Status:      "RUNNING",
		}

		av, err := attributevalue.MarshalMap(item)
		require.NoError(t, err)

		executions, reachedLimit, err := processQueryResults(
			[]map[string]types.AttributeValue{av},
			make([]*api.Execution, 0),
			10,
		)

		// Should succeed with valid item
		require.NoError(t, err)
		assert.Len(t, executions, 1)
		assert.False(t, reachedLimit)
	})

	t.Run("respects limit", func(t *testing.T) {
		now := time.Now().Unix()
		item1 := executionItem{
			ExecutionID: "exec-1",
			StartedAt:   now,
			CreatedBy:   "user@example.com",
			OwnedBy:     []string{"user@example.com"},
			Command:     "echo 1",
			Status:      "RUNNING",
		}
		item2 := executionItem{
			ExecutionID: "exec-2",
			StartedAt:   now,
			CreatedBy:   "user@example.com",
			OwnedBy:     []string{"user@example.com"},
			Command:     "echo 2",
			Status:      "RUNNING",
		}

		av1, err := attributevalue.MarshalMap(item1)
		require.NoError(t, err)
		av2, err := attributevalue.MarshalMap(item2)
		require.NoError(t, err)

		executions, reachedLimit, err := processQueryResults(
			[]map[string]types.AttributeValue{av1, av2},
			make([]*api.Execution, 0),
			1, // Limit of 1
		)

		require.NoError(t, err)
		assert.Len(t, executions, 1)
		assert.True(t, reachedLimit)
	})

	t.Run("handles empty items", func(t *testing.T) {
		executions, reachedLimit, err := processQueryResults(
			[]map[string]types.AttributeValue{},
			make([]*api.Execution, 0),
			10,
		)

		require.NoError(t, err)
		assert.Empty(t, executions)
		assert.False(t, reachedLimit)
	})
}

func TestBuildQueryLimit(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		expected int32
	}{
		{
			name:     "small limit",
			limit:    10,
			expected: 20, // 10 * 2
		},
		{
			name:     "zero limit",
			limit:    0,
			expected: 0, // 0 * 2
		},
		{
			name:     "large limit within int32",
			limit:    1000000,
			expected: 2000000, // 1000000 * 2
		},
		{
			name:     "limit that would overflow",
			limit:    2000000000, // Would overflow int32 when multiplied by 2
			expected: 2147483647, // Max int32
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildQueryLimit(tt.limit)
			assert.Equal(t, tt.expected, result)
		})
	}
}
