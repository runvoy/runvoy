package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"runvoy/internal/constants"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlattenMapAttr(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		value    any
		expected string
	}{
		{
			name:   "simple map[string]string",
			prefix: "",
			value: map[string]string{
				"status":   "SUCCESS",
				"deadline": "none",
			},
			expected: "deadline=none status=SUCCESS", // sorted alphabetically
		},
		{
			name:   "simple map[string]any",
			prefix: "",
			value: map[string]any{
				"count": 42,
				"name":  "test",
			},
			expected: "count=42 name=test",
		},
		{
			name:   "nested map",
			prefix: "task",
			value: map[string]any{
				"id": "123",
				"meta": map[string]string{
					"owner": "admin",
				},
			},
			expected: "task.id=123 task.meta.owner=admin",
		},
		{
			name:     "non-map value",
			prefix:   "",
			value:    "simple string",
			expected: "simple string",
		},
		{
			name:     "number value",
			prefix:   "",
			value:    42,
			expected: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := flattenMapAttr(tt.prefix, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReplaceAttrForDev(t *testing.T) {
	tests := []struct {
		name     string
		attr     slog.Attr
		expected string // expected value after transformation
	}{
		{
			name: "map[string]string gets flattened",
			attr: slog.Any("task", map[string]string{
				"id":     "123",
				"status": "running",
			}),
			expected: "task.id=123 task.status=running",
		},
		{
			name: "map[string]any gets flattened",
			attr: slog.Any("data", map[string]any{
				"count": 5,
			}),
			expected: "data.count=5",
		},
		{
			name:     "string attr unchanged",
			attr:     slog.String("message", "hello"),
			expected: "hello",
		},
		{
			name:     "int attr unchanged",
			attr:     slog.Int("count", 42),
			expected: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceAttrForDev(nil, tt.attr)

			// For map types, check the flattened string value
			if tt.name == "map[string]string gets flattened" || tt.name == "map[string]any gets flattened" {
				assert.Equal(t, tt.expected, result.Value.String())
			} else {
				assert.Contains(t, result.Value.String(), tt.expected)
			}
		})
	}
}

func TestInitialize(t *testing.T) {
	tests := []struct {
		name  string
		env   constants.Environment
		level slog.Level
	}{
		{
			name:  "production environment with info level",
			env:   constants.Production,
			level: slog.LevelInfo,
		},
		{
			name:  "development environment with debug level",
			env:   constants.Development,
			level: slog.LevelDebug,
		},
		{
			name:  "CLI environment with warn level",
			env:   constants.CLI,
			level: slog.LevelWarn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := Initialize(tt.env, tt.level)

			assert.NotNil(t, logger, "Logger should not be nil")
			assert.Equal(t, logger, slog.Default(), "Logger should be set as default")
		})
	}
}

func TestGetRequestID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "context with request ID",
			ctx:      context.WithValue(context.Background(), requestIDContextKey, "test-request-123"),
			expected: "test-request-123",
		},
		{
			name:     "context with wrong type",
			ctx:      context.WithValue(context.Background(), requestIDContextKey, 12345),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetRequestID(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "test-request-123"

	ctx = WithRequestID(ctx, requestID)
	retrieved := GetRequestID(ctx)

	assert.Equal(t, requestID, retrieved)
}

func TestDeriveRequestLogger(t *testing.T) {
	t.Run("nil base logger returns default", func(t *testing.T) {
		logger := DeriveRequestLogger(context.Background(), nil)
		assert.NotNil(t, logger)
	})

	t.Run("context with request ID", func(t *testing.T) {
		buf := &bytes.Buffer{}
		baseLogger := slog.New(slog.NewJSONHandler(buf, nil))
		ctx := context.WithValue(context.Background(), requestIDContextKey, "req-123")

		logger := DeriveRequestLogger(ctx, baseLogger)
		logger.Info("test message", "context", map[string]string{"key": "value"})

		output := buf.String()
		assert.Contains(t, output, "req-123", "Output should contain request ID")
		assert.Contains(t, output, "test message")
		// Verify requestID is at root level, not nested in context object
		var logEntry map[string]any
		err := json.Unmarshal([]byte(output), &logEntry)
		require.NoError(t, err)
		assert.Equal(t, "req-123", logEntry[constants.RequestIDLogField], "request_id should be at root level")
		assert.NotContains(t, logEntry, "context.request_id", "request_id should not be nested in context")
	})

	t.Run("context with AWS Lambda request ID", func(t *testing.T) {
		// Save and restore original extractors
		originalExtractors := contextExtractors
		defer func() { contextExtractors = originalExtractors }()

		ClearContextExtractors()

		// Register a mock Lambda extractor
		lambdaExtractor := &mockContextExtractor{
			requestID:  "lambda-req-456",
			shouldFind: true,
		}
		RegisterContextExtractor(lambdaExtractor)

		buf := &bytes.Buffer{}
		baseLogger := slog.New(slog.NewJSONHandler(buf, nil))

		// The extractor will be called even though we're not using Lambda context
		// This simulates what would happen with the real AWS Lambda extractor
		logger := DeriveRequestLogger(context.Background(), baseLogger)
		logger.Info("lambda test")

		output := buf.String()
		assert.Contains(t, output, "lambda-req-456", "Output should contain Lambda request ID")
	})

	t.Run("context without request ID", func(t *testing.T) {
		buf := &bytes.Buffer{}
		baseLogger := slog.New(slog.NewJSONHandler(buf, nil))

		logger := DeriveRequestLogger(context.Background(), baseLogger)
		assert.NotNil(t, logger)

		logger.Info("no request id")
		output := buf.String()
		assert.Contains(t, output, "no request id")
	})
}

func TestGetDeadlineInfo(t *testing.T) {
	t.Run("context without deadline", func(t *testing.T) {
		ctx := context.Background()
		attrs := GetDeadlineInfo(ctx)

		require.Len(t, attrs, 4)
		assert.Equal(t, "deadline", attrs[0])
		assert.Equal(t, "none", attrs[1])
		assert.Equal(t, "deadline_remaining", attrs[2])
		assert.Equal(t, "none", attrs[3])
	})

	t.Run("context with deadline", func(t *testing.T) {
		deadline := time.Now().Add(5 * time.Minute)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		attrs := GetDeadlineInfo(ctx)

		require.Len(t, attrs, 4)
		assert.Equal(t, "deadline", attrs[0])
		assert.Contains(t, attrs[1].(string), "T", "Should contain RFC3339 formatted time")
		assert.Equal(t, "deadline_remaining", attrs[2])
		assert.NotEqual(t, "none", attrs[3])
	})
}

func TestSliceToMap(t *testing.T) {
	tests := []struct {
		name     string
		args     []any
		expected map[string]any
	}{
		{
			name:     "empty slice",
			args:     []any{},
			expected: map[string]any{},
		},
		{
			name: "valid key-value pairs",
			args: []any{"key1", "value1", "key2", 42, "key3", true},
			expected: map[string]any{
				"key1": "value1",
				"key2": 42,
				"key3": true,
			},
		},
		{
			name: "odd number of elements",
			args: []any{"key1", "value1", "key2"},
			expected: map[string]any{
				"key1": "value1",
			},
		},
		{
			name: "non-string keys are skipped",
			args: []any{"key1", "value1", 123, "value2", "key3", "value3"},
			expected: map[string]any{
				"key1": "value1",
				"key3": "value3",
			},
		},
		{
			name: "mixed types",
			args: []any{
				"string", "text",
				"number", 123,
				"bool", false,
				"map", map[string]string{"nested": "value"},
			},
			expected: map[string]any{
				"string": "text",
				"number": 123,
				"bool":   false,
				"map":    map[string]string{"nested": "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SliceToMap(tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSliceToMapEdgeCases(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		result := SliceToMap(nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("single element", func(t *testing.T) {
		result := SliceToMap([]any{"lonely"})
		assert.Empty(t, result)
	})

	t.Run("all non-string keys", func(t *testing.T) {
		result := SliceToMap([]any{1, "a", 2, "b", 3, "c"})
		assert.Empty(t, result)
	})
}

// mockContextExtractor is a test implementation of ContextExtractor.
type mockContextExtractor struct {
	requestID  string
	shouldFind bool
}

func (m *mockContextExtractor) ExtractRequestID(_ context.Context) (string, bool) {
	return m.requestID, m.shouldFind
}

func TestRegisterContextExtractor(t *testing.T) {
	t.Run("registers extractor and appends to list", func(t *testing.T) {
		// Save and restore original extractors
		originalExtractors := contextExtractors
		defer func() { contextExtractors = originalExtractors }()

		ClearContextExtractors()

		extractor1 := &mockContextExtractor{requestID: "ext1", shouldFind: true}
		extractor2 := &mockContextExtractor{requestID: "ext2", shouldFind: true}

		RegisterContextExtractor(extractor1)
		RegisterContextExtractor(extractor2)

		// Verify both extractors are registered
		require.Len(t, contextExtractors, 2)
		assert.Equal(t, extractor1, contextExtractors[0])
		assert.Equal(t, extractor2, contextExtractors[1])
	})
}

func TestClearContextExtractors(t *testing.T) {
	t.Run("clears all registered extractors", func(t *testing.T) {
		// Save and restore original extractors
		originalExtractors := contextExtractors
		defer func() { contextExtractors = originalExtractors }()

		// Register some extractors
		extractor1 := &mockContextExtractor{requestID: "ext1", shouldFind: true}
		extractor2 := &mockContextExtractor{requestID: "ext2", shouldFind: true}
		RegisterContextExtractor(extractor1)
		RegisterContextExtractor(extractor2)

		// Verify extractors are registered
		require.Len(t, contextExtractors, 2)

		// Clear extractors
		ClearContextExtractors()

		// Verify extractors are cleared
		assert.Nil(t, contextExtractors)
		assert.Len(t, contextExtractors, 0)
	})
}

func TestContextExtractor(t *testing.T) {
	t.Run("custom extractor can be registered and used", func(t *testing.T) {
		// Save and restore original extractors
		originalExtractors := contextExtractors
		defer func() { contextExtractors = originalExtractors }()

		ClearContextExtractors()

		// Register a custom extractor
		customExtractor := &mockContextExtractor{
			requestID:  "custom-req-789",
			shouldFind: true,
		}
		RegisterContextExtractor(customExtractor)

		buf := &bytes.Buffer{}
		baseLogger := slog.New(slog.NewJSONHandler(buf, nil))

		logger := DeriveRequestLogger(context.Background(), baseLogger)
		logger.Info("test with custom extractor")

		output := buf.String()
		assert.Contains(t, output, "custom-req-789", "Output should contain custom request ID")
	})

	t.Run("multiple extractors are tried in order", func(t *testing.T) {
		// Save and restore original extractors
		originalExtractors := contextExtractors
		defer func() { contextExtractors = originalExtractors }()

		ClearContextExtractors()

		// Register multiple extractors - first one doesn't find, second one does
		extractor1 := &mockContextExtractor{shouldFind: false}
		extractor2 := &mockContextExtractor{requestID: "second-extractor", shouldFind: true}
		extractor3 := &mockContextExtractor{requestID: "third-extractor", shouldFind: true}

		RegisterContextExtractor(extractor1)
		RegisterContextExtractor(extractor2)
		RegisterContextExtractor(extractor3)

		buf := &bytes.Buffer{}
		baseLogger := slog.New(slog.NewJSONHandler(buf, nil))

		logger := DeriveRequestLogger(context.Background(), baseLogger)
		logger.Info("test multiple extractors")

		output := buf.String()
		assert.Contains(t, output, "second-extractor", "Should use first successful extractor")
		assert.NotContains(t, output, "third-extractor", "Should not try remaining extractors")
	})

	t.Run("WithRequestID takes precedence over extractors", func(t *testing.T) {
		// Save and restore original extractors
		originalExtractors := contextExtractors
		defer func() { contextExtractors = originalExtractors }()

		ClearContextExtractors()

		// Register an extractor
		customExtractor := &mockContextExtractor{
			requestID:  "extractor-id",
			shouldFind: true,
		}
		RegisterContextExtractor(customExtractor)

		buf := &bytes.Buffer{}
		baseLogger := slog.New(slog.NewJSONHandler(buf, nil))

		// Set request ID via WithRequestID
		ctx := WithRequestID(context.Background(), "context-value-id")

		logger := DeriveRequestLogger(ctx, baseLogger)
		logger.Info("test precedence")

		output := buf.String()
		assert.Contains(t, output, "context-value-id", "Should use WithRequestID value")
		assert.NotContains(t, output, "extractor-id", "Should not use extractor when WithRequestID is set")
	})
}

func TestExtractRequestIDFromContext(t *testing.T) {
	t.Run("returns empty string when no request ID available", func(t *testing.T) {
		// Save and restore original extractors
		originalExtractors := contextExtractors
		defer func() { contextExtractors = originalExtractors }()

		ClearContextExtractors()

		result := ExtractRequestIDFromContext(context.Background())
		assert.Empty(t, result)
	})

	t.Run("returns request ID from WithRequestID", func(t *testing.T) {
		ctx := WithRequestID(context.Background(), "test-request-id")
		result := ExtractRequestIDFromContext(ctx)
		assert.Equal(t, "test-request-id", result)
	})

	t.Run("returns request ID from registered extractor", func(t *testing.T) {
		// Save and restore original extractors
		originalExtractors := contextExtractors
		defer func() { contextExtractors = originalExtractors }()

		ClearContextExtractors()

		customExtractor := &mockContextExtractor{
			requestID:  "extractor-request-id",
			shouldFind: true,
		}
		RegisterContextExtractor(customExtractor)

		result := ExtractRequestIDFromContext(context.Background())
		assert.Equal(t, "extractor-request-id", result)
	})

	t.Run("WithRequestID takes precedence over extractors", func(t *testing.T) {
		// Save and restore original extractors
		originalExtractors := contextExtractors
		defer func() { contextExtractors = originalExtractors }()

		ClearContextExtractors()

		customExtractor := &mockContextExtractor{
			requestID:  "extractor-id",
			shouldFind: true,
		}
		RegisterContextExtractor(customExtractor)

		ctx := WithRequestID(context.Background(), "context-priority-id")
		result := ExtractRequestIDFromContext(ctx)
		assert.Equal(t, "context-priority-id", result)
	})

	t.Run("tries extractors in order until one succeeds", func(t *testing.T) {
		// Save and restore original extractors
		originalExtractors := contextExtractors
		defer func() { contextExtractors = originalExtractors }()

		ClearContextExtractors()

		extractor1 := &mockContextExtractor{shouldFind: false}
		extractor2 := &mockContextExtractor{requestID: "second-extractor-id", shouldFind: true}
		extractor3 := &mockContextExtractor{requestID: "third-extractor-id", shouldFind: true}

		RegisterContextExtractor(extractor1)
		RegisterContextExtractor(extractor2)
		RegisterContextExtractor(extractor3)

		result := ExtractRequestIDFromContext(context.Background())
		assert.Equal(t, "second-extractor-id", result)
	})
}
