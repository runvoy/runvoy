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

	awsconfig "runvoy/internal/config/aws"
	"runvoy/internal/constants"

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
	BackendProvider constants.BackendProvider `mapstructure:"backend_provider" yaml:"backend_provider"`
	InitTimeout     time.Duration             `mapstructure:"init_timeout"`
	LogLevel        string                    `mapstructure:"log_level"`
	Port            int                       `mapstructure:"port" validate:"omitempty"`
	RequestTimeout  time.Duration             `mapstructure:"request_timeout"`

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

	normalizeLogLevel(&cfg)

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

	normalizeLogLevel(&cfg)

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

	normalizeLogLevel(&cfg)

	if err := validateBackendProvider(&cfg, awsconfig.ValidateOrchestrator); err != nil {
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

	normalizeLogLevel(&cfg)

	if err := validateBackendProvider(&cfg, awsconfig.ValidateEventProcessor); err != nil {
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
	_ = v.BindEnv("port", "RUNVOY_DEV_SERVER_PORT", "RUNVOY_PORT")
	_ = v.BindEnv("backend_provider", "RUNVOY_BACKEND_PROVIDER")
	_ = v.BindEnv("init_timeout", "RUNVOY_INIT_TIMEOUT")
	_ = v.BindEnv("log_level", "RUNVOY_LOG_LEVEL")
	_ = v.BindEnv("request_timeout", "RUNVOY_REQUEST_TIMEOUT")
	_ = v.BindEnv("web_url", "RUNVOY_WEB_URL")

	// Bind provider-specific environment variables
	awsconfig.BindEnvVars(v)
}

// validateBackendProvider validates required fields for a backend provider.
func validateBackendProvider(cfg *Config, validateFn func(*awsconfig.Config) error) error {
	provider := normalizeBackendProvider(cfg.BackendProvider)

	switch provider {
	case constants.AWS:
		return validateFn(cfg.AWS)
	default:
		return fmt.Errorf("unsupported backend provider: %s", provider)
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

func normalizeLogLevel(cfg *Config) {
	val := strings.TrimSpace(cfg.LogLevel)
	if val == "" {
		cfg.LogLevel = slog.LevelInfo.String()
		return
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(val)); err != nil {
		cfg.LogLevel = slog.LevelInfo.String()
		return
	}

	cfg.LogLevel = level.String()
}
