// Package config manages configuration for the runvoy CLI and services.
// It uses Viper for unified configuration management from files and environment variables.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"runvoy/internal/constants"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

// Config represents the unified configuration structure for both CLI and services.
// It supports loading from YAML files and environment variables.
type Config struct {
	// CLI Configuration
	APIEndpoint string `mapstructure:"api_endpoint" yaml:"api_endpoint" validate:"omitempty,url"`
	APIKey      string `mapstructure:"api_key" yaml:"api_key"`
	WebURL      string `mapstructure:"web_url" yaml:"web_url" validate:"omitempty,url"`

	// Backend Service Configuration
	APIKeysTable              string                    `mapstructure:"api_keys_table"`
	BackendProvider           constants.BackendProvider `mapstructure:"backend_provider" yaml:"backend_provider"`
	ECSCluster                string                    `mapstructure:"ecs_cluster"`
	ExecutionsTable           string                    `mapstructure:"executions_table"`
	InitTimeout               time.Duration             `mapstructure:"init_timeout"`
	LogGroup                  string                    `mapstructure:"log_group"`
	LogLevel                  string                    `mapstructure:"log_level"`
	PendingAPIKeysTable       string                    `mapstructure:"pending_api_keys_table"`
	Port                      int                       `mapstructure:"port" validate:"omitempty"`
	RequestTimeout            time.Duration             `mapstructure:"request_timeout"`
	SecurityGroup             string                    `mapstructure:"security_group"`
	Subnet1                   string                    `mapstructure:"subnet_1"`
	Subnet2                   string                    `mapstructure:"subnet_2"`
	TaskDefinition            string                    `mapstructure:"task_definition"`
	TaskExecRoleARN           string                    `mapstructure:"task_exec_role_arn"`
	TaskRoleARN               string                    `mapstructure:"task_role_arn"`
	WebSocketAPIEndpoint      string                    `mapstructure:"websocket_api_endpoint"`
	WebSocketConnectionsTable string                    `mapstructure:"websocket_connections_table"`
	WebSocketTokensTable      string                    `mapstructure:"websocket_tokens_table"`
}

var validate = validator.New()

// Load loads the configuration using Viper.
// For CLI: loads from ~/.runvoy/config.yaml
// For services: loads from environment variables with RUNVOY_ prefix
// Environment variables take precedence over config file values.
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults for service configuration
	setDefaults(v)

	// Try to load config file for CLI
	if err := loadConfigFile(v); err != nil {
		// Config file not found is acceptable for services (they use env vars only)
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error loading config file: %w", err)
		}
	}

	// Bind environment variables
	v.SetEnvPrefix("RUNVOY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Manually bind all env vars for better control
	bindEnvVars(v)

	var cfg Config
	var err error
	if err = v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err = validate.Struct(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// LoadCLI loads configuration specifically for CLI usage.
// Returns an error if the config file doesn't exist.
func LoadCLI() (*Config, error) {
	v := viper.New()

	if err := loadConfigFile(v); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &cfg, nil
}

// LoadOrchestrator loads configuration for the orchestrator service.
// Loads from environment variables and validates required fields.
// This maintains parity with the Lambda orchestrator which requires all AWS resources.
func LoadOrchestrator() (*Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetEnvPrefix("RUNVOY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindEnvVars(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling orchestrator config: %w", err)
	}

	// Validate required fields (matches old caarlos0/env notEmpty tags)
	if err := validateOrchestrator(&cfg); err != nil {
		return nil, err
	}

	// Normalize WebSocket endpoint: strip protocol if present
	cfg.WebSocketAPIEndpoint = normalizeWebSocketEndpoint(cfg.WebSocketAPIEndpoint)

	return &cfg, nil
}

// LoadEventProcessor loads configuration for the event processor service.
// Loads from environment variables and validates required fields.
func LoadEventProcessor() (*Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetEnvPrefix("RUNVOY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindEnvVars(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling event processor config: %w", err)
	}

	// Validate required fields (matches old caarlos0/env notEmpty tags)
	if err := validateEventProcessor(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// MustLoadOrchestrator loads orchestrator configuration and exits on error.
// Suitable for application startup where configuration errors should be fatal.
func MustLoadOrchestrator() *Config {
	cfg, err := LoadOrchestrator()
	if err != nil {
		slog.Error("failed to load orchestrator configuration", "error", err)
		os.Exit(1)
	}
	return cfg
}

// MustLoadEventProcessor loads event processor configuration and exits on error.
// Suitable for application startup where configuration errors should be fatal.
func MustLoadEventProcessor() *Config {
	cfg, err := LoadEventProcessor()
	if err != nil {
		slog.Error("failed to load event processor configuration", "error", err)
		os.Exit(1)
	}
	return cfg
}

// Save saves the configuration to the user's home directory.
// Overwrites the existing config file if it exists.
func Save(config *Config) error {
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("error getting current user: %w", err)
	}

	configDir := constants.ConfigDirPath(currentUser.HomeDir)

	if err = os.MkdirAll(configDir, constants.ConfigDirPermissions); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	configFilePath := filepath.Join(configDir, constants.ConfigFileName)

	v := viper.New()
	v.Set("api_endpoint", config.APIEndpoint)
	v.Set("api_key", config.APIKey)
	v.Set("web_url", config.WebURL)

	if err = v.WriteConfigAs(configFilePath); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	// Set proper permissions
	if err = os.Chmod(configFilePath, constants.ConfigFilePermissions); err != nil {
		return fmt.Errorf("error setting config file permissions: %w", err)
	}

	return nil
}

// GetConfigPath returns the path to the config file
func GetConfigPath() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("error getting current user: %w", err)
	}

	configDir := constants.ConfigDirPath(currentUser.HomeDir)
	return filepath.Join(configDir, constants.ConfigFileName), nil
}

