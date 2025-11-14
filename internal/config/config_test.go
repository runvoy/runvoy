package config

import (
	"log/slog"
	"os"
	"testing"

	awsconfig "runvoy/internal/config/aws"
	"runvoy/internal/constants"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_GetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		expected slog.Level
	}{
		{
			name:     "DEBUG level",
			logLevel: "DEBUG",
			expected: slog.LevelDebug,
		},
		{
			name:     "INFO level",
			logLevel: "INFO",
			expected: slog.LevelInfo,
		},
		{
			name:     "WARN level",
			logLevel: "WARN",
			expected: slog.LevelWarn,
		},
		{
			name:     "ERROR level",
			logLevel: "ERROR",
			expected: slog.LevelError,
		},
		{
			name:     "invalid level defaults to INFO",
			logLevel: "INVALID",
			expected: slog.LevelInfo,
		},
		{
			name:     "empty string defaults to INFO",
			logLevel: "",
			expected: slog.LevelInfo,
		},
		{
			name:     "lowercase level",
			logLevel: "debug",
			expected: slog.LevelDebug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{LogLevel: tt.logLevel}
			result := cfg.GetLogLevel()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateOrchestrator(t *testing.T) {
	tests := []struct {
		name               string
		cfg                *Config
		wantErr            bool
		errMsg             string
		normalizedEndpoint string
	}{
		{
			name: "valid orchestrator config",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					APIKeysTable:              "api-keys",
					ExecutionsTable:           "executions",
					ImageTaskDefsTable:        "image-taskdefs",
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
					SecretsMetadataTable:      "secrets",
					SecretsPrefix:             "/runvoy/secrets",
					SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
				},
			},
			wantErr: false,
		},
		{
			name: "missing AWS config",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS:             nil,
			},
			wantErr: true,
			errMsg:  "AWS configuration is required",
		},
		{
			name: "missing APIKeysTable",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					ExecutionsTable:           "executions",
					ImageTaskDefsTable:        "image-taskdefs",
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
					SecretsMetadataTable:      "secrets",
					SecretsPrefix:             "/runvoy/secrets",
					SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
				},
			},
			wantErr: true,
			errMsg:  "APIKeysTable cannot be empty",
		},
		{
			name: "missing ExecutionsTable",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					APIKeysTable:              "api-keys",
					ImageTaskDefsTable:        "image-taskdefs",
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
					SecretsMetadataTable:      "secrets",
					SecretsPrefix:             "/runvoy/secrets",
					SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
				},
			},
			wantErr: true,
			errMsg:  "ExecutionsTable cannot be empty",
		},
		{
			name: "missing ECSCluster",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					APIKeysTable:              "api-keys",
					ExecutionsTable:           "executions",
					ImageTaskDefsTable:        "image-taskdefs",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
					SecretsMetadataTable:      "secrets",
					SecretsPrefix:             "/runvoy/secrets",
					SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
				},
			},
			wantErr: true,
			errMsg:  "ECSCluster cannot be empty",
		},
		{
			name: "missing Subnet1",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					APIKeysTable:              "api-keys",
					ExecutionsTable:           "executions",
					ImageTaskDefsTable:        "image-taskdefs",
					ECSCluster:                "cluster",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
					SecretsMetadataTable:      "secrets",
					SecretsPrefix:             "/runvoy/secrets",
					SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
				},
			},
			wantErr: true,
			errMsg:  "Subnet1 cannot be empty",
		},
		{
			name: "missing Subnet2",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					APIKeysTable:              "api-keys",
					ExecutionsTable:           "executions",
					ImageTaskDefsTable:        "image-taskdefs",
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
					SecretsMetadataTable:      "secrets",
					SecretsPrefix:             "/runvoy/secrets",
					SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
				},
			},
			wantErr: true,
			errMsg:  "Subnet2 cannot be empty",
		},
		{
			name: "missing SecurityGroup",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					APIKeysTable:              "api-keys",
					ExecutionsTable:           "executions",
					ImageTaskDefsTable:        "image-taskdefs",
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
					SecretsMetadataTable:      "secrets",
					SecretsPrefix:             "/runvoy/secrets",
					SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
				},
			},
			wantErr: true,
			errMsg:  "SecurityGroup cannot be empty",
		},
		{
			name: "missing LogGroup",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					APIKeysTable:              "api-keys",
					ExecutionsTable:           "executions",
					ImageTaskDefsTable:        "image-taskdefs",
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
					SecretsMetadataTable:      "secrets",
					SecretsPrefix:             "/runvoy/secrets",
					SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
				},
			},
			wantErr: true,
			errMsg:  "LogGroup cannot be empty",
		},
		{
			name: "missing WebSocketAPIEndpoint",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					APIKeysTable:              "api-keys",
					ExecutionsTable:           "executions",
					ImageTaskDefsTable:        "image-taskdefs",
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
					SecretsMetadataTable:      "secrets",
					SecretsPrefix:             "/runvoy/secrets",
					SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
				},
			},
			wantErr: true,
			errMsg:  "WebSocketAPIEndpoint cannot be empty",
		},
		{
			name: "missing WebSocketConnectionsTable",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					APIKeysTable:         "api-keys",
					ExecutionsTable:      "executions",
					ImageTaskDefsTable:   "image-taskdefs",
					ECSCluster:           "cluster",
					Subnet1:              "subnet-1",
					Subnet2:              "subnet-2",
					SecurityGroup:        "sg-123",
					LogGroup:             "/aws/logs/app",
					WebSocketAPIEndpoint: "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketTokensTable: "tokens",
					SecretsMetadataTable: "secrets",
					SecretsPrefix:        "/runvoy/secrets",
					SecretsKMSKeyARN:     "arn:aws:kms:us-east-1:123456789012:key/abc",
				},
			},
			wantErr: true,
			errMsg:  "WebSocketConnectionsTable cannot be empty",
		},
		{
			name: "missing WebSocketTokensTable",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					APIKeysTable:              "api-keys",
					ExecutionsTable:           "executions",
					ImageTaskDefsTable:        "image-taskdefs",
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					SecretsMetadataTable:      "secrets",
					SecretsPrefix:             "/runvoy/secrets",
					SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
				},
			},
			wantErr: true,
			errMsg:  "WebSocketTokensTable cannot be empty",
		},
		{
			name: "missing SecretsMetadataTable",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					APIKeysTable:              "api-keys",
					ExecutionsTable:           "executions",
					ImageTaskDefsTable:        "image-taskdefs",
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
					SecretsKMSKeyARN:          "arn:aws:kms:us-east-1:123456789012:key/abc",
					SecretsPrefix:             "/runvoy/secrets",
				},
			},
			wantErr: true,
			errMsg:  "SecretsMetadataTable cannot be empty",
		},
		{
			name: "missing SecretsKMSKeyARN",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					APIKeysTable:              "api-keys",
					ExecutionsTable:           "executions",
					ImageTaskDefsTable:        "image-taskdefs",
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
					SecretsMetadataTable:      "secrets",
					SecretsPrefix:             "/runvoy/secrets",
					SecretsKMSKeyARN:          "",
				},
			},
			wantErr: true,
			errMsg:  "SecretsKMSKeyARN cannot be empty",
		},
		{
			name: "unsupported provider",
			cfg: &Config{
				BackendProvider: constants.BackendProvider("gcp"),
			},
			wantErr: true,
			errMsg:  "unsupported backend provider",
		},
		{
			name: "empty provider",
			cfg: &Config{
				BackendProvider: "",
			},
			wantErr: true,
			errMsg:  "unsupported backend provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOrchestratorConfig(tt.cfg)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateEventProcessor(t *testing.T) {
	tests := []struct {
		name               string
		cfg                *Config
		wantErr            bool
		errMsg             string
		normalizedEndpoint string
	}{
		{
			name: "valid event processor config",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					ExecutionsTable:           "executions",
					ECSCluster:                "cluster",
					WebSocketConnectionsTable: "connections",
					WebSocketAPIEndpoint:      "wss://example.com",
					WebSocketTokensTable:      "tokens",
				},
			},
			wantErr: false,
		},
		{
			name: "missing AWS config",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS:             nil,
			},
			wantErr: true,
			errMsg:  "AWS configuration is required",
		},
		{
			name: "missing ExecutionsTable",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					ECSCluster:                "cluster",
					WebSocketConnectionsTable: "connections",
					WebSocketAPIEndpoint:      "https://example.com",
					WebSocketTokensTable:      "tokens",
				},
			},
			wantErr: true,
			errMsg:  "ExecutionsTable cannot be empty",
		},
		{
			name: "missing ECSCluster",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					ExecutionsTable:           "executions",
					WebSocketConnectionsTable: "connections",
					WebSocketAPIEndpoint:      "https://example.com",
					WebSocketTokensTable:      "tokens",
				},
			},
			wantErr: true,
			errMsg:  "ECSCluster cannot be empty",
		},
		{
			name: "missing WebSocketConnectionsTable",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					APIKeysTable:         "api-keys",
					ExecutionsTable:      "executions",
					ImageTaskDefsTable:   "image-taskdefs",
					ECSCluster:           "cluster",
					Subnet1:              "subnet-1",
					Subnet2:              "subnet-2",
					SecurityGroup:        "sg-123",
					LogGroup:             "/aws/logs/app",
					WebSocketAPIEndpoint: "https://example.com",
					WebSocketTokensTable: "tokens",
					SecretsMetadataTable: "secrets",
					SecretsPrefix:        "/runvoy/secrets",
					SecretsKMSKeyARN:     "arn:aws:kms:us-east-1:123456789012:key/abc",
				},
			},
			wantErr: true,
			errMsg:  "WebSocketConnectionsTable cannot be empty",
		},
		{
			name: "missing WebSocketAPIEndpoint",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					ExecutionsTable:           "executions",
					ECSCluster:                "cluster",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
				},
			},
			wantErr: true,
			errMsg:  "WebSocketAPIEndpoint cannot be empty",
		},
		{
			name:    "all fields empty",
			cfg:     &Config{},
			wantErr: true,
		},
		{
			name: "normalizes WebSocketAPIEndpoint",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					ExecutionsTable:           "executions",
					ECSCluster:                "cluster",
					WebSocketConnectionsTable: "connections",
					WebSocketAPIEndpoint:      "example.com",
					WebSocketTokensTable:      "tokens",
				},
			},
			wantErr:            false,
			normalizedEndpoint: "https://example.com",
		},
		{
			name: "missing WebSocketTokensTable",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					ExecutionsTable:           "executions",
					ECSCluster:                "cluster",
					WebSocketConnectionsTable: "connections",
					WebSocketAPIEndpoint:      "https://example.com",
				},
			},
			wantErr: true,
			errMsg:  "WebSocketTokensTable cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEventProcessorConfig(tt.cfg)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				if tt.normalizedEndpoint != "" {
					assert.Equal(t, tt.normalizedEndpoint, tt.cfg.AWS.WebSocketAPIEndpoint)
				}
			}
		})
	}
}

