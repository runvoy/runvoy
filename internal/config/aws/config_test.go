// Package aws contains AWS-specific configuration helpers for Runvoy services.
package aws

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeWebSocketEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "https protocol",
			input:    "https://example.com",
			expected: "example.com",
		},
		{
			name:     "http protocol",
			input:    "http://example.com",
			expected: "example.com",
		},
		{
			name:     "wss protocol",
			input:    "wss://example.com",
			expected: "example.com",
		},
		{
			name:     "ws protocol",
			input:    "ws://example.com",
			expected: "example.com",
		},
		{
			name:     "no protocol",
			input:    "example.com",
			expected: "example.com",
		},
		{
			name:     "with path",
			input:    "https://example.com/path",
			expected: "example.com/path",
		},
		{
			name:     "with whitespace",
			input:    "  https://example.com  ",
			expected: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeWebSocketEndpoint(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateOrchestrator(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		err := ValidateOrchestrator(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "AWS configuration is required")
	})

	t.Run("empty config", func(t *testing.T) {
		cfg := &Config{}
		err := ValidateOrchestrator(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("missing APIKeysTable", func(t *testing.T) {
		cfg := &Config{
			ECSCluster:                "cluster",
			ExecutionsTable:           "executions",
			LogGroup:                  "logs",
			SecurityGroup:             "sg",
			Subnet1:                   "subnet1",
			Subnet2:                   "subnet2",
			WebSocketAPIEndpoint:      "wss://example.com",
			WebSocketConnectionsTable: "connections",
			WebSocketTokensTable:      "tokens",
			SecretsMetadataTable:      "secrets",
			SecretsPrefix:             "/runvoy",
			SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
		}
		err := ValidateOrchestrator(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "APIKeysTable")
	})

	t.Run("missing ECSCluster", func(t *testing.T) {
		cfg := &Config{
			APIKeysTable:              "keys",
			ExecutionsTable:           "executions",
			LogGroup:                  "logs",
			SecurityGroup:             "sg",
			Subnet1:                   "subnet1",
			Subnet2:                   "subnet2",
			WebSocketAPIEndpoint:      "wss://example.com",
			WebSocketConnectionsTable: "connections",
			WebSocketTokensTable:      "tokens",
			SecretsMetadataTable:      "secrets",
			SecretsPrefix:             "/runvoy",
			SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
		}
		err := ValidateOrchestrator(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ECSCluster")
	})

	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{
			APIKeysTable:              "keys",
			ECSCluster:                "cluster",
			ExecutionsTable:           "executions",
			ImageTaskDefsTable:        "image-taskdefs",
			LogGroup:                  "logs",
			SecurityGroup:             "sg",
			Subnet1:                   "subnet1",
			Subnet2:                   "subnet2",
			TaskDefinition:            "task-def",
			DefaultTaskExecRoleARN:    "arn:aws:iam::123456789012:role/exec-role",
			DefaultTaskRoleARN:        "arn:aws:iam::123456789012:role/task-role",
			WebSocketAPIEndpoint:      "wss://example.com",
			WebSocketConnectionsTable: "connections",
			WebSocketTokensTable:      "tokens",
			SecretsMetadataTable:      "secrets",
			SecretsPrefix:             "/runvoy",
			SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
		}
		err := ValidateOrchestrator(cfg)
		assert.NoError(t, err)
	})
}

func TestValidateEventProcessor(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		err := ValidateEventProcessor(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "AWS configuration is required")
	})

	t.Run("empty config", func(t *testing.T) {
		cfg := &Config{}
		err := ValidateEventProcessor(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("missing ECSCluster", func(t *testing.T) {
		cfg := &Config{
			ExecutionsTable:           "executions",
			WebSocketAPIEndpoint:      "wss://example.com",
			WebSocketConnectionsTable: "connections",
			WebSocketTokensTable:      "tokens",
		}
		err := ValidateEventProcessor(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ECSCluster")
	})

	t.Run("missing ExecutionsTable", func(t *testing.T) {
		cfg := &Config{
			ECSCluster:                "cluster",
			WebSocketAPIEndpoint:      "wss://example.com",
			WebSocketConnectionsTable: "connections",
			WebSocketTokensTable:      "tokens",
		}
		err := ValidateEventProcessor(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ExecutionsTable")
	})

	t.Run("valid config and normalization", func(t *testing.T) {
		cfg := &Config{
			ECSCluster:                "cluster",
			ExecutionsTable:           "executions",
			WebSocketAPIEndpoint:      "wss://example.com",
			WebSocketConnectionsTable: "connections",
			WebSocketTokensTable:      "tokens",
		}
		err := ValidateEventProcessor(cfg)
		assert.NoError(t, err)
		// Check that endpoint was normalized and https:// added
		assert.Equal(t, "https://example.com", cfg.WebSocketAPIEndpoint)
	})

	t.Run("endpoint already normalized", func(t *testing.T) {
		cfg := &Config{
			ECSCluster:                "cluster",
			ExecutionsTable:           "executions",
			WebSocketAPIEndpoint:      "example.com",
			WebSocketConnectionsTable: "connections",
			WebSocketTokensTable:      "tokens",
		}
		err := ValidateEventProcessor(cfg)
		assert.NoError(t, err)
		assert.Equal(t, "https://example.com", cfg.WebSocketAPIEndpoint)
	})
}
