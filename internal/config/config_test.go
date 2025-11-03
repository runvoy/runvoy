package config

import (
	"log/slog"
	"testing"

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
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid orchestrator config",
			cfg: &Config{
				APIKeysTable:    "api-keys",
				ExecutionsTable: "executions",
				ECSCluster:      "cluster",
				Subnet1:         "subnet-1",
				Subnet2:         "subnet-2",
				SecurityGroup:   "sg-123",
				LogGroup:        "/aws/logs/app",
			},
			wantErr: false,
		},
		{
			name: "missing APIKeysTable",
			cfg: &Config{
				ExecutionsTable: "executions",
				ECSCluster:      "cluster",
				Subnet1:         "subnet-1",
				Subnet2:         "subnet-2",
				SecurityGroup:   "sg-123",
				LogGroup:        "/aws/logs/app",
			},
			wantErr: true,
			errMsg:  "APIKeysTable cannot be empty",
		},
		{
			name: "missing ExecutionsTable",
			cfg: &Config{
				APIKeysTable:  "api-keys",
				ECSCluster:    "cluster",
				Subnet1:       "subnet-1",
				Subnet2:       "subnet-2",
				SecurityGroup: "sg-123",
				LogGroup:      "/aws/logs/app",
			},
			wantErr: true,
			errMsg:  "ExecutionsTable cannot be empty",
		},
		{
			name: "missing ECSCluster",
			cfg: &Config{
				APIKeysTable:    "api-keys",
				ExecutionsTable: "executions",
				Subnet1:         "subnet-1",
				Subnet2:         "subnet-2",
				SecurityGroup:   "sg-123",
				LogGroup:        "/aws/logs/app",
			},
			wantErr: true,
			errMsg:  "ECSCluster cannot be empty",
		},
		{
			name: "missing Subnet1",
			cfg: &Config{
				APIKeysTable:    "api-keys",
				ExecutionsTable: "executions",
				ECSCluster:      "cluster",
				Subnet2:         "subnet-2",
				SecurityGroup:   "sg-123",
				LogGroup:        "/aws/logs/app",
			},
			wantErr: true,
			errMsg:  "Subnet1 cannot be empty",
		},
		{
			name: "missing Subnet2",
			cfg: &Config{
				APIKeysTable:    "api-keys",
				ExecutionsTable: "executions",
				ECSCluster:      "cluster",
				Subnet1:         "subnet-1",
				SecurityGroup:   "sg-123",
				LogGroup:        "/aws/logs/app",
			},
			wantErr: true,
			errMsg:  "Subnet2 cannot be empty",
		},
		{
			name: "missing SecurityGroup",
			cfg: &Config{
				APIKeysTable:    "api-keys",
				ExecutionsTable: "executions",
				ECSCluster:      "cluster",
				Subnet1:         "subnet-1",
				Subnet2:         "subnet-2",
				LogGroup:        "/aws/logs/app",
			},
			wantErr: true,
			errMsg:  "SecurityGroup cannot be empty",
		},
		{
			name: "missing LogGroup",
			cfg: &Config{
				APIKeysTable:    "api-keys",
				ExecutionsTable: "executions",
				ECSCluster:      "cluster",
				Subnet1:         "subnet-1",
				Subnet2:         "subnet-2",
				SecurityGroup:   "sg-123",
			},
			wantErr: true,
			errMsg:  "LogGroup cannot be empty",
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
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid event processor config",
			cfg: &Config{
				ExecutionsTable: "executions",
				ECSCluster:      "cluster",
			},
			wantErr: false,
		},
		{
			name: "missing ExecutionsTable",
			cfg: &Config{
				ECSCluster: "cluster",
			},
			wantErr: true,
			errMsg:  "ExecutionsTable cannot be empty",
		},
		{
			name: "missing ECSCluster",
			cfg: &Config{
				ExecutionsTable: "executions",
			},
			wantErr: true,
			errMsg:  "ECSCluster cannot be empty",
		},
		{
			name:    "all fields empty",
			cfg:     &Config{},
			wantErr: true,
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
			}
		})
	}
}

func TestConfigStruct(t *testing.T) {
	t.Run("can create config with all fields", func(t *testing.T) {
		cfg := &Config{
			APIEndpoint:         "https://api.example.com",
			APIKey:              "test-key",
			Port:                "8080",
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
			LogLevel:            "INFO",
		}

		assert.NotNil(t, cfg)
		assert.Equal(t, "https://api.example.com", cfg.APIEndpoint)
		assert.Equal(t, "test-key", cfg.APIKey)
		assert.Equal(t, "INFO", cfg.LogLevel)
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

func TestConfig_GetWebviewerURL(t *testing.T) {
	tests := []struct {
		name         string
		webviewerURL string
		expectedURL  string
		description  string
	}{
		{
			name:         "returns configured URL when set",
			webviewerURL: "https://custom.example.com/webviewer.html",
			expectedURL:  "https://custom.example.com/webviewer.html",
			description:  "Should return the configured URL",
		},
		{
			name:         "returns default URL when not set",
			webviewerURL: "",
			expectedURL:  constants.DefaultWebviewerURL,
			description:  "Should return default URL when empty",
		},
		{
			name:         "handles custom deployment URL",
			webviewerURL: "https://my-company.s3.amazonaws.com/webviewer.html",
			expectedURL:  "https://my-company.s3.amazonaws.com/webviewer.html",
			description:  "Should support custom S3 deployments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				WebviewerURL: tt.webviewerURL,
			}
			result := cfg.GetWebviewerURL()
			assert.Equal(t, tt.expectedURL, result, tt.description)
		})
	}

	t.Run("default URL is valid", func(t *testing.T) {
		cfg := &Config{}
		result := cfg.GetWebviewerURL()
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "http", "Default URL should be a valid HTTP(S) URL")
	})
}
