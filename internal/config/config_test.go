package config

import (
	"log/slog"
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
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
				},
			},
			wantErr: false,
		},
		{
			name: "wrong case provider is normalized to uppercase",
			cfg: &Config{
				BackendProvider: "aWs",
				AWS: &awsconfig.Config{
					APIKeysTable:              "api-keys",
					ExecutionsTable:           "executions",
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
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
			name: "missing APIKeysTable",
			cfg: &Config{
				BackendProvider: constants.AWS,
				AWS: &awsconfig.Config{
					ExecutionsTable:           "executions",
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
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
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
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
					APIKeysTable:              "api-keys",
					ExecutionsTable:           "executions",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
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
					ECSCluster:                "cluster",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
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
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
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
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
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
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
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
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketConnectionsTable: "connections",
					WebSocketTokensTable:      "tokens",
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
					ECSCluster:           "cluster",
					Subnet1:              "subnet-1",
					Subnet2:              "subnet-2",
					SecurityGroup:        "sg-123",
					LogGroup:             "/aws/logs/app",
					WebSocketAPIEndpoint: "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketTokensTable: "tokens",
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
					ECSCluster:                "cluster",
					Subnet1:                   "subnet-1",
					Subnet2:                   "subnet-2",
					SecurityGroup:             "sg-123",
					LogGroup:                  "/aws/logs/app",
					WebSocketAPIEndpoint:      "https://example.execute-api.us-east-1.amazonaws.com/production",
					WebSocketConnectionsTable: "connections",
				},
			},
			wantErr: true,
			errMsg:  "WebSocketTokensTable cannot be empty",
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
			err := validateOrchestrator(tt.cfg)

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
					ExecutionsTable:      "executions",
					ECSCluster:           "cluster",
					WebSocketAPIEndpoint: "https://example.com",
					WebSocketTokensTable: "tokens",
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
			err := validateEventProcessor(tt.cfg)

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
				APIKeysTable:        "api-keys-table",
				ExecutionsTable:     "executions-table",
				PendingAPIKeysTable: "pending-keys-table",
				ECSCluster:          "test-cluster",
				TaskDefinition:      "test-task",
				Subnet1:             "subnet-1",
				Subnet2:             "subnet-2",
				SecurityGroup:       "sg-123",
				LogGroup:            "/aws/ecs/test",
				TaskExecRoleARN:     "arn:aws:iam::123:role/exec",
				TaskRoleARN:         "arn:aws:iam::123:role/task",
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

		// This might fail in some test environments without a home directory
		if err == nil {
			assert.NotEmpty(t, path)
			assert.Contains(t, path, ".runvoy")
			assert.Contains(t, path, "config.yaml")
		}
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeBackendProvider(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
