package cmd

import (
	"context"
	"fmt"

	"runvoy/internal/client/infra"
	"runvoy/internal/client/output"
	"runvoy/internal/config"
	"runvoy/internal/constants"

	"github.com/spf13/cobra"
)

var (
	// infra apply flags
	infraApplyStackName     string
	infraApplyTemplate      string
	infraApplyVersion       string
	infraApplyParameters    []string
	infraApplyWait          bool
	infraApplyConfigure     bool
	infraApplyRegion        string
	infraApplyProvider      string
	infraApplySeedAdminUser string

	// infra destroy flags
	infraDestroyStackName string
	infraDestroyWait      bool
	infraDestroyRegion    string
	infraDestroyProvider  string
)

// infraCmd is the parent command for infrastructure operations
var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Infrastructure management commands",
	Long:  "Commands for applying and managing backend infrastructure.",
}

// infraApplyCmd applies the runvoy backend using CloudFormation
var infraApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply backend infrastructure",
	Long: `Apply or update the backend infrastructure.

By default, this command uses the official template from the releases bucket
for the current CLI version. You can override this with a custom template URL
or a local file path.`,
	Example: fmt.Sprintf(
		"  # Apply using default template and version\n"+
			"  %s infra apply --stack-name my-stack\n\n"+
			"  # Apply a specific version\n"+
			"  %s infra apply --stack-name my-stack --version 1.2.3\n\n"+
			"  # Apply with custom template from S3\n"+
			"  %s infra apply --stack-name my-stack --template https://my-bucket.s3.amazonaws.com/template.yaml\n\n"+
			"  # Apply with local template file\n"+
			"  %s infra apply --stack-name my-stack --template ./my-template.yaml\n\n"+
			"  # Apply with custom parameters\n"+
			"  %s infra apply --stack-name my-stack --parameter ProjectName=myproject "+
			"--parameter LambdaCodeBucket=my-bucket\n\n"+
			"  # Apply and automatically configure CLI\n"+
			"  %s infra apply --stack-name my-stack --configure\n\n"+
			"  # Apply, configure CLI, and seed admin user\n"+
			"  %s infra apply --stack-name my-stack --configure --seed-admin-user admin@example.com",
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
	),
	Run: infraApplyRun,
}

// infraDestroyCmd destroys the runvoy backend infrastructure
var infraDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy backend infrastructure",
	Long: `Destroy the backend infrastructure stack.

This command will delete all resources created by the apply command, including
the CloudFormation stack and all associated AWS resources.`,
	Example: fmt.Sprintf(
		"  # Destroy infrastructure stack\n"+
			"  %s infra destroy --stack-name my-stack\n\n"+
			"  # Destroy without waiting for completion\n"+
			"  %s infra destroy --stack-name my-stack --wait=false",
		constants.ProjectName,
		constants.ProjectName,
	),
	Run: infraDestroyRun,
}

func init() {
	rootCmd.AddCommand(infraCmd)
	infraCmd.AddCommand(infraApplyCmd)
	infraCmd.AddCommand(infraDestroyCmd)

	cfg, err := config.Load()
	if err != nil {
		output.Fatalf("failed to load config: %v", err)
	}

	defaultStackName := cfg.GetDefaultStackName()
	defaultProvider := cfg.GetProviderIdentifier()

	// Define flags for infra apply
	infraApplyCmd.Flags().StringVar(&infraApplyProvider, "provider", defaultProvider,
		"Cloud provider (currently supported: aws)")
	infraApplyCmd.Flags().StringVar(&infraApplyStackName, "stack-name", defaultStackName,
		"Infrastructure stack name")
	infraApplyCmd.Flags().StringVar(&infraApplyTemplate, "template", "",
		"Template URL or local file path. If not specified, uses the official template")
	infraApplyCmd.Flags().StringVar(&infraApplyVersion, "version", "",
		"Release version to apply. Defaults to CLI version")
	infraApplyCmd.Flags().StringSliceVar(&infraApplyParameters, "parameter", []string{},
		"Stack parameter in KEY=VALUE format (can be specified multiple times)")
	infraApplyCmd.Flags().BoolVar(&infraApplyWait, "wait", true,
		"Wait for stack operation to complete")
	infraApplyCmd.Flags().BoolVar(&infraApplyConfigure, "configure", false,
		"Automatically configure CLI with the applied endpoint after successful application")
	infraApplyCmd.Flags().StringVar(&infraApplyRegion, "region", "",
		"Provider region. Uses provider default if not specified")
	infraApplyCmd.Flags().StringVar(&infraApplySeedAdminUser, "seed-admin-user", "",
		"Email address for the admin user to seed into DynamoDB after successful deployment")

	// Define flags for infra destroy
	infraDestroyCmd.Flags().StringVar(&infraDestroyProvider, "provider", defaultProvider,
		"Cloud provider (currently supported: aws)")
	infraDestroyCmd.Flags().StringVar(&infraDestroyStackName, "stack-name", defaultStackName,
		"Infrastructure stack name")
	infraDestroyCmd.Flags().BoolVar(&infraDestroyWait, "wait", true,
		"Wait for stack deletion to complete")
	infraDestroyCmd.Flags().StringVar(&infraDestroyRegion, "region", "",
		"Provider region. Uses provider default if not specified")
}

