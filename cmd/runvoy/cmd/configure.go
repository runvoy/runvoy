// Package cmd implements the CLI commands for the runvoy tool.
package cmd

import (
	"context"
	"fmt"
	"runvoy/internal/config"
	"runvoy/internal/output"

	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure local environment with API key and endpoint URL",
	Long: `Configure the local environment with your API key and endpoint URL.
This creates or updates the configuration file at ` + output.Bold("~/.runvoy/config.yaml"),
	Run: runConfigure,
}

func init() {
	rootCmd.AddCommand(configureCmd)
}

func runConfigure(_ *cobra.Command, _ []string) {
	service := NewConfigureService(NewOutputWrapper(), NewConfigSaver(), config.Load, config.GetConfigPath)
	if err := service.Configure(context.Background()); err != nil {
		output.Errorf(err.Error())
	}
}

// ConfigLoader defines an interface for loading configuration
type ConfigLoader interface {
	Load() (*config.Config, error)
}

// configLoaderWrapper wraps the global config.Load function
type configLoaderWrapper struct{}

func (c *configLoaderWrapper) Load() (*config.Config, error) {
	return config.Load()
}

// NewConfigLoader creates a new ConfigLoader that uses the global config.Load function
func NewConfigLoader() ConfigLoader {
	return &configLoaderWrapper{}
}

// ConfigPathGetter defines an interface for getting config path
type ConfigPathGetter interface {
	GetConfigPath() (string, error)
}

// configPathGetterWrapper wraps the global config.GetConfigPath function
type configPathGetterWrapper struct{}

func (c *configPathGetterWrapper) GetConfigPath() (string, error) {
	return config.GetConfigPath()
}

// NewConfigPathGetter creates a new ConfigPathGetter that uses the global config.GetConfigPath function
func NewConfigPathGetter() ConfigPathGetter {
	return &configPathGetterWrapper{}
}

// ConfigureService handles configuration logic
type ConfigureService struct {
	output           OutputInterface
	configSaver      ConfigSaver
	configLoader     ConfigLoader
	configPathGetter ConfigPathGetter
}

// NewConfigureService creates a new ConfigureService with the provided dependencies
func NewConfigureService(output OutputInterface, configSaver ConfigSaver, configLoader func() (*config.Config, error), configPathGetter func() (string, error)) *ConfigureService {
	return &ConfigureService{
		output:           output,
		configSaver:      configSaver,
		configLoader:     &configLoaderFunc{load: configLoader},
		configPathGetter: &configPathGetterFunc{getPath: configPathGetter},
	}
}

type configLoaderFunc struct {
	load func() (*config.Config, error)
}

func (c *configLoaderFunc) Load() (*config.Config, error) {
	return c.load()
}

type configPathGetterFunc struct {
	getPath func() (string, error)
}

func (c *configPathGetterFunc) GetConfigPath() (string, error) {
	return c.getPath()
}

// Configure runs the interactive configuration flow
func (s *ConfigureService) Configure(ctx context.Context) error {
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
		return fmt.Errorf("Failed to save configuration: %w", err)
	}

	configPath, err := s.configPathGetter.GetConfigPath()
	if err != nil {
		return fmt.Errorf("Failed to get config path: %w", err)
	}

	s.output.Successf("Configuration saved successfully")
	s.output.KeyValue("Configuration path", configPath)
	s.output.Infof("Configuration complete!")
	return nil
}
