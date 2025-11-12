// Package secrets provides secret management functionality for the Runvoy orchestrator.
package secrets

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetParameterName(t *testing.T) {
	logger := slog.Default()
	m := NewParameterStoreManager(nil, "/runvoy/secrets", "arn:aws:kms:us-east-1:123456789012:key/abc", logger)

	t.Run("constructs correct parameter name", func(t *testing.T) {
		name := m.getParameterName("db-password")
		assert.Equal(t, "/runvoy/secrets/db-password", name)
	})

	t.Run("handles empty prefix", func(t *testing.T) {
		m2 := NewParameterStoreManager(nil, "", "arn:aws:kms:us-east-1:123456789012:key/abc", logger)
		name := m2.getParameterName("db-password")
		assert.Equal(t, "/db-password", name)
	})

	t.Run("handles prefix without leading slash", func(t *testing.T) {
		m2 := NewParameterStoreManager(nil, "runvoy/secrets", "arn:aws:kms:us-east-1:123456789012:key/abc", logger)
		name := m2.getParameterName("db-password")
		assert.Equal(t, "runvoy/secrets/db-password", name)
	})
}

func TestIsParameterNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "ParameterNotFound error",
			err:      errors.New("ParameterNotFound"),
			expected: true,
		},
		{
			name:     "InvalidParameters error",
			err:      errors.New("InvalidParameters: [/runvoy/secrets/test]"),
			expected: true,
		},
		{
			name:     "other error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "access denied error",
			err:      errors.New("AccessDeniedException"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isParameterNotFound(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewParameterStoreManager(t *testing.T) {
	t.Run("creates manager with correct fields", func(t *testing.T) {
		logger := slog.Default()
		prefix := "/runvoy/secrets"
		kmsKey := "arn:aws:kms:us-east-1:123456789012:key/abc"

		m := NewParameterStoreManager(nil, prefix, kmsKey, logger)

		require.NotNil(t, m)
		assert.Equal(t, prefix, m.secretPrefix)
		assert.Equal(t, kmsKey, m.kmsKeyARN)
		assert.NotNil(t, m.logger)
		assert.Nil(t, m.client)
	})
}

func TestParameterTags(t *testing.T) {
	logger := slog.Default()
	m := NewParameterStoreManager(nil, "/runvoy/secrets", "arn:aws:kms:us-east-1:123456789012:key/abc", logger)

	t.Run("returns correct tags", func(t *testing.T) {
		tags := m.parameterTags()

		require.Len(t, tags, 2)

		// Check Application tag
		assert.Equal(t, "Application", *tags[0].Key)
		assert.Equal(t, "runvoy", *tags[0].Value)

		// Check ManagedBy tag
		assert.Equal(t, "ManagedBy", *tags[1].Key)
		assert.Equal(t, "runvoy-orchestrator", *tags[1].Value)
	})
}