func infraApplyRun(cmd *cobra.Command, _ []string) {
	version := infraApplyVersion
	if version == "" {
		version = *constants.GetVersion()
	}

	applier, err := infra.NewDeployer(cmd.Context(), infraApplyProvider, infraApplyRegion)
	if err != nil {
		output.Fatalf("failed to initialize applier: %v", err)
	}

	templateSource, err := infra.ResolveTemplate(infraApplyProvider, infraApplyTemplate, version, applier.GetRegion())
	if err != nil {
		output.Fatalf("failed to resolve template: %v", err)
	}

	output.Infof("Applying infrastructure changes")
	output.KeyValue("Provider", infraApplyProvider)
	output.KeyValue("Stack name", infraApplyStackName)
	output.KeyValue("Version", version)
	if templateSource.URL != "" {
		output.KeyValue("Template URL", templateSource.URL)
	} else {
		output.KeyValue("Template", "local file")
	}
	output.KeyValue("Region", applier.GetRegion())
	output.Blank()

	opts := &infra.DeployOptions{
		StackName:  infraApplyStackName,
		Template:   infraApplyTemplate,
		Version:    version,
		Parameters: infraApplyParameters,
		Wait:       infraApplyWait,
		Region:     infraApplyRegion,
	}

	stackExists, err := applier.CheckStackExists(cmd.Context(), infraApplyStackName)
	if err != nil {
		output.Fatalf("failed to check stack status: %v", err)
	}

	msg := "Creating new stack..."
	if stackExists {
		msg = "Updating existing stack..."
	}
	spinner := output.NewSpinner(msg)
	spinner.Start()

	result, err := applier.Deploy(cmd.Context(), opts)
	if err != nil {
		spinner.Error("Failed to apply stack")
		output.Fatalf(err.Error())
	}

	handleApplyResult(
		result,
		spinner,
		infraApplyConfigure, infraApplySeedAdminUser,
		infraApplyRegion,
	)
}

// handleApplyResult handles the result of an application operation
func handleApplyResult(
	result *infra.DeployResult,
	spinner *output.Spinner,
	configure bool,
	seedAdminUserEmail,
	region string,
) {
	if result.NoChanges {
		spinner.Success("Stack is already up to date")
		return
	}

	const stackStatusInProgress = "IN_PROGRESS"
	if result.Status == stackStatusInProgress {
		spinner.Success(
			fmt.Sprintf(
				"Stack %s initiated. Use cloud console or CLI to monitor progress.",
				result.OperationType,
			),
		)
		return
	}

	spinner.Success("Stack operation completed with status: " + result.Status)

	if len(result.Outputs) > 0 {
		output.Blank()
		output.Infof("Stack outputs:")
		for key, value := range result.Outputs {
			output.KeyValue(key, value)
		}
	}

	if configure {
		if endpoint, ok := result.Outputs["APIEndpoint"]; ok {
			configErr := configureEndpoint(endpoint)
			if configErr != nil {
				output.Warningf("Failed to configure CLI: %v", configErr)
			} else {
				output.Blank()
				output.Successf("CLI configured with API endpoint: %s", endpoint)
			}
		} else {
			output.Warningf("APIEndpoint not found in stack outputs, cannot configure CLI")
		}
	}

	if seedAdminUserEmail != "" {
		ctx := context.Background()
		if err := seedAdminUser(ctx, seedAdminUserEmail, region, result.Outputs); err != nil {
			output.Warningf("Failed to seed admin user: %v", err)
		} else {
			output.Blank()
			output.Successf("Admin user %s seeded successfully", seedAdminUserEmail)
		}
	}
}

