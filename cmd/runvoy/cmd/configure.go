// Package cmd implements the CLI commands for the runvoy tool.
package cmd

import (
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
	existingConfig, err := config.Load()
	configExists := err == nil
	if configExists {
		output.Success("Found existing configuration")
	} else {
		existingConfig = &config.Config{}
		output.Info("Creating new configuration")
	}

	endpoint := output.Prompt("Enter API endpoint URL")

	if endpoint == "" {
		if configExists && existingConfig.APIEndpoint != "" {
			endpoint = existingConfig.APIEndpoint
			output.Info("Using existing endpoint: %s", endpoint)
		} else {
			output.Fatal("API endpoint is required")
		}
	}

	apiKey := output.Prompt("Enter API key")

	if apiKey == "" {
		if configExists && existingConfig.APIKey != "" {
			apiKey = existingConfig.APIKey
			output.Info("Using existing API key")
		} else {
			output.Fatal("API key is required")
		}
	}

	cfg := &config.Config{
		APIEndpoint: endpoint,
		APIKey:      apiKey,
	}

	if err := config.Save(cfg); err != nil {
		output.Fatal("Failed to save configuration: %v", err)
	}

	configPath, err := config.GetConfigPath()
	if err != nil {
		output.Fatal("Failed to get config path: %v", err)
	}

	output.Success("Configuration saved successfully")
	output.KeyValue("Configuration path", configPath)
	output.Info("Configuration complete!")
}
