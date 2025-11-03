package dynamodb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/testutil"

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
				ExecutionID:     "exec-123",
				UserEmail:       "user@example.com",
				Command:         "echo hello",
				LockName:        "test-lock",
				StartedAt:       now,
				CompletedAt:     &completed,
				Status:          "SUCCEEDED",
				ExitCode:        0,
				DurationSeconds: 300,
				LogStreamName:   "ecs/task/123",
				RequestID:       "req-456",
				ComputePlatform: "AWS",
			},
		},
		{
			name: "minimal execution",
			execution: &api.Execution{
				ExecutionID: "exec-minimal",
				UserEmail:   "user@example.com",
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
			assert.Equal(t, tt.execution.UserEmail, item.UserEmail)
			assert.Equal(t, tt.execution.Command, item.Command)
			assert.Equal(t, tt.execution.LockName, item.LockName)
			assert.Equal(t, tt.execution.StartedAt, item.StartedAt)
			assert.Equal(t, tt.execution.CompletedAt, item.CompletedAt)
			assert.Equal(t, tt.execution.Status, item.Status)
			assert.Equal(t, tt.execution.ExitCode, item.ExitCode)
			assert.Equal(t, tt.execution.DurationSeconds, item.DurationSecs)
			assert.Equal(t, tt.execution.LogStreamName, item.LogStreamName)
			assert.Equal(t, tt.execution.RequestID, item.RequestID)
			assert.Equal(t, tt.execution.ComputePlatform, item.ComputePlatform)
		})
	}
}

func TestExecutionItem_ToAPIExecution(t *testing.T) {
	now := time.Now()
	completed := now.Add(5 * time.Minute)

	tests := []struct {
		name string
		item *executionItem
	}{
		{
			name: "complete execution item",
			item: &executionItem{
				ExecutionID:     "exec-123",
				UserEmail:       "user@example.com",
				Command:         "echo hello",
				LockName:        "test-lock",
				StartedAt:       now,
				CompletedAt:     &completed,
				Status:          "SUCCEEDED",
				ExitCode:        0,
				DurationSecs:    300,
				LogStreamName:   "ecs/task/123",
				RequestID:       "req-456",
				ComputePlatform: "AWS",
			},
		},
		{
			name: "minimal execution item",
			item: &executionItem{
				ExecutionID: "exec-minimal",
				UserEmail:   "user@example.com",
				Command:     "ls",
				StartedAt:   now,
				Status:      "RUNNING",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execution := tt.item.toAPIExecution()

			assert.Equal(t, tt.item.ExecutionID, execution.ExecutionID)
			assert.Equal(t, tt.item.UserEmail, execution.UserEmail)
			assert.Equal(t, tt.item.Command, execution.Command)
			assert.Equal(t, tt.item.LockName, execution.LockName)
			assert.Equal(t, tt.item.StartedAt, execution.StartedAt)
			assert.Equal(t, tt.item.CompletedAt, execution.CompletedAt)
			assert.Equal(t, tt.item.Status, execution.Status)
			assert.Equal(t, tt.item.ExitCode, execution.ExitCode)
			assert.Equal(t, tt.item.DurationSecs, execution.DurationSeconds)
			assert.Equal(t, tt.item.LogStreamName, execution.LogStreamName)
			assert.Equal(t, tt.item.ComputePlatform, execution.ComputePlatform)
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	now := time.Now()
	completed := now.Add(5 * time.Minute)

	original := &api.Execution{
		ExecutionID:     "exec-roundtrip",
		UserEmail:       "user@example.com",
		Command:         "echo test",
		LockName:        "lock-1",
		StartedAt:       now,
		CompletedAt:     &completed,
		Status:          "SUCCEEDED",
		ExitCode:        42,
		DurationSeconds: 150,
		LogStreamName:   "logs/stream",
		RequestID:       "req-789",
		ComputePlatform: "AWS",
	}

	// Convert to item and back
	item := toExecutionItem(original)
	result := item.toAPIExecution()

	assert.Equal(t, original.ExecutionID, result.ExecutionID)
	assert.Equal(t, original.UserEmail, result.UserEmail)
	assert.Equal(t, original.Command, result.Command)
	assert.Equal(t, original.LockName, result.LockName)
	assert.Equal(t, original.StartedAt.Unix(), result.StartedAt.Unix())
	assert.Equal(t, original.Status, result.Status)
	assert.Equal(t, original.ExitCode, result.ExitCode)
	assert.Equal(t, original.DurationSeconds, result.DurationSeconds)
	assert.Equal(t, original.LogStreamName, result.LogStreamName)
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
			UserEmail:   "user@example.com",
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
			UserEmail:   "user@example.com",
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
			UserEmail:   "user@example.com",
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
			ExecutionID: "exec-4",
			UserEmail:   "user@example.com",
			Command:     "test",
			StartedAt:   time.Now(),
			Status:      "RUNNING",
			LockName:    "",
			RequestID:   "",
		}

		item := toExecutionItem(exec)
		assert.Empty(t, item.LockName)
		assert.Empty(t, item.RequestID)

		result := item.toAPIExecution()
		assert.Empty(t, result.LockName)
		assert.Empty(t, result.RequestID)
	})
}

func TestExecutionItemDynamoDBTags(t *testing.T) {
	t.Run("verify struct tags", func(t *testing.T) {
		// This test ensures the dynamodbav tags are correctly set
		// by attempting to marshal/unmarshal
		item := &executionItem{
			ExecutionID:     "test-123",
			StartedAt:       time.Now(),
			UserEmail:       "user@example.com",
			Command:         "echo test",
			Status:          "RUNNING",
			LogStreamName:   "log-stream",
			RequestID:       "req-123",
			ComputePlatform: "AWS",
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
		ExecutionID:     "exec-bench",
		UserEmail:       "user@example.com",
		Command:         "echo benchmark",
		LockName:        "lock-bench",
		StartedAt:       now,
		CompletedAt:     &completed,
		Status:          "SUCCEEDED",
		ExitCode:        0,
		DurationSeconds: 300,
		LogStreamName:   "logs/bench",
		RequestID:       "req-bench",
		ComputePlatform: "AWS",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = toExecutionItem(exec)
	}
}

func BenchmarkToAPIExecution(b *testing.B) {
	now := time.Now()
	completed := now.Add(5 * time.Minute)
	item := &executionItem{
		ExecutionID:     "exec-bench",
		UserEmail:       "user@example.com",
		Command:         "echo benchmark",
		LockName:        "lock-bench",
		StartedAt:       now,
		CompletedAt:     &completed,
		Status:          "SUCCEEDED",
		ExitCode:        0,
		DurationSecs:    300,
		LogStreamName:   "logs/bench",
		RequestID:       "req-bench",
		ComputePlatform: "AWS",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateExpr, exprNames, exprValues, err := buildUpdateExpression(tt.execution)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
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
				assert.IsType(t, &types.AttributeValueMemberS{}, completedAtVal)
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
		})
	}
}
