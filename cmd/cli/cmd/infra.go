package cmd

import (
	"fmt"
	"runvoy/internal/client/infra"
	"runvoy/internal/client/output"
	"runvoy/internal/config"
	"runvoy/internal/constants"

	"github.com/spf13/cobra"
)

var (
	// infra apply flags
	infraApplyStackName  string
	infraApplyTemplate   string
	infraApplyVersion    string
	infraApplyParameters []string
	infraApplyWait       bool
	infraApplyConfigure  bool
	infraApplyRegion     string
	infraApplyProvider   string
)

// infraCmd is the parent command for infrastructure operations
var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Manage runvoy infrastructure",
	Long:  "Commands for applying and managing runvoy backend infrastructure.",
}

// infraApplyCmd applies the runvoy backend using CloudFormation
var infraApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply backend infrastructure",
	Long: fmt.Sprintf(`Apply or update the backend infrastructure.

By default, this command uses the official template from the releases bucket
for the current CLI version. You can override this with a custom template URL
or a local file path.

Examples:
  # Apply using default template and version
  %s infra apply --stack-name my-runvoy

  # Apply a specific version
  %s infra apply --stack-name my-runvoy --version 0.3.3

  # Apply with custom template from S3
  %s infra apply --stack-name my-runvoy --template https://my-bucket.s3.amazonaws.com/template.yaml

  # Apply with local template file
  %s infra apply --stack-name my-runvoy --template ./my-template.yaml

  # Apply with custom parameters
  %s infra apply --stack-name my-runvoy --parameter ProjectName=myproject --parameter LambdaCodeBucket=my-bucket

  # Apply and automatically configure CLI
  %s infra apply --stack-name my-runvoy --configure`,
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName),
	Run: infraApplyRun,
}

func init() {
	rootCmd.AddCommand(infraCmd)
	infraCmd.AddCommand(infraApplyCmd)

	cfg, err := config.Load()
	if err != nil {

		output.Fatalf("failed to load config: %v", err)
	}

	defaultStackName := cfg.GetDefaultStackName()

	// Define flags for infra apply
	infraApplyCmd.Flags().StringVar(&infraApplyProvider, "provider", infra.ProviderAWS,
		"Cloud provider (currently supported: aws)")
	infraApplyCmd.Flags().StringVar(&infraApplyStackName, "stack-name", defaultStackName,
		"Infrastructure stack name")
	infraApplyCmd.Flags().StringVar(&infraApplyTemplate, "template", "",
		"Template URL or local file path. If not specified, uses the official template from runvoy-releases")
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
}

func infraApplyRun(cmd *cobra.Command, _ []string) {
	ctx := cmd.Context()

	// Determine version to use
	version := infraApplyVersion
	if version == "" {
		version = *constants.GetVersion()
	}

	// Create applier for the specified provider
	applier, err := infra.NewDeployer(ctx, infraApplyProvider, infraApplyRegion)
	if err != nil {
		output.Fatalf("failed to initialize applier: %v", err)
	}

	// Resolve template for display purposes
	templateSource, err := infra.ResolveTemplate(infraApplyProvider, infraApplyTemplate, version)
	if err != nil {
		output.Fatalf("failed to resolve template: %v", err)
	}

	// Display application info
	output.Infof("Applying runvoy infrastructure")
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

	// Prepare apply options
	opts := &infra.DeployOptions{
		StackName:  infraApplyStackName,
		Template:   infraApplyTemplate,
		Version:    version,
		Parameters: infraApplyParameters,
		Wait:       infraApplyWait,
		Region:     infraApplyRegion,
	}

	// Show operation type before starting
	stackExists, err := applier.CheckStackExists(ctx, infraApplyStackName)
	if err != nil {
		output.Fatalf("failed to check stack status: %v", err)
	}

	if stackExists {
		output.Infof("Updating existing stack...")
	} else {
		output.Infof("Creating new stack...")
	}

	// Apply the stack
	result, err := applier.Deploy(ctx, opts)
	if err != nil {
		output.Fatalf("failed to apply stack: %v", err)
	}

	handleApplyResult(result, infraApplyConfigure)
}

// handleApplyResult handles the result of an application operation
func handleApplyResult(result *infra.DeployResult, configure bool) {
	if result.NoChanges {
		output.Successf("Stack is already up to date")
		return
	}

	if result.Status == "IN_PROGRESS" {
		output.Successf("Stack %s initiated. Use cloud console or CLI to monitor progress.", result.OperationType)
		return
	}

	output.Successf("Stack operation completed with status: %s", result.Status)

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