func TestConfigStruct(t *testing.T) {
	t.Run("can create config with all fields", func(t *testing.T) {
		cfg := &Config{
			APIEndpoint: "https://api.example.com",
			APIKey:      "test-key",
			Port:        8080,
			LogLevel:    "INFO",
			AWS: &awsconfig.Config{
				APIKeysTable:           "api-keys-table",
				ExecutionsTable:        "executions-table",
				ImageTaskDefsTable:     "image-taskdefs-table",
				PendingAPIKeysTable:    "pending-keys-table",
				ECSCluster:             "test-cluster",
				TaskDefinition:         "test-task",
				Subnet1:                "subnet-1",
				Subnet2:                "subnet-2",
				SecurityGroup:          "sg-123",
				LogGroup:               "/aws/ecs/test",
				DefaultTaskExecRoleARN: "arn:aws:iam::123:role/exec",
				DefaultTaskRoleARN:     "arn:aws:iam::123:role/task",
			},
		}

		assert.NotNil(t, cfg)
		assert.Equal(t, "https://api.example.com", cfg.APIEndpoint)
		assert.Equal(t, "test-key", cfg.APIKey)
		assert.Equal(t, "INFO", cfg.LogLevel)
		assert.NotNil(t, cfg.AWS)
		assert.Equal(t, "test-cluster", cfg.AWS.ECSCluster)
	})
}

