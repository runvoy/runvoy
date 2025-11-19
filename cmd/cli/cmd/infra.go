package cmd

import (
	"fmt"

	"runvoy/internal/client/infra"
	"runvoy/internal/client/output"
	"runvoy/internal/config"
	"runvoy/internal/constants"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/spf13/cobra"
)

var (
	// infra deploy flags
	infraDeployStackName  string
	infraDeployTemplate   string
	infraDeployVersion    string
	infraDeployParameters []string
	infraDeployWait       bool
	infraDeployConfigure  bool
	infraDeployRegion     string
)

// infraCmd is the parent command for infrastructure operations
var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Manage runvoy infrastructure",
	Long:  "Commands for deploying and managing runvoy backend infrastructure on AWS.",
}

// infraDeployCmd deploys the runvoy backend using CloudFormation
var infraDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy runvoy backend infrastructure",
	Long: `Deploy or update the runvoy backend infrastructure using AWS CloudFormation.

By default, this command uses the official CloudFormation template from the runvoy-releases
S3 bucket for the current CLI version. You can override this with a custom template URL
or a local file path.

Examples:
  # Deploy using default template and version
  runvoy infra deploy --stack-name my-runvoy

  # Deploy a specific version
  runvoy infra deploy --stack-name my-runvoy --version 0.3.3

  # Deploy with custom template from S3
  runvoy infra deploy --stack-name my-runvoy --template https://my-bucket.s3.amazonaws.com/template.yaml

  # Deploy with local template file
  runvoy infra deploy --stack-name my-runvoy --template ./my-template.yaml

  # Deploy with custom parameters
  runvoy infra deploy --stack-name my-runvoy --parameter ProjectName=myproject --parameter LambdaCodeBucket=my-bucket

  # Deploy and automatically configure CLI
  runvoy infra deploy --stack-name my-runvoy --configure`,
	RunE: infraDeployRun,
}

func init() {
	rootCmd.AddCommand(infraCmd)
	infraCmd.AddCommand(infraDeployCmd)

	// Define flags for infra deploy
	infraDeployCmd.Flags().StringVar(&infraDeployStackName, "stack-name", infra.DefaultStackName,
		"CloudFormation stack name")
	infraDeployCmd.Flags().StringVar(&infraDeployTemplate, "template", "",
		"Template URL (S3 HTTPS URL) or local file path. If not specified, uses the official template from runvoy-releases bucket")
	infraDeployCmd.Flags().StringVar(&infraDeployVersion, "version", "",
		"Release version to deploy. Defaults to CLI version")
	infraDeployCmd.Flags().StringSliceVar(&infraDeployParameters, "parameter", []string{},
		"CloudFormation parameter in KEY=VALUE format (can be specified multiple times)")
	infraDeployCmd.Flags().BoolVar(&infraDeployWait, "wait", true,
		"Wait for stack operation to complete")
	infraDeployCmd.Flags().BoolVar(&infraDeployConfigure, "configure", false,
		"Automatically configure CLI with the deployed endpoint after successful deployment")
	infraDeployCmd.Flags().StringVar(&infraDeployRegion, "region", "",
		"AWS region. Uses AWS SDK default if not specified")
}

func infraDeployRun(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Determine version to use
	version := infraDeployVersion
	if version == "" {
		version = *constants.GetVersion()
	}

	// Load AWS configuration
	var awsOpts []func(*awsconfig.LoadOptions) error
	if infraDeployRegion != "" {
		awsOpts = append(awsOpts, awsconfig.WithRegion(infraDeployRegion))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsOpts...)
	if err != nil {
		return fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	// Create CloudFormation client and service
	cfnClient := cloudformation.NewFromConfig(awsCfg)
	service := infra.NewDeployService(cfnClient)

	// Resolve template for display purposes
	templateSource, err := infra.ResolveTemplate(infraDeployTemplate, version)
	if err != nil {
		return fmt.Errorf("failed to resolve template: %w", err)
	}

	// Display deployment info
	output.Infof("Deploying runvoy infrastructure")
	output.KeyValue("Stack name", infraDeployStackName)
	output.KeyValue("Version", version)
	if templateSource.URL != "" {
		output.KeyValue("Template URL", templateSource.URL)
	} else {
		output.KeyValue("Template", "local file")
	}
	output.KeyValue("Region", awsCfg.Region)
	output.Blank()

	// Prepare deploy options
	opts := infra.DeployOptions{
		StackName:  infraDeployStackName,
		Template:   infraDeployTemplate,
		Version:    version,
		Parameters: infraDeployParameters,
		Wait:       infraDeployWait,
	}

	// Show operation type before starting
	stackExists, err := service.CheckStackExists(ctx, infraDeployStackName)
	if err != nil {
		return fmt.Errorf("failed to check stack status: %w", err)
	}

	if stackExists {
		output.Infof("Updating existing stack...")
	} else {
		output.Infof("Creating new stack...")
	}

	// Deploy the stack
	result, err := service.Deploy(ctx, opts)
	if err != nil {
		return err
	}

	// Handle result
	if result.NoChanges {
		output.Successf("Stack is already up to date")
		return nil
	}

	if result.Status == "IN_PROGRESS" {
		output.Successf("Stack %s initiated. Use AWS Console or CLI to monitor progress.", result.OperationType)
		return nil
	}

	output.Successf("Stack operation completed with status: %s", result.Status)

	// Display stack outputs
	if len(result.Outputs) > 0 {
		output.Blank()
		output.Infof("Stack outputs:")
		for key, value := range result.Outputs {
			output.KeyValue(key, value)
		}
	}

	// Configure CLI if requested
	if infraDeployConfigure {
		if endpoint, ok := result.Outputs["APIEndpoint"]; ok {
			if err := configureEndpoint(endpoint); err != nil {
				output.Warningf("Failed to configure CLI: %v", err)
			} else {
				output.Blank()
				output.Successf("CLI configured with API endpoint: %s", endpoint)
			}
		} else {
			output.Warningf("APIEndpoint not found in stack outputs, cannot configure CLI")
		}
	}

	return nil
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
