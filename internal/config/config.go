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
	APIEndpoint  string `mapstructure:"api_endpoint" yaml:"api_endpoint" validate:"omitempty,url"`
	APIKey       string `mapstructure:"api_key" yaml:"api_key"`
	WebviewerURL string `mapstructure:"webviewer_url" yaml:"webviewer_url" validate:"omitempty,url"`

	// Backend Service Configuration
	Port                      string        `mapstructure:"port" validate:"omitempty"`
	RequestTimeout            time.Duration `mapstructure:"request_timeout"`
	APIKeysTable              string        `mapstructure:"api_keys_table"`
	ExecutionsTable           string        `mapstructure:"executions_table"`
	PendingAPIKeysTable       string        `mapstructure:"pending_api_keys_table"`
	ECSCluster                string        `mapstructure:"ecs_cluster"`
	TaskDefinition            string        `mapstructure:"task_definition"`
	Subnet1                   string        `mapstructure:"subnet_1"`
	Subnet2                   string        `mapstructure:"subnet_2"`
	SecurityGroup             string        `mapstructure:"security_group"`
	LogGroup                  string        `mapstructure:"log_group"`
	TaskExecRoleARN           string        `mapstructure:"task_exec_role_arn"`
	TaskRoleARN               string        `mapstructure:"task_role_arn"`
	WebSocketConnectionsTable string        `mapstructure:"websocket_connections_table"`
	WebSocketAPIEndpoint      string        `mapstructure:"websocket_api_endpoint"`
	InitTimeout               time.Duration `mapstructure:"init_timeout"`
	LogLevel                  string        `mapstructure:"log_level"`
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

// LoadConnectionManager loads configuration for the connection manager service.
// Loads from environment variables and validates required fields.
func LoadConnectionManager() (*Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetEnvPrefix("RUNVOY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindEnvVars(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling connection manager config: %w", err)
	}

	if err := validateConnectionManager(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// MustLoadConnectionManager loads connection manager configuration and exits on error.
// Suitable for application startup where configuration errors should be fatal.
func MustLoadConnectionManager() *Config {
	cfg, err := LoadConnectionManager()
	if err != nil {
		slog.Error("failed to load connection manager configuration", "error", err)
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
	v.Set("webviewer_url", config.WebviewerURL)

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

// GetWebviewerURL returns the webviewer URL from configuration.
// Returns the configured URL if set, otherwise returns the default URL.
func (c *Config) GetWebviewerURL() string {
	if c.WebviewerURL != "" {
		return c.WebviewerURL
	}
	return constants.DefaultWebviewerURL
}

// Helper functions

func setDefaults(v *viper.Viper) {
	v.SetDefault("port", "56212")
	v.SetDefault("request_timeout", 0)
	v.SetDefault("init_timeout", "10s")
	v.SetDefault("log_level", "INFO")
	v.SetDefault("webviewer_url", constants.DefaultWebviewerURL)
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
		"WEBSOCKET_CONNECTIONS_TABLE",
		"WEBSOCKET_API_ENDPOINT",
		"WEBVIEWER_URL",
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
// These match the old caarlos0/env notEmpty tags to maintain parity.
// TaskDefinition is no longer required - task definitions are managed dynamically via API.
// WebSocketConnectionsTable is only required by the WebSocket lambdas, not the orchestrator.
func validateOrchestrator(cfg *Config) error {
	required := map[string]string{
		"APIKeysTable":         cfg.APIKeysTable,
		"ExecutionsTable":      cfg.ExecutionsTable,
		"ECSCluster":           cfg.ECSCluster,
		"Subnet1":              cfg.Subnet1,
		"Subnet2":              cfg.Subnet2,
		"SecurityGroup":        cfg.SecurityGroup,
		"LogGroup":             cfg.LogGroup,
		"WebSocketAPIEndpoint": cfg.WebSocketAPIEndpoint,
	}

	for field, value := range required {
		if value == "" {
			return fmt.Errorf("%s cannot be empty", field)
		}
	}

	return nil
}

// validateEventProcessor validates required fields for event processor service.
// These match the old caarlos0/env notEmpty tags.
func validateEventProcessor(cfg *Config) error {
	required := map[string]string{
		"ExecutionsTable": cfg.ExecutionsTable,
		"ECSCluster":      cfg.ECSCluster,
	}

	for field, value := range required {
		if value == "" {
			return fmt.Errorf("%s cannot be empty", field)
		}
	}

	return nil
}

// LoadLogForwarder loads configuration for the log forwarder service.
// Loads from environment variables and validates required fields.
func LoadLogForwarder() (*Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetEnvPrefix("RUNVOY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindEnvVars(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling log forwarder config: %w", err)
	}

	if err := validateLogForwarder(&cfg); err != nil {
		return nil, err
	}

	// Normalize WebSocket endpoint: strip protocol if present
	cfg.WebSocketAPIEndpoint = normalizeWebSocketEndpoint(cfg.WebSocketAPIEndpoint)

	return &cfg, nil
}

// MustLoadLogForwarder loads log forwarder configuration and exits on error.
// Suitable for application startup where configuration errors should be fatal.
func MustLoadLogForwarder() *Config {
	cfg, err := LoadLogForwarder()
	if err != nil {
		slog.Error("failed to load log forwarder configuration", "error", err)
		os.Exit(1)
	}
	return cfg
}

// validateConnectionManager validates required fields for connection manager service.
func validateConnectionManager(cfg *Config) error {
	required := map[string]string{
		"WebSocketConnectionsTable": cfg.WebSocketConnectionsTable,
	}

	for field, value := range required {
		if value == "" {
			return fmt.Errorf("%s cannot be empty", field)
		}
	}

	return nil
}

// validateLogForwarder validates required fields for log forwarder service.
func validateLogForwarder(cfg *Config) error {
	required := map[string]string{
		"WebSocketConnectionsTable": cfg.WebSocketConnectionsTable,
		"WebSocketApiEndpoint":      cfg.WebSocketAPIEndpoint,
	}

	for field, value := range required {
		if value == "" {
			return fmt.Errorf("%s cannot be empty", field)
		}
	}

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
