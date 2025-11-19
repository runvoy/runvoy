package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"runvoy/internal/client/output"
	"runvoy/internal/config"
	"runvoy/internal/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/spf13/cobra"
)

const (
	defaultS3Bucket       = "runvoy-releases"
	defaultStackName      = "runvoy"
	templateFileName      = "cloudformation-backend.yaml"
	stackPollInterval     = 5 * time.Second
	stackOperationTimeout = 30 * time.Minute
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
	infraDeployCmd.Flags().StringVar(&infraDeployStackName, "stack-name", defaultStackName,
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

	// Determine template source
	templateURL, templateBody, err := resolveTemplate(infraDeployTemplate, version)
	if err != nil {
		return fmt.Errorf("failed to resolve template: %w", err)
	}

	// Parse parameters
	cfnParams, err := parseParameters(infraDeployParameters)
	if err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
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

	cfnClient := cloudformation.NewFromConfig(awsCfg)

	// Check if stack exists
	stackExists, err := checkStackExists(ctx, cfnClient, infraDeployStackName)
	if err != nil {
		return fmt.Errorf("failed to check stack status: %w", err)
	}

	// Display deployment info
	output.Infof("Deploying runvoy infrastructure")
	output.KeyValue("Stack name", infraDeployStackName)
	output.KeyValue("Version", version)
	if templateURL != "" {
		output.KeyValue("Template URL", templateURL)
	} else {
		output.KeyValue("Template", "local file")
	}
	output.KeyValue("Region", awsCfg.Region)
	output.Blank()

	var operationType string
	if stackExists {
		operationType = "UPDATE"
		output.Infof("Updating existing stack...")
		err = updateStack(ctx, cfnClient, infraDeployStackName, templateURL, templateBody, cfnParams)
	} else {
		operationType = "CREATE"
		output.Infof("Creating new stack...")
		err = createStack(ctx, cfnClient, infraDeployStackName, templateURL, templateBody, cfnParams)
	}

	if err != nil {
		// Check if it's a "no updates" error
		if strings.Contains(err.Error(), "No updates are to be performed") {
			output.Successf("Stack is already up to date")
			return nil
		}
		return fmt.Errorf("failed to %s stack: %w", strings.ToLower(operationType), err)
	}

	if !infraDeployWait {
		output.Successf("Stack %s initiated. Use AWS Console or CLI to monitor progress.", operationType)
		return nil
	}

	// Wait for stack operation to complete
	output.Infof("Waiting for stack operation to complete...")
	finalStatus, err := waitForStackOperation(ctx, cfnClient, infraDeployStackName, operationType)
	if err != nil {
		return fmt.Errorf("stack operation failed: %w", err)
	}

	output.Successf("Stack operation completed with status: %s", finalStatus)

	// Get stack outputs
	outputs, err := getStackOutputs(ctx, cfnClient, infraDeployStackName)
	if err != nil {
		output.Warningf("Failed to retrieve stack outputs: %v", err)
	} else {
		output.Blank()
		output.Infof("Stack outputs:")
		for key, value := range outputs {
			output.KeyValue(key, value)
		}
	}

	// Configure CLI if requested
	if infraDeployConfigure {
		if endpoint, ok := outputs["APIEndpoint"]; ok {
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

// resolveTemplate determines the template source and returns either a URL or body
func resolveTemplate(template, version string) (templateURL string, templateBody string, err error) {
	if template == "" {
		// Use default S3 URL
		templateURL = fmt.Sprintf("https://%s.s3.amazonaws.com/%s/%s",
			defaultS3Bucket, version, templateFileName)
		return templateURL, "", nil
	}

	// Check if it's a URL (starts with http:// or https://)
	if strings.HasPrefix(template, "http://") || strings.HasPrefix(template, "https://") {
		return template, "", nil
	}

	// Check if it's an S3 URI (starts with s3://)
	if strings.HasPrefix(template, "s3://") {
		// Convert s3://bucket/key to https://bucket.s3.amazonaws.com/key
		parts := strings.SplitN(strings.TrimPrefix(template, "s3://"), "/", 2)
		if len(parts) < 2 {
			return "", "", fmt.Errorf("invalid S3 URI: %s", template)
		}
		bucket := parts[0]
		key := parts[1]
		templateURL = fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucket, key)
		return templateURL, "", nil
	}

	// Treat as local file
	content, err := os.ReadFile(template)
	if err != nil {
		return "", "", fmt.Errorf("failed to read template file: %w", err)
	}

	return "", string(content), nil
}

// parseParameters parses KEY=VALUE parameter strings into CloudFormation parameters
func parseParameters(params []string) ([]types.Parameter, error) {
	var cfnParams []types.Parameter

	for _, param := range params {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid parameter format: %s (expected KEY=VALUE)", param)
		}

		cfnParams = append(cfnParams, types.Parameter{
			ParameterKey:   aws.String(parts[0]),
			ParameterValue: aws.String(parts[1]),
		})
	}

	return cfnParams, nil
}

// checkStackExists checks if a CloudFormation stack exists
func checkStackExists(ctx context.Context, client *cloudformation.Client, stackName string) (bool, error) {
	_, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		// Check if it's a "stack does not exist" error
		if strings.Contains(err.Error(), "does not exist") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// createStack creates a new CloudFormation stack
func createStack(ctx context.Context, client *cloudformation.Client, stackName, templateURL, templateBody string, params []types.Parameter) error {
	input := &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		Parameters:   params,
		Capabilities: []types.Capability{types.CapabilityCapabilityNamedIam},
		Tags: []types.Tag{
			{
				Key:   aws.String("ManagedBy"),
				Value: aws.String("runvoy-cli"),
			},
		},
	}

	if templateURL != "" {
		input.TemplateURL = aws.String(templateURL)
	} else {
		input.TemplateBody = aws.String(templateBody)
	}

	_, err := client.CreateStack(ctx, input)
	return err
}

// updateStack updates an existing CloudFormation stack
func updateStack(ctx context.Context, client *cloudformation.Client, stackName, templateURL, templateBody string, params []types.Parameter) error {
	input := &cloudformation.UpdateStackInput{
		StackName:    aws.String(stackName),
		Parameters:   params,
		Capabilities: []types.Capability{types.CapabilityCapabilityNamedIam},
	}

	if templateURL != "" {
		input.TemplateURL = aws.String(templateURL)
	} else {
		input.TemplateBody = aws.String(templateBody)
	}

	_, err := client.UpdateStack(ctx, input)
	return err
}

// waitForStackOperation waits for a stack create/update to complete
func waitForStackOperation(ctx context.Context, client *cloudformation.Client, stackName, operationType string) (string, error) {
	ticker := time.NewTicker(stackPollInterval)
	defer ticker.Stop()

	timeout := time.After(stackOperationTimeout)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for stack operation")
		case <-ticker.C:
			result, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
				StackName: aws.String(stackName),
			})
			if err != nil {
				return "", err
			}

			if len(result.Stacks) == 0 {
				return "", fmt.Errorf("stack not found")
			}

			status := string(result.Stacks[0].StackStatus)

			// Check for completion states
			switch result.Stacks[0].StackStatus {
			case types.StackStatusCreateComplete, types.StackStatusUpdateComplete:
				return status, nil
			case types.StackStatusCreateFailed, types.StackStatusRollbackComplete,
				types.StackStatusRollbackFailed, types.StackStatusUpdateRollbackComplete,
				types.StackStatusUpdateRollbackFailed, types.StackStatusDeleteComplete,
				types.StackStatusDeleteFailed:
				reason := ""
				if result.Stacks[0].StackStatusReason != nil {
					reason = *result.Stacks[0].StackStatusReason
				}
				return status, fmt.Errorf("stack operation failed with status %s: %s", status, reason)
			default:
				// Still in progress
				output.Infof("Status: %s", status)
			}
		}
	}
}

// getStackOutputs retrieves the outputs from a CloudFormation stack
func getStackOutputs(ctx context.Context, client *cloudformation.Client, stackName string) (map[string]string, error) {
	result, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return nil, err
	}

	if len(result.Stacks) == 0 {
		return nil, fmt.Errorf("stack not found")
	}

	outputs := make(map[string]string)
	for _, out := range result.Stacks[0].Outputs {
		if out.OutputKey != nil && out.OutputValue != nil {
			outputs[*out.OutputKey] = *out.OutputValue
		}
	}

	return outputs, nil
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
