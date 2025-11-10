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

// AWSConfig holds AWS-specific configuration for backend services.
// It includes settings for DynamoDB tables, ECS/Fargate resources, and WebSocket endpoints.
type AWSConfig struct {
	// DynamoDB Tables
	APIKeysTable              string `mapstructure:"api_keys_table"`
	ExecutionsTable           string `mapstructure:"executions_table"`
	PendingAPIKeysTable       string `mapstructure:"pending_api_keys_table"`
	WebSocketConnectionsTable string `mapstructure:"websocket_connections_table"`
	WebSocketTokensTable      string `mapstructure:"websocket_tokens_table"`

	// ECS/Fargate Configuration
	ECSCluster      string `mapstructure:"ecs_cluster"`
	TaskDefinition  string `mapstructure:"task_definition"`
	Subnet1         string `mapstructure:"subnet_1"`
	Subnet2         string `mapstructure:"subnet_2"`
	SecurityGroup   string `mapstructure:"security_group"`
	LogGroup        string `mapstructure:"log_group"`
	TaskExecRoleARN string `mapstructure:"task_exec_role_arn"`
	TaskRoleARN     string `mapstructure:"task_role_arn"`

	// WebSocket Configuration
	WebSocketAPIEndpoint string `mapstructure:"websocket_api_endpoint"`
}

// GCPConfig holds GCP-specific configuration for backend services.
// This is a placeholder for future GCP support.
type GCPConfig struct {
	// Placeholder for GCP-specific configuration
	// Example fields:
	// ProjectID string `mapstructure:"project_id"`
	// Region    string `mapstructure:"region"`
}

