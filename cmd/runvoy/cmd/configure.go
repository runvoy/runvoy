package cmd

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"runvoy/internal/config"

	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	fmt.Println("→ Configuring runvoy CLI...")

	// Check if config already exists
	existingConfig, err := config.Load()
	configExists := err == nil
	if configExists {
		fmt.Printf("✓ Found existing configuration\n")
		slog.Debug("Loaded existing configuration")
	} else {
		// Create a new config if it doesn't exist
		existingConfig = &config.Config{}
		slog.Debug("Creating new configuration")
	}

	// Prompt for API endpoint
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("\nEnter API endpoint URL: ")
	endpoint, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
	endpoint = strings.TrimSpace(endpoint)

	if endpoint == "" {
		if configExists && existingConfig.APIEndpoint != "" {
			endpoint = existingConfig.APIEndpoint
			fmt.Printf("→ Using existing endpoint: %s\n", endpoint)
		} else {
			fmt.Fprintf(os.Stderr, "Error: API endpoint is required\n")
			os.Exit(1)
		}
	}

	// Prompt for API key with masking
	fmt.Print("Enter API key: ")
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
	fmt.Println() // Add newline after masked input
	apiKey := strings.TrimSpace(string(bytePassword))

	if apiKey == "" {
		if configExists && existingConfig.APIKey != "" {
			apiKey = existingConfig.APIKey
			fmt.Printf("→ Using existing API key\n")
		} else {
			fmt.Fprintf(os.Stderr, "Error: API key is required\n")
			os.Exit(1)
		}
	}

	// Create config structure
	cfg := &config.Config{
		APIEndpoint: endpoint,
		APIKey:      apiKey,
	}

	// Save configuration
	if err := config.Save(cfg); err != nil {
		slog.Error("Failed to save configuration", "error", err)
		fmt.Fprintf(os.Stderr, "Error saving configuration: %v\n", err)
		os.Exit(1)
	}

	// Get config path for display
	configPath, err := config.GetConfigPath()
	if err != nil {
		slog.Error("Failed to get config path", "error", err)
		fmt.Fprintf(os.Stderr, "Error getting config path: %v\n", err)
		os.Exit(1)
	}

	slog.Info("Configuration saved successfully", "path", configPath)
	fmt.Printf("\n✓ Configuration saved to %s\n", configPath)
	fmt.Println("→ Configuration complete!")
}