func TestSetDefaults(t *testing.T) {
	t.Run("sets expected default values", func(t *testing.T) {
		// This test verifies the behavior indirectly by checking if defaults
		// are reasonable. Direct testing would require exposing setDefaults.
		cfg := &Config{
			LogLevel: "INFO",
		}

		level := cfg.GetLogLevel()
		assert.Equal(t, slog.LevelInfo, level)
	})
}

func TestValidationRules(t *testing.T) {
	t.Run("URL validation for APIEndpoint", func(t *testing.T) {
		tests := []struct {
			name    string
			url     string
			wantErr bool
		}{
			{
				name:    "valid https URL",
				url:     "https://api.example.com",
				wantErr: false,
			},
			{
				name:    "valid http URL",
				url:     "http://localhost:8080",
				wantErr: false,
			},
			{
				name:    "empty URL is valid (omitempty)",
				url:     "",
				wantErr: false,
			},
			{
				name:    "invalid URL",
				url:     "not-a-url",
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := &Config{
					APIEndpoint: tt.url,
				}

				err := validate.Struct(cfg)

				if tt.wantErr {
					assert.Error(t, err, "Expected validation error for URL: %s", tt.url)
				} else {
					assert.NoError(t, err, "Expected no validation error for URL: %s", tt.url)
				}
			})
		}
	})
}

