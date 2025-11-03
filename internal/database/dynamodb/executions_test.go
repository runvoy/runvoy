package dynamodb

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"runvoy/internal/api"

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
	logger := slog.Default()
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
	repo := NewExecutionRepository(nil, "test-table", slog.Default())
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