// GetLogLevel returns the slog.Level from the string configuration.
// Defaults to INFO if the level string is invalid.
func (c *Config) GetLogLevel() slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.LogLevel)); err != nil {
		return slog.LevelInfo
	}
	return level
}

// Helper functions

func setDefaults(v *viper.Viper) {
	v.SetDefault("port", "56212")
	v.SetDefault("request_timeout", 0)
	v.SetDefault("init_timeout", "10s")
	v.SetDefault("web_url", constants.DefaultWebURL)
	v.SetDefault("backend_provider", string(constants.AWS))
	// TODO: we set DEBUG for development, we should update this to use INFO
	v.SetDefault("log_level", "DEBUG")
}

func loadConfigFile(v *viper.Viper) error {
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("error getting current user: %w", err)
	}

	configDir := constants.ConfigDirPath(currentUser.HomeDir)
	configFile := filepath.Join(configDir, constants.ConfigFileName)

	v.SetConfigFile(configFile)
	v.SetConfigType("yaml")

	if readErr := v.ReadInConfig(); readErr != nil {
		return readErr
	}

	return nil
}

func bindEnvVars(v *viper.Viper) {
	// Bind all environment variables explicitly
	envVars := []string{
		"API_KEYS_TABLE",
		"BACKEND_PROVIDER",
		"DEV_SERVER_PORT",
		"ECS_CLUSTER",
		"EXECUTIONS_TABLE",
		"INIT_TIMEOUT",
		"LOG_GROUP",
		"LOG_LEVEL",
		"PENDING_API_KEYS_TABLE",
		"REQUEST_TIMEOUT",
		"SECURITY_GROUP",
		"SUBNET_1",
		"SUBNET_2",
		"TASK_DEFINITION",
		"TASK_EXEC_ROLE_ARN",
		"TASK_ROLE_ARN",
		"WEB_URL",
		"WEBSOCKET_API_ENDPOINT",
		"WEBSOCKET_CONNECTIONS_TABLE",
		"WEBSOCKET_TOKENS_TABLE",
	}

	for _, envVar := range envVars {
		// Map DEV_SERVER_PORT to port
		if envVar == "DEV_SERVER_PORT" {
			_ = v.BindEnv("port", "RUNVOY_DEV_SERVER_PORT")
		} else {
			// Convert to lowercase to match mapstructure tags (keep underscores)
			configKey := strings.ToLower(envVar)
			_ = v.BindEnv(configKey, "RUNVOY_"+envVar)
		}
	}
}

// validateOrchestrator validates required fields for orchestrator service.
func validateOrchestrator(cfg *Config) error {
	provider := normalizeBackendProvider(cfg.BackendProvider)

	switch provider {
	case constants.AWS:
		return validateAWSOrchestrator(cfg)
	default:
		return fmt.Errorf("unsupported backend provider: %s", provider)
	}
}

func validateAWSOrchestrator(cfg *Config) error {
	required := map[string]string{
		"APIKeysTable":              cfg.APIKeysTable,
		"ECSCluster":                cfg.ECSCluster,
		"ExecutionsTable":           cfg.ExecutionsTable,
		"LogGroup":                  cfg.LogGroup,
		"SecurityGroup":             cfg.SecurityGroup,
		"Subnet1":                   cfg.Subnet1,
		"Subnet2":                   cfg.Subnet2,
		"WebSocketAPIEndpoint":      cfg.WebSocketAPIEndpoint,
		"WebSocketConnectionsTable": cfg.WebSocketConnectionsTable,
		"WebSocketTokensTable":      cfg.WebSocketTokensTable,
	}

	for field, value := range required {
		if value == "" {
			return fmt.Errorf("%s cannot be empty", field)
		}
	}

	return nil
}

// validateEventProcessor validates required fields for event processor service.
func validateEventProcessor(cfg *Config) error {
	provider := normalizeBackendProvider(cfg.BackendProvider)

	switch provider {
	case constants.AWS:
		return validateAWSEventProcessor(cfg)
	default:
		return fmt.Errorf("unsupported backend provider: %s", provider)
	}
}

func validateAWSEventProcessor(cfg *Config) error {
	required := map[string]string{
		"ECSCluster":                cfg.ECSCluster,
		"ExecutionsTable":           cfg.ExecutionsTable,
		"WebSocketAPIEndpoint":      cfg.WebSocketAPIEndpoint,
		"WebSocketConnectionsTable": cfg.WebSocketConnectionsTable,
		"WebSocketTokensTable":      cfg.WebSocketTokensTable,
	}

	for field, value := range required {
		if value == "" {
			return fmt.Errorf("%s cannot be empty", field)
		}
	}

	cfg.WebSocketAPIEndpoint = "https://" + normalizeWebSocketEndpoint(cfg.WebSocketAPIEndpoint)

	return nil
}

// normalizeWebSocketEndpoint strips protocol prefixes from WebSocket endpoint URLs.
// Accepts: https://example.com, http://example.com, wss://example.com, ws://example.com, example.com
// Returns: example.com (without protocol)
func normalizeWebSocketEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "wss://")
	endpoint = strings.TrimPrefix(endpoint, "ws://")
	return endpoint
}

// normalizeBackendProvider trims whitespace and uppercases the backend provider identifier.
func normalizeBackendProvider(provider constants.BackendProvider) constants.BackendProvider {
	normalized := strings.TrimSpace(string(provider))
	if normalized == "" {
		return ""
	}
	return constants.BackendProvider(strings.ToUpper(normalized))
}