// configureEndpoint updates the CLI configuration with the API endpoint
func configureEndpoint(endpoint string) error {
	cfg, err := config.Load()
	if err != nil {
		// Config doesn't exist yet, create a new one
		cfg = &config.Config{
			APIEndpoint: endpoint,
			APIKey:      "",
		}
	} else {
		cfg.APIEndpoint = endpoint
	}

	return config.Save(cfg)
}

// seedAdminUser seeds an admin user into the database
func seedAdminUser(ctx context.Context, adminEmail, region string, stackOutputs map[string]string) error {
	tableName, ok := stackOutputs["APIKeysTableName"]
	if !ok {
		return fmt.Errorf("APIKeysTableName not found in stack outputs")
	}

	apiKey, err := infra.SeedAdminUser(ctx, adminEmail, region, tableName)
	if err != nil {
		return err
	}

	endpoint := stackOutputs["APIEndpoint"]
	err = saveAPIKeyToConfig(apiKey, endpoint)
	if err != nil {
		return err
	}

	output.Infof("API key saved to config file")
	return nil
}

// saveAPIKeyToConfig saves the API key to the config file
// It preserves the existing endpoint if set, or uses the provided endpoint if the config doesn't have one
func saveAPIKeyToConfig(apiKey, endpoint string) error {
	cfg, err := config.Load()
	if err != nil {
		// Config doesn't exist yet, create a new one
		cfg = &config.Config{
			APIKey:      apiKey,
			APIEndpoint: endpoint,
		}
	} else {
		// Preserve existing endpoint if set, otherwise use the provided one
		cfg.APIKey = apiKey
		if cfg.APIEndpoint == "" && endpoint != "" {
			cfg.APIEndpoint = endpoint
		}
	}

	if err = config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	return nil
}

func infraDestroyRun(cmd *cobra.Command, _ []string) {
	ctx := cmd.Context()

	applier, err := infra.NewDeployer(ctx, infraDestroyProvider, infraDestroyRegion)
	if err != nil {
		output.Fatalf("failed to initialize deployer: %v", err)
	}

	output.Infof("Destroying infrastructure")
	output.KeyValue("Provider", infraDestroyProvider)
	output.KeyValue("Stack name", infraDestroyStackName)
	output.KeyValue("Region", applier.GetRegion())
	output.Blank()

	stackExists, err := applier.CheckStackExists(ctx, infraDestroyStackName)
	if err != nil {
		output.Fatalf("failed to check stack status: %v", err)
	}

	if !stackExists {
		output.Successf("Stack does not exist, nothing to destroy")
		return
	}

	opts := &infra.DestroyOptions{
		StackName: infraDestroyStackName,
		Wait:      infraDestroyWait,
		Region:    infraDestroyRegion,
	}

	spinner := output.NewSpinner("Destroying stack...")
	spinner.Start()

	result, err := applier.Destroy(ctx, opts)
	if err != nil {
		spinner.Error("Failed to destroy stack")
		output.Fatalf(err.Error())
	}

	handleDestroyResult(result, spinner)
}

// handleDestroyResult handles the result of a destroy operation
func handleDestroyResult(result *infra.DestroyResult, spinner *output.Spinner) {
	if result.NotFound {
		spinner.Success("Stack was already deleted")
		return
	}

	const stackStatusInProgress = "IN_PROGRESS"
	if result.Status == stackStatusInProgress {
		spinner.Success("Stack deletion initiated. Use cloud console or CLI to monitor progress.")
		return
	}

	if result.Status == "DELETE_COMPLETE" {
		spinner.Success("Stack successfully destroyed")
		return
	}

	spinner.Success("Stack deletion completed with status: " + result.Status)
}
