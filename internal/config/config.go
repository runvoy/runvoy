// Package config manages configuration for the runvoy CLI and services.
// It uses Viper for unified configuration management from files and environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	awsconfig "runvoy/internal/config/aws"
	"runvoy/internal/constants"
	awsConstants "runvoy/internal/providers/aws/constants"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

// Config represents the unified configuration structure for both CLI and services.
// It supports loading from YAML files and environment variables.
// Provider-specific configurations are nested under their respective provider keys.
type Config struct {
	// CLI Configuration
	APIEndpoint string `mapstructure:"api_endpoint" yaml:"api_endpoint" validate:"omitempty,url"`
	APIKey      string `mapstructure:"api_key" yaml:"api_key"`
	WebURL      string `mapstructure:"web_url" yaml:"web_url" validate:"omitempty,url"`

	// Backend Service Configuration
	BackendProvider    constants.BackendProvider `mapstructure:"backend_provider" yaml:"backend_provider"`
	InitTimeout        time.Duration             `mapstructure:"init_timeout"`
	LogLevel           string                    `mapstructure:"log_level"`
	Port               int                       `mapstructure:"port" validate:"omitempty"`
	RequestTimeout     time.Duration             `mapstructure:"request_timeout"`
	CORSAllowedOrigins []string                  `mapstructure:"cors_allowed_origins" yaml:"cors_allowed_origins"`

	// Provider-specific configurations
	AWS *awsconfig.Config `mapstructure:"aws" yaml:"aws,omitempty"`
	// Future providers can be added here:
	// GCP *GCPConfig `mapstructure:"gcp" yaml:"gcp,omitempty"`
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
		// Check for both viper.ConfigFileNotFoundError and os.ErrNotExist
		// When SetConfigFile() is used, ReadInConfig() returns os.PathError with os.ErrNotExist
		var configFileNotFound viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFound) && !errors.Is(err, os.ErrNotExist) {
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

	// Handle comma-separated string slices from environment variables
	normalizeStringSlice(&cfg.CORSAllowedOrigins)

	// Apply defaults for empty values (env vars that were unset may override defaults with empty strings)
	applyDefaults(&cfg)

	// Validate configuration
	if err = validate.Struct(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Normalize backend provider
	cfg.BackendProvider = normalizeBackendProvider(cfg.BackendProvider)

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

	// Handle comma-separated string slices from environment variables
	normalizeStringSlice(&cfg.CORSAllowedOrigins)

	// Apply defaults for empty values
	applyDefaults(&cfg)

	if err := validateOrchestratorConfig(&cfg); err != nil {
		return nil, err
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

	// Handle comma-separated string slices from environment variables
	normalizeStringSlice(&cfg.CORSAllowedOrigins)

	// Apply defaults for empty values
	applyDefaults(&cfg)

	if err := validateEventProcessorConfig(&cfg); err != nil {
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
	return saveToPath(config, configFilePath)
}

// saveToPath saves the configuration to the specified file path.
// Creates the directory if it doesn't exist and sets appropriate file permissions.
func saveToPath(config *Config, configFilePath string) error {
	configDir := filepath.Dir(configFilePath)

	if err := os.MkdirAll(configDir, constants.ConfigDirPermissions); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	v := viper.New()
	v.Set("api_endpoint", config.APIEndpoint)
	v.Set("api_key", config.APIKey)
	v.Set("web_url", config.WebURL)

	if err := v.WriteConfigAs(configFilePath); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	if err := os.Chmod(configFilePath, constants.ConfigFilePermissions); err != nil {
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

// GetDefaultStackName returns the default infrastructure stack name.
// Returns the configured value or the default if not set.
func (c *Config) GetDefaultStackName() string {
	if c.AWS != nil && c.AWS.InfraDefaultStackName != "" {
		return c.AWS.InfraDefaultStackName
	}
	return awsConstants.DefaultInfraStackName
}

// GetProviderIdentifier returns the lowercase provider identifier string.
// For AWS, returns "aws".
func (c *Config) GetProviderIdentifier() string {
	return strings.ToLower(string(c.BackendProvider))
}

// Helper functions

func setDefaults(v *viper.Viper) {
	v.SetDefault("port", "56212")
	v.SetDefault("request_timeout", 0)
	v.SetDefault("init_timeout", "10s")
	v.SetDefault("web_url", constants.DefaultWebURL)
	v.SetDefault("backend_provider", string(constants.AWS))
	v.SetDefault("cors_allowed_origins", constants.DefaultCORSAllowedOrigins)
	// TODO: we set DEBUG for development, we should update this to use INFO
	v.SetDefault("log_level", "DEBUG")
}

// applyDefaults applies default values to empty fields in the config.
// This ensures that even if environment variables were unset and Viper bound them
// to empty strings, we still get the default values.
func applyDefaults(cfg *Config) {
	if cfg.WebURL == "" {
		cfg.WebURL = constants.DefaultWebURL
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "DEBUG"
	}
	if cfg.BackendProvider == "" {
		cfg.BackendProvider = constants.AWS
	}
	if cfg.Port == 0 {
		cfg.Port = 56212
	}
	if cfg.InitTimeout == 0 {
		cfg.InitTimeout = constants.DefaultContextTimeout
	}
	if len(cfg.CORSAllowedOrigins) == 0 {
		cfg.CORSAllowedOrigins = constants.DefaultCORSAllowedOrigins
	}
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
	_ = v.BindEnv("port", "RUNVOY_DEV_SERVER_PORT", "RUNVOY_PORT")
	_ = v.BindEnv("backend_provider", "RUNVOY_BACKEND_PROVIDER")
	_ = v.BindEnv("init_timeout", "RUNVOY_INIT_TIMEOUT")
	_ = v.BindEnv("log_level", "RUNVOY_LOG_LEVEL")
	_ = v.BindEnv("request_timeout", "RUNVOY_REQUEST_TIMEOUT")
	_ = v.BindEnv("web_url", "RUNVOY_WEB_URL")
	_ = v.BindEnv("cors_allowed_origins", "RUNVOY_CORS_ALLOWED_ORIGINS")

	// Bind provider-specific environment variables
	awsconfig.BindEnvVars(v)
}

func validateOrchestratorConfig(cfg *Config) error {
	switch cfg.BackendProvider {
	case constants.AWS:
		return awsconfig.ValidateOrchestrator(cfg.AWS)
	default:
		return fmt.Errorf("unsupported backend provider: %s", cfg.BackendProvider)
	}
}

func validateEventProcessorConfig(cfg *Config) error {
	switch cfg.BackendProvider {
	case constants.AWS:
		return awsconfig.ValidateEventProcessor(cfg.AWS)
	default:
		return fmt.Errorf("unsupported backend provider: %s", cfg.BackendProvider)
	}
}

// normalizeBackendProvider trims whitespace and uppercases the backend provider identifier.
func normalizeBackendProvider(provider constants.BackendProvider) constants.BackendProvider {
	normalized := strings.TrimSpace(string(provider))
	if normalized == "" {
		return ""
	}
	return constants.BackendProvider(strings.ToUpper(normalized))
}

// normalizeStringSlice handles comma-separated string slices from environment variables.
// If the slice has a single element containing commas, it splits it into multiple elements.
// This is needed because Viper doesn't automatically split comma-separated env vars into slices.
func normalizeStringSlice(slice *[]string) {
	if len(*slice) == 1 {
		value := strings.TrimSpace((*slice)[0])
		// Check if the value contains commas, suggesting it's a comma-separated list from env var
		if strings.Contains(value, ",") {
			parts := strings.Split(value, ",")
			result := make([]string, 0, len(parts))
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					result = append(result, trimmed)
				}
			}
			if len(result) > 0 {
				*slice = result
			}
		}
	}
}
