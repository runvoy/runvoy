// Package cmd implements the CLI commands for the runvoy tool.
package cmd

import (
	"context"
	"fmt"

	"github.com/runvoy/runvoy/internal/client/output"
	"github.com/runvoy/runvoy/internal/config"
	"github.com/runvoy/runvoy/internal/constants"

	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure local environment with API key and endpoint URL",
	Long: fmt.Sprintf(`Configure the local environment with your API key and endpoint URL.
This creates or updates the configuration file at ~/%s/%s`, constants.ConfigDirName, constants.ConfigFileName),
	Run: runConfigure,
}

func init() {
	rootCmd.AddCommand(configureCmd)
}

func runConfigure(_ *cobra.Command, _ []string) {
	service := NewConfigureService(
		NewOutputWrapper(),
		NewConfigSaver(),
		NewConfigLoader(),
		NewConfigPathGetter(),
	)
	if err := service.Configure(context.Background()); err != nil {
		output.Errorf(err.Error())
	}
}

// ConfigLoader defines an interface for loading configuration
type ConfigLoader interface {
	Load() (*config.Config, error)
}

// ConfigSaver defines an interface for saving configuration
type ConfigSaver interface {
	Save(*config.Config) error
}

// ConfigPathGetter defines an interface for retrieving the configuration path
type ConfigPathGetter interface {
	GetConfigPath() (string, error)
}

// ConfigLoaderFunc adapts a function to the ConfigLoader interface
type ConfigLoaderFunc func() (*config.Config, error)

// Load executes the underlying function to load configuration
func (f ConfigLoaderFunc) Load() (*config.Config, error) {
	return f()
}

// ConfigSaverFunc adapts a function to the ConfigSaver interface
type ConfigSaverFunc func(*config.Config) error

// Save executes the underlying function to persist configuration
func (f ConfigSaverFunc) Save(cfg *config.Config) error {
	return f(cfg)
}

// ConfigPathGetterFunc adapts a function to the ConfigPathGetter interface
type ConfigPathGetterFunc func() (string, error)

// GetConfigPath executes the underlying function to retrieve the config path
func (f ConfigPathGetterFunc) GetConfigPath() (string, error) {
	return f()
}

// NewConfigLoader creates a ConfigLoader using the global config.Load function
func NewConfigLoader() ConfigLoader {
	return ConfigLoaderFunc(config.Load)
}

// NewConfigSaver creates a ConfigSaver using the global config.Save function
func NewConfigSaver() ConfigSaver {
	return ConfigSaverFunc(config.Save)
}

// NewConfigPathGetter creates a ConfigPathGetter using the global config.GetConfigPath function
func NewConfigPathGetter() ConfigPathGetter {
	return ConfigPathGetterFunc(config.GetConfigPath)
}

// ConfigureService handles configuration logic
type ConfigureService struct {
	output           OutputInterface
	configSaver      ConfigSaver
	configLoader     ConfigLoader
	configPathGetter ConfigPathGetter
}

// NewConfigureService creates a new ConfigureService with the provided dependencies
func NewConfigureService(
	outputter OutputInterface,
	configSaver ConfigSaver,
	configLoader ConfigLoader,
	configPathGetter ConfigPathGetter,
) *ConfigureService {
	return &ConfigureService{
		output:           outputter,
		configSaver:      configSaver,
		configLoader:     configLoader,
		configPathGetter: configPathGetter,
	}
}

// Configure runs the interactive configuration flow
func (s *ConfigureService) Configure(_ context.Context) error {
	existingConfig, err := s.configLoader.Load()
	configExists := err == nil

	if configExists {
		s.output.Successf("Found existing configuration")
	} else {
		existingConfig = &config.Config{}
		s.output.Infof("Creating new configuration")
	}

	endpoint := s.output.Prompt("Enter API endpoint URL")
	if endpoint == "" {
		if configExists && existingConfig.APIEndpoint != "" {
			endpoint = existingConfig.APIEndpoint
			s.output.Infof("Using existing endpoint: %s", endpoint)
		} else {
			return fmt.Errorf("API endpoint is required")
		}
	}

	apiKey := s.output.Prompt("Enter API key")
	if apiKey == "" {
		if configExists && existingConfig.APIKey != "" {
			apiKey = existingConfig.APIKey
			s.output.Infof("Using existing API key")
		} else {
			return fmt.Errorf("API key is required")
		}
	}

	cfg := &config.Config{
		APIEndpoint: endpoint,
		APIKey:      apiKey,
	}

	if err = s.configSaver.Save(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	configPath, err := s.configPathGetter.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	s.output.Successf("Configuration saved successfully")
	s.output.KeyValue("Configuration path", configPath)
	s.output.Infof("Configuration complete!")
	return nil
}
