// Package config manages configuration for the runvoy CLI and services.
// It handles reading and writing configuration files and environment variables.
package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"runvoy/internal/constants"

	"gopkg.in/yaml.v3"
)

// Config represents the global configuration structure
type Config struct {
	APIEndpoint string `yaml:"api_endpoint"`
	APIKey      string `yaml:"api_key"`
}

// Load loads the configuration from the user's home directory
func Load() (*Config, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("error getting current user: %w", err)
	}

	configFile := constants.ConfigFilePath(currentUser.HomeDir)

	if _, err := os.Stat(configFile); err != nil {
		return nil, fmt.Errorf("config file not found at %s", configFile)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

// Save saves the configuration to the user's home directory
// Overwrites the existing config file if it exists.
func Save(config *Config) error {
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("error getting current user: %w", err)
	}

	configDir := constants.ConfigDirPath(currentUser.HomeDir)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	configFilePath := filepath.Join(configDir, constants.ConfigFileName)
	if err := os.WriteFile(configFilePath, data, 0600); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
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
