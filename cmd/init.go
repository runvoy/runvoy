package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	internalConfig "mycli/internal/config"
	"mycli/internal/provider"

	"github.com/spf13/cobra"
)

var (
	initStackPrefix string
	initRegion      string
	forceInit       bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize mycli infrastructure in your cloud account",
	Long: `Deploys the complete mycli infrastructure to your cloud account:
- Creates infrastructure stack with all required resources
- Generates and stores a secure API key
- Optionally configures Git credentials for private repositories
- Configures the CLI automatically

This is a one-time setup command. Supports multiple cloud providers via the --provider flag.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initStackPrefix, "stack-prefix", "mycli", "CloudFormation stack name prefix (provider creates all necessary stacks)")
	initCmd.Flags().StringVar(&initRegion, "region", "us-east-2", "Cloud provider region")
	initCmd.Flags().BoolVar(&forceInit, "force", false, "Skip confirmation prompt")
}

// getProvider returns the configured provider. For now, only AWS is supported.
func getProvider() (provider.Provider, error) {
	factory := provider.NewProviderFactory()
	factory.Register("aws", provider.NewAWSProvider)

	return factory.CreateProvider("aws")
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Initializing mycli infrastructure...")
	fmt.Println("   Provider: aws")
	fmt.Printf("   Stack prefix: %s\n", initStackPrefix)
	fmt.Printf("   Region: %s\n\n", initRegion)

	p, err := getProvider()
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	// Create provider configuration
	// The provider will be responsible for creating all necessary stacks
	cfg := &provider.Config{
		StackPrefix: initStackPrefix,
		Region:      initRegion,
		Force:       forceInit,
	}

	// Validate configuration
	if err := p.ValidateConfig(cfg); err != nil {
		return err
	}

	// Confirmation prompt
	if !forceInit {
		if err := showInitConfirmation(); err != nil {
			return err
		}
	}

	// Initialize infrastructure
	outputs, err := p.InitializeInfrastructure(cmd.Context(), cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize infrastructure: %w", err)
	}

	// Save configuration
	if err := saveConfiguration(outputs); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Display success message
	displaySuccessMessage(outputs)

	return nil
}

// showInitConfirmation prompts the user for confirmation
func showInitConfirmation() error {
	fmt.Println("⚠️  This will create cloud infrastructure in your account")
	fmt.Print("   Type 'yes' to confirm: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "yes" {
		fmt.Println("Initialization cancelled.")
		os.Exit(0)
	}
	fmt.Println()
	return nil
}

// saveConfiguration saves the configuration to disk
func saveConfiguration(outputs *provider.InfrastructureOutput) error {
	fmt.Println("→ Saving configuration to disk...")
	cliConfig := &internalConfig.Config{
		APIEndpoint: outputs.APIEndpoint,
		APIKey:      outputs.APIKey,
		Region:      outputs.Region,
		StackPrefix: outputs.StackPrefix,
	}
	if err := internalConfig.Save(cliConfig); err != nil {
		return err
	}
	fmt.Println("✓ Configuration saved")
	return nil
}

// displaySuccessMessage shows the success message with configuration details
func displaySuccessMessage(outputs *provider.InfrastructureOutput) {
	fmt.Println("\n✅ Setup complete!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Configuration saved to ~/.mycli/config.yaml")
	fmt.Printf("  API Endpoint: %s\n", outputs.APIEndpoint)
	fmt.Printf("  Region:       %s\n", outputs.Region)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("\n🔑 Your API key: %s\n", outputs.APIKey)
	fmt.Println("   (Also saved to config file)")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Test it: mycli exec --repo=https://github.com/user/repo \"echo hello\"")
}
