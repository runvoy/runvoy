package infra

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

const (
	awsDefaultS3Bucket       = "runvoy-releases"
	awsTemplateFileName      = "cloudformation-backend.yaml"
	awsStackPollInterval     = 5 * time.Second
	awsStackOperationTimeout = 30 * time.Minute
)

// CloudFormationClient defines the interface for CloudFormation operations.
// This interface enables mocking for unit tests.
type CloudFormationClient interface {
	DescribeStacks(
		ctx context.Context,
		params *cloudformation.DescribeStacksInput,
		optFns ...func(*cloudformation.Options),
	) (*cloudformation.DescribeStacksOutput, error)
	CreateStack(
		ctx context.Context,
		params *cloudformation.CreateStackInput,
		optFns ...func(*cloudformation.Options),
	) (*cloudformation.CreateStackOutput, error)
	UpdateStack(
		ctx context.Context,
		params *cloudformation.UpdateStackInput,
		optFns ...func(*cloudformation.Options),
	) (*cloudformation.UpdateStackOutput, error)
}

// AWSDeployer implements Deployer for AWS CloudFormation
type AWSDeployer struct {
	client CloudFormationClient
	region string
}

// NewAWSDeployer creates a new AWS deployer with the given region.
// If region is empty, uses the AWS SDK default.
func NewAWSDeployer(ctx context.Context, region string) (*AWSDeployer, error) {
	var awsOpts []func(*awsconfig.LoadOptions) error
	if region != "" {
		awsOpts = append(awsOpts, awsconfig.WithRegion(region))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	cfnClient := cloudformation.NewFromConfig(awsCfg)

	return &AWSDeployer{
		client: cfnClient,
		region: awsCfg.Region,
	}, nil
}

// NewAWSDeployerWithClient creates a new AWS deployer with a custom client (for testing)
func NewAWSDeployerWithClient(client CloudFormationClient, region string) *AWSDeployer {
	return &AWSDeployer{
		client: client,
		region: region,
	}
}

// GetRegion returns the AWS region being used
func (d *AWSDeployer) GetRegion() string {
	return d.region
}

// Deploy deploys or updates the CloudFormation stack
func (d *AWSDeployer) Deploy(ctx context.Context, opts *DeployOptions) (*DeployResult, error) {
	// Resolve template
	templateSource, err := resolveAWSTemplate(opts.Template, opts.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve template: %w", err)
	}

	// Parse parameters to CloudFormation format
	cfnParams, err := d.parseParametersToCFN(opts.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	// Check if stack exists
	stackExists, err := d.CheckStackExists(ctx, opts.StackName)
	if err != nil {
		return nil, fmt.Errorf("failed to check stack status: %w", err)
	}

	result := &DeployResult{
		StackName: opts.StackName,
		Outputs:   make(map[string]string),
	}

	// Create or update stack
	if stackExists {
		result.OperationType = "UPDATE"
		err = d.updateStack(ctx, opts.StackName, templateSource, cfnParams)
	} else {
		result.OperationType = "CREATE"
		err = d.createStack(ctx, opts.StackName, templateSource, cfnParams)
	}

	if err != nil {
		// Check if it's a "no updates" error
		if strings.Contains(err.Error(), "No updates are to be performed") {
			result.NoChanges = true
			result.Status = "NO_CHANGES"
			return result, nil
		}
		return nil, fmt.Errorf("failed to %s stack: %w", strings.ToLower(result.OperationType), err)
	}

	if !opts.Wait {
		result.Status = "IN_PROGRESS"
		return result, nil
	}

	// Wait for stack operation to complete
	finalStatus, err := d.waitForStackOperation(ctx, opts.StackName)
	if err != nil {
		return nil, fmt.Errorf("stack operation failed: %w", err)
	}
	result.Status = finalStatus

	// Get stack outputs
	outputs, err := d.GetStackOutputs(ctx, opts.StackName)
	if err != nil {
		// Don't fail, just return without outputs
		return result, nil
	}
	result.Outputs = outputs

	return result, nil
}

// parseParametersToCFN converts string parameters to CloudFormation parameter types
func (d *AWSDeployer) parseParametersToCFN(params []string) ([]types.Parameter, error) {
	var cfnParams []types.Parameter

	for _, param := range params {
		parts := strings.SplitN(param, "=", parameterSplitParts)
		if len(parts) != parameterSplitParts {
			return nil, fmt.Errorf("invalid parameter format: %s (expected KEY=VALUE)", param)
		}

		cfnParams = append(cfnParams, types.Parameter{
			ParameterKey:   aws.String(parts[0]),
			ParameterValue: aws.String(parts[1]),
		})
	}

	return cfnParams, nil
}

// CheckStackExists checks if a CloudFormation stack exists
func (d *AWSDeployer) CheckStackExists(ctx context.Context, stackName string) (bool, error) {
	_, err := d.client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
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
func (d *AWSDeployer) createStack(
	ctx context.Context,
	stackName string,
	template *TemplateSource,
	params []types.Parameter,
) error {
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

	if template.URL != "" {
		input.TemplateURL = aws.String(template.URL)
	} else {
		input.TemplateBody = aws.String(template.Body)
	}

	_, err := d.client.CreateStack(ctx, input)
	return err
}

// updateStack updates an existing CloudFormation stack
func (d *AWSDeployer) updateStack(
	ctx context.Context,
	stackName string,
	template *TemplateSource,
	params []types.Parameter,
) error {
	input := &cloudformation.UpdateStackInput{
		StackName:    aws.String(stackName),
		Parameters:   params,
		Capabilities: []types.Capability{types.CapabilityCapabilityNamedIam},
	}

	if template.URL != "" {
		input.TemplateURL = aws.String(template.URL)
	} else {
		input.TemplateBody = aws.String(template.Body)
	}

	_, err := d.client.UpdateStack(ctx, input)
	return err
}

// waitForStackOperation waits for a stack create/update to complete
func (d *AWSDeployer) waitForStackOperation(ctx context.Context, stackName string) (string, error) {
	ticker := time.NewTicker(awsStackPollInterval)
	defer ticker.Stop()

	timeout := time.After(awsStackOperationTimeout)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for stack operation")
		case <-ticker.C:
			status, statusReason, err := d.getStackStatus(ctx, stackName)
			if err != nil {
				return "", err
			}

			// Check for completion states
			switch types.StackStatus(status) {
			case types.StackStatusCreateComplete, types.StackStatusUpdateComplete:
				return status, nil
			case types.StackStatusCreateFailed, types.StackStatusRollbackComplete,
				types.StackStatusRollbackFailed, types.StackStatusUpdateRollbackComplete,
				types.StackStatusUpdateRollbackFailed, types.StackStatusDeleteComplete,
				types.StackStatusDeleteFailed, types.StackStatusUpdateFailed:
				return status, fmt.Errorf("stack operation failed with status %s: %s", status, statusReason)
			case types.StackStatusCreateInProgress, types.StackStatusRollbackInProgress,
				types.StackStatusDeleteInProgress, types.StackStatusUpdateInProgress,
				types.StackStatusUpdateCompleteCleanupInProgress,
				types.StackStatusUpdateRollbackInProgress,
				types.StackStatusUpdateRollbackCompleteCleanupInProgress,
				types.StackStatusReviewInProgress, types.StackStatusImportInProgress,
				types.StackStatusImportComplete, types.StackStatusImportRollbackInProgress,
				types.StackStatusImportRollbackFailed, types.StackStatusImportRollbackComplete:
				// Still in progress, continue polling
			}
		}
	}
}

// getStackStatus returns the current status of a stack
func (d *AWSDeployer) getStackStatus(ctx context.Context, stackName string) (status, reason string, err error) {
	result, err := d.client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return
	}

	if len(result.Stacks) == 0 {
		err = fmt.Errorf("stack not found")
		return
	}

	status = string(result.Stacks[0].StackStatus)
	reason = ""
	if result.Stacks[0].StackStatusReason != nil {
		reason = *result.Stacks[0].StackStatusReason
	}

	return
}

// GetStackOutputs retrieves the outputs from a CloudFormation stack
func (d *AWSDeployer) GetStackOutputs(ctx context.Context, stackName string) (map[string]string, error) {
	result, err := d.client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
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