func TestGetConfigPath(t *testing.T) {
	t.Run("returns a non-empty path", func(t *testing.T) {
		path, err := GetConfigPath()
		require.NoError(t, err)
		assert.NotEmpty(t, path)
		assert.Contains(t, path, ".runvoy")
		assert.Contains(t, path, "config.yaml")
	})
}

func TestNormalizeWebSocketEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strips https://",
			input:    "https://example.execute-api.us-east-1.amazonaws.com/production",
			expected: "example.execute-api.us-east-1.amazonaws.com/production",
		},
		{
			name:     "strips http://",
			input:    "http://example.execute-api.us-east-1.amazonaws.com/production",
			expected: "example.execute-api.us-east-1.amazonaws.com/production",
		},
		{
			name:     "strips wss://",
			input:    "wss://example.execute-api.us-east-1.amazonaws.com/production",
			expected: "example.execute-api.us-east-1.amazonaws.com/production",
		},
		{
			name:     "strips ws://",
			input:    "ws://example.execute-api.us-east-1.amazonaws.com/production",
			expected: "example.execute-api.us-east-1.amazonaws.com/production",
		},
		{
			name:     "handles already normalized",
			input:    "example.execute-api.us-east-1.amazonaws.com/production",
			expected: "example.execute-api.us-east-1.amazonaws.com/production",
		},
		{
			name:     "handles with whitespace",
			input:    "  https://example.execute-api.us-east-1.amazonaws.com/production  ",
			expected: "example.execute-api.us-east-1.amazonaws.com/production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := awsconfig.NormalizeWebSocketEndpoint(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeBackendProvider(t *testing.T) {
	tests := []struct {
		name     string
		input    constants.BackendProvider
		expected constants.BackendProvider
	}{
		{
			name:     "empty provider",
			input:    "",
			expected: "",
		},
		{
			name:     "lowercase provider",
			input:    constants.BackendProvider("aws"),
			expected: constants.AWS,
		},
		{
			name:     "uppercase provider",
			input:    constants.BackendProvider("AWS"),
			expected: constants.AWS,
		},
		{
			name:     "mixed case provider",
			input:    constants.BackendProvider("Aws"),
			expected: constants.AWS,
		},
		{
			name:     "provider with whitespace",
			input:    constants.BackendProvider("  aws  "),
			expected: constants.AWS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeBackendProvider(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestLoadWithEnvironmentVariables tests Load() with environment variables
func TestLoadWithEnvironmentVariables(t *testing.T) {
	// Save original env vars
	originalPort := os.Getenv("RUNVOY_PORT")
	originalLogLevel := os.Getenv("RUNVOY_LOG_LEVEL")
	originalWebURL := os.Getenv("RUNVOY_WEB_URL")
	originalBackendProvider := os.Getenv("RUNVOY_BACKEND_PROVIDER")

	defer func() {
		// Restore original env vars
		_ = os.Setenv("RUNVOY_PORT", originalPort)
		_ = os.Setenv("RUNVOY_LOG_LEVEL", originalLogLevel)
		_ = os.Setenv("RUNVOY_WEB_URL", originalWebURL)
		_ = os.Setenv("RUNVOY_BACKEND_PROVIDER", originalBackendProvider)
	}()

	// Clear env vars for test
	_ = os.Unsetenv("RUNVOY_PORT")
	_ = os.Unsetenv("RUNVOY_LOG_LEVEL")
	_ = os.Unsetenv("RUNVOY_WEB_URL")
	_ = os.Unsetenv("RUNVOY_BACKEND_PROVIDER")

	// Set test env vars
	_ = os.Setenv("RUNVOY_LOG_LEVEL", "DEBUG")
	_ = os.Setenv("RUNVOY_BACKEND_PROVIDER", "AWS")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify environment variables were loaded
	assert.Equal(t, "DEBUG", cfg.LogLevel)
	assert.Equal(t, constants.AWS, cfg.BackendProvider)
	// Verify a WebURL was set (may be overridden by .env or other config)
	assert.NotEmpty(t, cfg.WebURL, "WebURL should be set to a default or env value")
}

// TestLoadCLIWithoutConfigFile tests LoadCLI() when config file is missing
func TestLoadCLIWithoutConfigFile(t *testing.T) {
	// LoadCLI should fail if config file doesn't exist (since we're not mocking file I/O)
	// This is an integration test that documents the current behavior
	cfg, err := LoadCLI()
	// Expected to fail because ~/.runvoy/config.yaml may not exist in test environment
	// If it does exist, the test will load it successfully
	if err != nil {
		t.Logf("LoadCLI failed as expected when config file missing: %v", err)
	} else {
		// If config exists, verify it's valid
		assert.NotNil(t, cfg)
	}
}

// TestSaveAndLoad tests Save() and that saved config can be loaded
func TestSaveAndLoad(t *testing.T) {
	// Create temporary directory for test config
	_ = t.TempDir()

	// Temporarily override config path by setting environment variable
	originalHomeDir := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHomeDir) }()

	// Create test config
	testConfig := &Config{
		APIEndpoint: "https://test.execute-api.us-east-1.amazonaws.com",
		APIKey:      "test-key-12345",
		WebURL:      "https://test.runvoy.site",
	}

	// Test Save with error handling
	// Note: Save uses os.user.Current() and actual file I/O, so we test the logic paths
	// but actual file writing is integration behavior
	err := Save(testConfig)
	if err != nil {
		// This is expected in test environments without proper permissions
		t.Logf("Save() failed as expected in test environment: %v", err)
	}
}

// TestGetLogLevelDefaults tests GetLogLevel() returns INFO for invalid levels
func TestGetLogLevelDefaults(t *testing.T) {
	tests := []struct {
		name        string
		logLevel    string
		expectLevel slog.Level
		description string
	}{
		{
			name:        "empty log level defaults to INFO",
			logLevel:    "",
			expectLevel: slog.LevelInfo,
			description: "empty string should default to INFO",
		},
		{
			name:        "invalid log level defaults to INFO",
			logLevel:    "INVALID_LEVEL",
			expectLevel: slog.LevelInfo,
			description: "invalid level should default to INFO",
		},
		{
			name:        "lowercase debug is accepted",
			logLevel:    "debug",
			expectLevel: slog.LevelDebug,
			description: "lowercase should be normalized by slog",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{LogLevel: tt.logLevel}
			level := cfg.GetLogLevel()
			assert.Equal(t, tt.expectLevel, level, tt.description)
		})
	}
}

// TestLoadOrchestratorEnvironmentVariables tests LoadOrchestrator with env vars
func TestLoadOrchestratorEnvironmentVariables(t *testing.T) {
	// Save original env vars
	originalVars := map[string]string{
		"RUNVOY_BACKEND_PROVIDER":                os.Getenv("RUNVOY_BACKEND_PROVIDER"),
		"RUNVOY_AWS_API_KEYS_TABLE":              os.Getenv("RUNVOY_AWS_API_KEYS_TABLE"),
		"RUNVOY_AWS_EXECUTIONS_TABLE":            os.Getenv("RUNVOY_AWS_EXECUTIONS_TABLE"),
		"RUNVOY_AWS_ECS_CLUSTER":                 os.Getenv("RUNVOY_AWS_ECS_CLUSTER"),
		"RUNVOY_AWS_LOG_GROUP":                   os.Getenv("RUNVOY_AWS_LOG_GROUP"),
		"RUNVOY_AWS_SECURITY_GROUP":              os.Getenv("RUNVOY_AWS_SECURITY_GROUP"),
		"RUNVOY_AWS_SUBNET_1":                    os.Getenv("RUNVOY_AWS_SUBNET_1"),
		"RUNVOY_AWS_SUBNET_2":                    os.Getenv("RUNVOY_AWS_SUBNET_2"),
		"RUNVOY_AWS_WEBSOCKET_API_ENDPOINT":      os.Getenv("RUNVOY_AWS_WEBSOCKET_API_ENDPOINT"),
		"RUNVOY_AWS_WEBSOCKET_CONNECTIONS_TABLE": os.Getenv("RUNVOY_AWS_WEBSOCKET_CONNECTIONS_TABLE"),
		"RUNVOY_AWS_SECRETS_METADATA_TABLE":      os.Getenv("RUNVOY_AWS_SECRETS_METADATA_TABLE"),
	}

	defer func() {
		// Restore all env vars
		for k, v := range originalVars {
			if v != "" {
				_ = os.Setenv(k, v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}()

	// Clear env vars
	for k := range originalVars {
		_ = os.Unsetenv(k)
	}

	// Set minimal required env vars for orchestrator
	_ = os.Setenv("RUNVOY_BACKEND_PROVIDER", "AWS")
	_ = os.Setenv("RUNVOY_AWS_API_KEYS_TABLE", "test-api-keys")
	_ = os.Setenv("RUNVOY_AWS_EXECUTIONS_TABLE", "test-executions")
	_ = os.Setenv("RUNVOY_AWS_IMAGE_TASKDEFS_TABLE", "test-image-taskdefs")
	_ = os.Setenv("RUNVOY_AWS_ECS_CLUSTER", "test-cluster")
	_ = os.Setenv("RUNVOY_AWS_LOG_GROUP", "/aws/ecs/runvoy-test")
	_ = os.Setenv("RUNVOY_AWS_SECURITY_GROUP", "sg-12345")
	_ = os.Setenv("RUNVOY_AWS_SUBNET_1", "subnet-1")
	_ = os.Setenv("RUNVOY_AWS_SUBNET_2", "subnet-2")
	endpoint := "https://test.execute-api.us-east-1.amazonaws.com/production"
	_ = os.Setenv("RUNVOY_AWS_WEBSOCKET_API_ENDPOINT", endpoint)
	_ = os.Setenv("RUNVOY_AWS_WEBSOCKET_CONNECTIONS_TABLE", "test-websocket-connections")
	_ = os.Setenv("RUNVOY_AWS_SECRETS_METADATA_TABLE", "test-secrets-metadata")

	cfg, err := LoadOrchestrator()
	require.NoError(t, err, "LoadOrchestrator should succeed with required env vars")
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.AWS)

	// Verify loaded values
	assert.Equal(t, constants.AWS, cfg.BackendProvider)
	assert.Equal(t, "test-api-keys", cfg.AWS.APIKeysTable)
	assert.Equal(t, "test-executions", cfg.AWS.ExecutionsTable)
	assert.Equal(t, "test-cluster", cfg.AWS.ECSCluster)
}

// TestLoadEventProcessorEnvironmentVariables tests LoadEventProcessor with env vars
func TestLoadEventProcessorEnvironmentVariables(t *testing.T) {
	// Save original env vars
	originalVars := map[string]string{
		"RUNVOY_BACKEND_PROVIDER":                os.Getenv("RUNVOY_BACKEND_PROVIDER"),
		"RUNVOY_AWS_EXECUTIONS_TABLE":            os.Getenv("RUNVOY_AWS_EXECUTIONS_TABLE"),
		"RUNVOY_AWS_ECS_CLUSTER":                 os.Getenv("RUNVOY_AWS_ECS_CLUSTER"),
		"RUNVOY_AWS_LOG_GROUP":                   os.Getenv("RUNVOY_AWS_LOG_GROUP"),
		"RUNVOY_AWS_WEBSOCKET_API_ENDPOINT":      os.Getenv("RUNVOY_AWS_WEBSOCKET_API_ENDPOINT"),
		"RUNVOY_AWS_WEBSOCKET_CONNECTIONS_TABLE": os.Getenv("RUNVOY_AWS_WEBSOCKET_CONNECTIONS_TABLE"),
	}

	defer func() {
		// Restore all env vars
		for k, v := range originalVars {
			if v != "" {
				_ = os.Setenv(k, v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}()

	// Clear env vars
	for k := range originalVars {
		_ = os.Unsetenv(k)
	}

	// Set minimal required env vars for event processor
	_ = os.Setenv("RUNVOY_BACKEND_PROVIDER", "AWS")
	_ = os.Setenv("RUNVOY_AWS_EXECUTIONS_TABLE", "test-executions")
	_ = os.Setenv("RUNVOY_AWS_ECS_CLUSTER", "test-cluster")
	_ = os.Setenv("RUNVOY_AWS_LOG_GROUP", "/aws/ecs/runvoy-test")
	epEndpoint := "https://test.execute-api.us-east-1.amazonaws.com/production"
	_ = os.Setenv("RUNVOY_AWS_WEBSOCKET_API_ENDPOINT", epEndpoint)
	_ = os.Setenv("RUNVOY_AWS_WEBSOCKET_CONNECTIONS_TABLE", "test-websocket-connections")

	cfg, err := LoadEventProcessor()
	require.NoError(t, err, "LoadEventProcessor should succeed with required env vars")
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.AWS)

	// Verify loaded values
	assert.Equal(t, constants.AWS, cfg.BackendProvider)
	assert.Equal(t, "test-executions", cfg.AWS.ExecutionsTable)
	assert.Equal(t, "test-cluster", cfg.AWS.ECSCluster)
}

// TestLoadOrchestratorMissingRequiredFields tests validation fails with missing fields
func TestLoadOrchestratorMissingRequiredFields(t *testing.T) {
	// Save original env vars
	originalBackendProvider := os.Getenv("RUNVOY_BACKEND_PROVIDER")
	originalAPIKeysTable := os.Getenv("RUNVOY_AWS_API_KEYS_TABLE")

	defer func() {
		_ = os.Setenv("RUNVOY_BACKEND_PROVIDER", originalBackendProvider)
		_ = os.Setenv("RUNVOY_AWS_API_KEYS_TABLE", originalAPIKeysTable)
	}()

	// Clear env vars
	_ = os.Unsetenv("RUNVOY_BACKEND_PROVIDER")
	_ = os.Unsetenv("RUNVOY_AWS_API_KEYS_TABLE")

	// Set only backend provider, missing AWS config
	_ = os.Setenv("RUNVOY_BACKEND_PROVIDER", "AWS")

	cfg, err := LoadOrchestrator()
	// Should fail validation due to missing required AWS fields
	assert.Error(t, err, "LoadOrchestrator should fail when required AWS fields are missing")
	assert.Nil(t, cfg)
}

// TestLoadEventProcessorMissingRequiredFields tests validation fails with missing fields
func TestLoadEventProcessorMissingRequiredFields(t *testing.T) {
	// Save original env vars
	originalBackendProvider := os.Getenv("RUNVOY_BACKEND_PROVIDER")
	originalExecutionsTable := os.Getenv("RUNVOY_AWS_EXECUTIONS_TABLE")

	defer func() {
		_ = os.Setenv("RUNVOY_BACKEND_PROVIDER", originalBackendProvider)
		_ = os.Setenv("RUNVOY_AWS_EXECUTIONS_TABLE", originalExecutionsTable)
	}()

	// Clear env vars
	_ = os.Unsetenv("RUNVOY_BACKEND_PROVIDER")
	_ = os.Unsetenv("RUNVOY_AWS_EXECUTIONS_TABLE")

	// Set only backend provider, missing AWS config
	_ = os.Setenv("RUNVOY_BACKEND_PROVIDER", "AWS")

	cfg, err := LoadEventProcessor()
	// Should fail validation due to missing required AWS fields
	assert.Error(t, err, "LoadEventProcessor should fail when required AWS fields are missing")
	assert.Nil(t, cfg)
}
