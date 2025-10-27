package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	internalConfig "runvoy/internal/config"
	"runvoy/internal/provider"

	"github.com/spf13/cobra"
)

var (
	destroyRegion string
	forceDestroy  bool
	keepConfig    bool
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy runvoy infrastructure from your cloud account",
	Long: `Removes all runvoy infrastructure from your cloud account:
- Deletes the main CloudFormation stack (Lambda, VPC, ECS, DynamoDB, etc.)
- Empties the S3 bucket containing Lambda code
- Deletes the S3 bucket stack

The stack name is automatically determined from your configuration file.

WARNING: This will permanently delete all infrastructure. This action cannot be undone.`,
	RunE: runDestroy,
}

func init() {
	rootCmd.AddCommand(destroyCmd)
	destroyCmd.Flags().StringVar(&destroyRegion, "region", "", "Cloud provider region (defaults to configured value)")
	destroyCmd.Flags().BoolVar(&forceDestroy, "force", false, "Skip confirmation prompt")
	destroyCmd.Flags().BoolVar(&keepConfig, "keep-config", false, "Keep configuration file after destroy")
}

func runDestroy(cmd *cobra.Command, args []string) error {
	// Load existing configuration to get stack name
	config, err := internalConfig.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v. Run 'runvoy init' first, or specify --region", err)
	}

	region := destroyRegion
	if region == "" {
		region = config.Region
	}
	if region == "" {
		region = "us-east-2"
	}

	fmt.Println("🗑️  Destroying runvoy infrastructure...")
	fmt.Println("   Provider:      aws")
	fmt.Printf("   Stack prefix:  %s\n", config.StackPrefix)
	fmt.Printf("   Region:        %s\n\n", region)

	p, err := getProvider()
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	// Create provider configuration
	cfg := &provider.Config{
		StackPrefix: config.StackPrefix,
		Region:      region,
		Force:       forceDestroy,
	}

	// Validate configuration
	if err := p.ValidateConfig(cfg); err != nil {
		return err
	}

	// Confirmation prompt
	if !forceDestroy {
		if err := showDestroyConfirmation(); err != nil {
			return err
		}
	}

	// Destroy infrastructure
	if err := p.DestroyInfrastructure(cmd.Context(), cfg); err != nil {
		return fmt.Errorf("failed to destroy infrastructure: %w", err)
	}

	// Remove configuration file
	if keepConfig {
		fmt.Println("✓ Configuration file preserved (--keep-config specified)")
	} else {
		if err := removeConfiguration(); err != nil {
			fmt.Printf("⚠️  Warning: failed to remove configuration: %v\n", err)
		} else {
			fmt.Println("✓ Configuration file removed")
		}
	}

	// Display success message
	displayDestroySuccessMessage()

	return nil
}

// showDestroyConfirmation prompts the user for confirmation
func showDestroyConfirmation() error {
	fmt.Println("⚠️  WARNING: This will permanently delete all runvoy infrastructure:")
	fmt.Println("   This action cannot be undone!")
	fmt.Print("   Type 'delete' to confirm: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "delete" {
		fmt.Println("Destruction cancelled.")
		os.Exit(0)
	}
	fmt.Println()
	return nil
}

// removeConfiguration removes the configuration file from disk
func removeConfiguration() error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config file doesn't exist, that's okay
		return nil
	}

	return os.Remove(configPath)
}

func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/.runvoy/config.yaml", home), nil
}

// displayDestroySuccessMessage shows the success message
func displayDestroySuccessMessage() {
	fmt.Println("\n✅ Destruction complete!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("All runvoy infrastructure has been removed from your cloud account.")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("\nTo deploy again, run:")
	fmt.Println("  runvoy init")
}