// Config represents the unified configuration structure for both CLI and services.
// It supports loading from YAML files and environment variables.
// Provider-specific configuration is nested under the AWS or GCP fields.
type Config struct {
	// CLI Configuration
	APIEndpoint string `mapstructure:"api_endpoint" yaml:"api_endpoint" validate:"omitempty,url"`
	APIKey      string `mapstructure:"api_key" yaml:"api_key"`
	WebURL      string `mapstructure:"web_url" yaml:"web_url" validate:"omitempty,url"`

	// Common Backend Service Configuration
	BackendProvider constants.BackendProvider `mapstructure:"backend_provider" yaml:"backend_provider"`
	InitTimeout     time.Duration             `mapstructure:"init_timeout"`
	LogLevel        string                    `mapstructure:"log_level"`
	Port            int                       `mapstructure:"port" validate:"omitempty"`
	RequestTimeout  time.Duration             `mapstructure:"request_timeout"`

	// Provider-specific Configuration
	// Only the configuration for the active BackendProvider should be populated
	AWS *AWSConfig `mapstructure:"aws"`
	GCP *GCPConfig `mapstructure:"gcp"`
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

	// Initialize provider-specific configs if needed
	if err = initializeProviderConfig(&cfg, v); err != nil {
		return nil, fmt.Errorf("error initializing provider config: %w", err)
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

	// Initialize provider-specific configs
	if err := initializeProviderConfig(&cfg, v); err != nil {
		return nil, fmt.Errorf("error initializing provider config: %w", err)
	}

	// Validate required fields (matches old caarlos0/env notEmpty tags)
	if err := validateOrchestrator(&cfg); err != nil {
		return nil, err
	}

	// Normalize WebSocket endpoint: strip protocol if present
	if cfg.AWS != nil {
		cfg.AWS.WebSocketAPIEndpoint = normalizeWebSocketEndpoint(cfg.AWS.WebSocketAPIEndpoint)
	}

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

	// Initialize provider-specific configs
	if err := initializeProviderConfig(&cfg, v); err != nil {
		return nil, fmt.Errorf("error initializing provider config: %w", err)
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
	// Bind common environment variables
	commonEnvVars := []string{
		"BACKEND_PROVIDER",
		"DEV_SERVER_PORT",
		"INIT_TIMEOUT",
		"LOG_LEVEL",
		"REQUEST_TIMEOUT",
		"WEB_URL",
	}

	for _, envVar := range commonEnvVars {
		// Map DEV_SERVER_PORT to port
		if envVar == "DEV_SERVER_PORT" {
			_ = v.BindEnv("port", "RUNVOY_DEV_SERVER_PORT")
		} else {
			// Convert to lowercase to match mapstructure tags (keep underscores)
			configKey := strings.ToLower(envVar)
			_ = v.BindEnv(configKey, "RUNVOY_"+envVar)
		}
	}

	// Bind AWS-specific environment variables with nested path
	awsEnvVars := []string{
		"API_KEYS_TABLE",
		"ECS_CLUSTER",
		"EXECUTIONS_TABLE",
		"LOG_GROUP",
		"PENDING_API_KEYS_TABLE",
		"SECURITY_GROUP",
		"SUBNET_1",
		"SUBNET_2",
		"TASK_DEFINITION",
		"TASK_EXEC_ROLE_ARN",
		"TASK_ROLE_ARN",
		"WEBSOCKET_API_ENDPOINT",
		"WEBSOCKET_CONNECTIONS_TABLE",
		"WEBSOCKET_TOKENS_TABLE",
	}

	for _, envVar := range awsEnvVars {
		// Convert to lowercase and nest under aws.
		configKey := "aws." + strings.ToLower(envVar)
		_ = v.BindEnv(configKey, "RUNVOY_"+envVar)
	}
}

// initializeProviderConfig initializes provider-specific configuration based on the backend provider.
// This ensures that only the relevant provider config is populated and properly structured.
func initializeProviderConfig(cfg *Config, v *viper.Viper) error {
	provider := normalizeBackendProvider(cfg.BackendProvider)

	switch provider {
	case constants.AWS:
		// Initialize AWS config if not already present
		if cfg.AWS == nil {
			cfg.AWS = &AWSConfig{}
		}
		// Unmarshal AWS-specific config
		if err := v.UnmarshalKey("aws", cfg.AWS); err != nil {
			return fmt.Errorf("error unmarshaling AWS config: %w", err)
		}
	case constants.BackendProvider("GCP"):
		// Future: Initialize GCP config
		if cfg.GCP == nil {
			cfg.GCP = &GCPConfig{}
		}
		if err := v.UnmarshalKey("gcp", cfg.GCP); err != nil {
			return fmt.Errorf("error unmarshaling GCP config: %w", err)
		}
	case "":
		// No provider specified - this is ok for CLI-only usage
		return nil
	default:
		return fmt.Errorf("unsupported backend provider: %s", provider)
	}

	return nil
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
	if cfg.AWS == nil {
		return fmt.Errorf("AWS configuration is required for AWS backend provider")
	}

	required := map[string]string{
		"APIKeysTable":              cfg.AWS.APIKeysTable,
		"ECSCluster":                cfg.AWS.ECSCluster,
		"ExecutionsTable":           cfg.AWS.ExecutionsTable,
		"LogGroup":                  cfg.AWS.LogGroup,
		"SecurityGroup":             cfg.AWS.SecurityGroup,
		"Subnet1":                   cfg.AWS.Subnet1,
		"Subnet2":                   cfg.AWS.Subnet2,
		"WebSocketAPIEndpoint":      cfg.AWS.WebSocketAPIEndpoint,
		"WebSocketConnectionsTable": cfg.AWS.WebSocketConnectionsTable,
		"WebSocketTokensTable":      cfg.AWS.WebSocketTokensTable,
	}

	for field, value := range required {
		if value == "" {
			return fmt.Errorf("AWS.%s cannot be empty", field)
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
	if cfg.AWS == nil {
		return fmt.Errorf("AWS configuration is required for AWS backend provider")
	}

	required := map[string]string{
		"ECSCluster":                cfg.AWS.ECSCluster,
		"ExecutionsTable":           cfg.AWS.ExecutionsTable,
		"WebSocketAPIEndpoint":      cfg.AWS.WebSocketAPIEndpoint,
		"WebSocketConnectionsTable": cfg.AWS.WebSocketConnectionsTable,
		"WebSocketTokensTable":      cfg.AWS.WebSocketTokensTable,
	}

	for field, value := range required {
		if value == "" {
			return fmt.Errorf("AWS.%s cannot be empty", field)
		}
	}

	cfg.AWS.WebSocketAPIEndpoint = "https://" + normalizeWebSocketEndpoint(cfg.AWS.WebSocketAPIEndpoint)

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
