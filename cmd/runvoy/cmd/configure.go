package cmd

import (
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/output"

	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure CLI with API key and endpoint URL",
	Long: `Configure the CLI with your API key and endpoint URL.
This creates or updates the configuration file at ~/.runvoy/config.yaml`,
	Run: runConfigure,
}

func init() {
	rootCmd.AddCommand(configureCmd)
}

func runConfigure(cmd *cobra.Command, args []string) {
	output.Header("🚀 " + constants.ProjectName)
	output.Subheader("Configure " + constants.ProjectName)

	// Check if config already exists
	existingConfig, err := config.Load()
	configExists := err == nil
	if configExists {
		output.Success("Found existing configuration")
	} else {
		// Create a new config if it doesn't exist
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

	// Create config structure
	cfg := &config.Config{
		APIEndpoint: endpoint,
		APIKey:      apiKey,
	}

	// Save configuration
	if err := config.Save(cfg); err != nil {
		output.Fatal("Failed to save configuration: %v", err)
	}

	// Get config path for display
	configPath, err := config.GetConfigPath()
	if err != nil {
		output.Fatal("Failed to get config path: %v", err)
	}

	output.Success("Configuration saved successfully")
	output.KeyValue("Configuration path", configPath)
	output.Info("Configuration complete!")
}
