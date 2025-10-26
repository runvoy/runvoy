package cmd

import (
	"context"
	"fmt"

	internalConfig "mycli/internal/config"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/spf13/cobra"
)

var (
	stackName    string
	region       string
	manualAPIKey string
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure CLI manually or by discovering CloudFormation stack",
	Long: `Configures the CLI either by:
1. Discovering an existing CloudFormation stack (default)
2. Manual configuration with flags

This is useful if you deployed the infrastructure manually or need to reconfigure.`,
	RunE: runConfigure,
}

func init() {
	rootCmd.AddCommand(configureCmd)
	configureCmd.Flags().StringVar(&stackName, "stack-name", "mycli", "CloudFormation stack name")
	configureCmd.Flags().StringVar(&region, "region", "", "AWS region (default: from AWS config)")
	configureCmd.Flags().StringVar(&manualAPIKey, "api-key", "", "Manually specify API key")
}

func runConfigure(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// If API key provided manually, we need other info too
	if manualAPIKey != "" {
		fmt.Println("Manual configuration not fully implemented yet.")
		fmt.Println("Please use: mycli init")
		return fmt.Errorf("manual configuration requires --api-endpoint flag (not yet implemented)")
	}

	fmt.Printf("→ Looking for CloudFormation stack '%s'...\n", stackName)

	// Load AWS config
	cfgOpts := []func(*config.LoadOptions) error{}
	if region != "" {
		cfgOpts = append(cfgOpts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Find stack
	cfnClient := cloudformation.NewFromConfig(cfg)
	stack, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &stackName,
	})
	if err != nil {
		return fmt.Errorf("stack not found: %w (did you run 'mycli init'?)", err)
	}

	if len(stack.Stacks) == 0 {
		return fmt.Errorf("stack not found")
	}

	// Extract outputs
	outputs := parseStackOutputs(stack.Stacks[0].Outputs)

	apiEndpoint := outputs["APIEndpoint"]

	if apiEndpoint == "" {
		return fmt.Errorf("stack outputs incomplete - APIEndpoint missing")
	}

	// Note: We can't get the API key from stack (it's not stored there)
	fmt.Println("\n⚠️  Warning: API key not found in stack outputs.")
	fmt.Println("   You'll need to provide it manually or run 'mycli init' to generate a new one.")
	fmt.Print("\nEnter API key: ")
	var apiKey string
	fmt.Scanln(&apiKey)

	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}

	// Save config
	cliConfig := &internalConfig.Config{
		APIEndpoint: apiEndpoint,
		APIKey:      apiKey,
		Region:      cfg.Region,
	}
	if err := internalConfig.Save(cliConfig); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\n✓ Configuration saved to ~/.mycli/config.yaml")
	fmt.Println("\nReady to use! Try: mycli exec \"echo hello\"")

	return nil
}
