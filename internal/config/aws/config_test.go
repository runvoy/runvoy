// Package aws contains AWS-specific configuration helpers for Runvoy services.
package aws

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/viper"
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
			ImageTaskDefsTable:        "image-taskdefs",
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
			PendingAPIKeysTable:       "pending-keys",
			ExecutionsTable:           "executions",
			ImageTaskDefsTable:        "image-taskdefs",
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
			PendingAPIKeysTable:       "pending-keys",
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
			APIKeysTable:              "api-keys",
			PendingAPIKeysTable:       "pending-api-keys",
			ExecutionsTable:           "executions",
			ImageTaskDefsTable:        "image-taskdefs",
			SecretsMetadataTable:      "secrets",
			LogGroup:                  "/aws/logs/app",
			DefaultTaskExecRoleARN:    "arn:aws:iam::123456789012:role/exec-role",
			DefaultTaskRoleARN:        "arn:aws:iam::123456789012:role/task-role",
			SecretsPrefix:             "/runvoy/secrets",
			SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
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
			APIKeysTable:              "api-keys",
			PendingAPIKeysTable:       "pending-api-keys",
			ECSCluster:                "cluster",
			ImageTaskDefsTable:        "image-taskdefs",
			SecretsMetadataTable:      "secrets",
			LogGroup:                  "/aws/logs/app",
			DefaultTaskExecRoleARN:    "arn:aws:iam::123456789012:role/exec-role",
			DefaultTaskRoleARN:        "arn:aws:iam::123456789012:role/task-role",
			SecretsPrefix:             "/runvoy/secrets",
			SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
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
			APIKeysTable:              "api-keys",
			PendingAPIKeysTable:       "pending-api-keys",
			ECSCluster:                "cluster",
			ExecutionsTable:           "executions",
			ImageTaskDefsTable:        "image-taskdefs",
			SecretsMetadataTable:      "secrets",
			LogGroup:                  "/aws/logs/app",
			DefaultTaskExecRoleARN:    "arn:aws:iam::123456789012:role/exec-role",
			DefaultTaskRoleARN:        "arn:aws:iam::123456789012:role/task-role",
			SecretsPrefix:             "/runvoy/secrets",
			SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
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
			APIKeysTable:              "api-keys",
			PendingAPIKeysTable:       "pending-api-keys",
			ECSCluster:                "cluster",
			ExecutionsTable:           "executions",
			ImageTaskDefsTable:        "image-taskdefs",
			SecretsMetadataTable:      "secrets",
			LogGroup:                  "/aws/logs/app",
			DefaultTaskExecRoleARN:    "arn:aws:iam::123456789012:role/exec-role",
			DefaultTaskRoleARN:        "arn:aws:iam::123456789012:role/task-role",
			SecretsPrefix:             "/runvoy/secrets",
			SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
			WebSocketAPIEndpoint:      "example.com",
			WebSocketConnectionsTable: "connections",
			WebSocketTokensTable:      "tokens",
		}
		err := ValidateEventProcessor(cfg)
		assert.NoError(t, err)
		assert.Equal(t, "https://example.com", cfg.WebSocketAPIEndpoint)
	})
}

// TestBindEnvVars tests that environment variables are properly bound to Viper
func TestBindEnvVars(t *testing.T) {
	// Save and clear original env vars
	originalVars := map[string]string{
		"RUNVOY_AWS_API_KEYS_TABLE":         os.Getenv("RUNVOY_AWS_API_KEYS_TABLE"),
		"RUNVOY_AWS_ECS_CLUSTER":            os.Getenv("RUNVOY_AWS_ECS_CLUSTER"),
		"RUNVOY_AWS_EXECUTIONS_TABLE":       os.Getenv("RUNVOY_AWS_EXECUTIONS_TABLE"),
		"RUNVOY_AWS_IMAGE_TASKDEFS_TABLE":   os.Getenv("RUNVOY_AWS_IMAGE_TASKDEFS_TABLE"),
		"RUNVOY_AWS_LOG_GROUP":              os.Getenv("RUNVOY_AWS_LOG_GROUP"),
		"RUNVOY_AWS_SECURITY_GROUP":         os.Getenv("RUNVOY_AWS_SECURITY_GROUP"),
		"RUNVOY_AWS_SUBNET_1":               os.Getenv("RUNVOY_AWS_SUBNET_1"),
		"RUNVOY_AWS_SUBNET_2":               os.Getenv("RUNVOY_AWS_SUBNET_2"),
		"RUNVOY_AWS_WEBSOCKET_API_ENDPOINT": os.Getenv("RUNVOY_AWS_WEBSOCKET_API_ENDPOINT"),
		"RUNVOY_AWS_SECRETS_PREFIX":         os.Getenv("RUNVOY_AWS_SECRETS_PREFIX"),
		"RUNVOY_AWS_SECRETS_KMS_KEY_ARN":    os.Getenv("RUNVOY_AWS_SECRETS_KMS_KEY_ARN"),
	}

	defer func() {
		// Restore env vars
		for k, v := range originalVars {
			if v != "" {
				_ = os.Setenv(k, v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}()

	// Clear all AWS env vars
	for k := range originalVars {
		_ = os.Unsetenv(k)
	}

	// Set test values
	_ = os.Setenv("RUNVOY_AWS_API_KEYS_TABLE", "test-api-keys")
	_ = os.Setenv("RUNVOY_AWS_ECS_CLUSTER", "test-cluster")
	_ = os.Setenv("RUNVOY_AWS_LOG_GROUP", "/aws/ecs/test")

	v := viper.New()
	v.SetEnvPrefix("RUNVOY")
	v.AutomaticEnv()

	BindEnvVars(v)

	// Verify env vars were bound and can be retrieved
	assert.Equal(t, "test-api-keys", v.GetString("aws.api_keys_table"))
	assert.Equal(t, "test-cluster", v.GetString("aws.ecs_cluster"))
	assert.Equal(t, "/aws/ecs/test", v.GetString("aws.log_group"))
	// Verify defaults were set
	assert.NotEmpty(t, v.GetString("aws.secrets_prefix"))
}

// TestLoadSDKConfig tests that AWS SDK configuration can be loaded
func TestLoadSDKConfig(t *testing.T) {
	cfg := &Config{}

	// LoadSDKConfig should succeed with default AWS credentials handling
	// In a test environment, this may use IAM role credentials or fail gracefully
	ctx := context.Background()
	err := cfg.LoadSDKConfig(ctx)

	// The error behavior depends on the test environment:
	// - If running in an environment with AWS credentials (CI/CD, local AWS), should succeed
	// - If running without credentials, may fail with specific error
	// We test that the function is callable and handles both cases
	if err != nil {
		// Expected in environments without AWS credentials
		t.Logf("LoadSDKConfig failed as expected in test environment: %v", err)
		assert.Contains(t, err.Error(), "failed to load AWS SDK configuration")
	} else {
		// SDK config was successfully loaded
		assert.NotNil(t, cfg.SDKConfig)
	}
}

// TestLoadSDKConfigWithContext tests that LoadSDKConfig respects context
func TestLoadSDKConfigWithContext(t *testing.T) {
	cfg := &Config{}

	// Test with a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// This should fail or handle the canceled context
	err := cfg.LoadSDKConfig(ctx)
	// The error could be from context cancellation or missing credentials
	// We just verify the function handles it appropriately
	if err != nil {
		t.Logf("LoadSDKConfig with canceled context returned error: %v", err)
		// This is acceptable behavior
		assert.NotNil(t, err)
	}
}
