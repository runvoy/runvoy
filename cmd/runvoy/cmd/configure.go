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
		output.Successf("Found existing configuration")
	} else {
		existingConfig = &config.Config{}
		output.Infof("Creating new configuration")
	}

	endpoint := output.Prompt("Enter API endpoint URL")
	if endpoint == "" {
		if configExists && existingConfig.APIEndpoint != "" {
			endpoint = existingConfig.APIEndpoint
			output.Infof("Using existing endpoint: %s", endpoint)
		} else {
			output.Fatalf("API endpoint is required")
		}
	}

	apiKey := output.Prompt("Enter API key")
	if apiKey == "" {
		if configExists && existingConfig.APIKey != "" {
			apiKey = existingConfig.APIKey
			output.Infof("Using existing API key")
		} else {
			output.Fatalf("API key is required")
		}
	}

	cfg := &config.Config{
		APIEndpoint: endpoint,
		APIKey:      apiKey,
	}

	if err = config.Save(cfg); err != nil {
		output.Fatalf("Failed to save configuration: %v", err)
	}

	configPath, err := config.GetConfigPath()
	if err != nil {
		output.Fatalf("Failed to get config path: %v", err)
	}

	output.Successf("Configuration saved successfully")
	output.KeyValue("Configuration path", configPath)
	output.Infof("Configuration complete!")
}
