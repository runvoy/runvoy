package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/runvoy/runvoy/internal/client/infra/core"
	awscfg "github.com/runvoy/runvoy/internal/config/aws"
	awsConstants "github.com/runvoy/runvoy/internal/providers/aws/constants"
)

// CloudFormationClient defines the interface for CloudFormation operations.
// This interface enables mocking for unit tests.
//
//nolint:dupl // Interface signature duplicated in test mock
type CloudFormationClient interface {
	DescribeStacks(
		ctx context.Context,
		params *cloudformation.DescribeStacksInput,
		optFns ...func(*cloudformation.Options),
	) (*cloudformation.DescribeStacksOutput, error)
	DescribeStackEvents(
		ctx context.Context,
		params *cloudformation.DescribeStackEventsInput,
		optFns ...func(*cloudformation.Options),
	) (*cloudformation.DescribeStackEventsOutput, error)
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
	DeleteStack(
		ctx context.Context,
		params *cloudformation.DeleteStackInput,
		optFns ...func(*cloudformation.Options),
	) (*cloudformation.DeleteStackOutput, error)
}

// AWSDeployer implements Deployer for AWS CloudFormation.
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

// NewAWSDeployerWithClient creates a new AWS deployer with a custom client (for testing).
func NewAWSDeployerWithClient(client CloudFormationClient, region string) *AWSDeployer {
	return &AWSDeployer{
		client: client,
		region: region,
	}
}

// GetRegion returns the AWS region being used.
func (d *AWSDeployer) GetRegion() string {
	return d.region
}

// validateRegionForDefaultTemplate validates the region if using the default template.
func (d *AWSDeployer) validateRegionForDefaultTemplate(template string) error {
	if template == "" {
		if err := awsConstants.ValidateRegion(d.region); err != nil {
			return fmt.Errorf("region validation failed: %w", err)
		}
	}
	return nil
}

// executeStackOperation creates or updates the stack and handles errors.
func (d *AWSDeployer) executeStackOperation(
	ctx context.Context,
	stackExists bool,
	stackName string,
	templateSource *core.TemplateSource,
	cfnParams []types.Parameter,
	result *core.DeployResult,
) error {
	if stackExists {
		result.OperationType = core.OperationTypeUpdate
		return d.updateStack(ctx, stackName, templateSource, cfnParams)
	}
	result.OperationType = core.OperationTypeCreate
	return d.createStack(ctx, stackName, templateSource, cfnParams)
}

// Deploy deploys or updates the CloudFormation stack.
func (d *AWSDeployer) Deploy(ctx context.Context, opts *core.DeployOptions) (*core.DeployResult, error) {
	if err := d.validateRegionForDefaultTemplate(opts.Template); err != nil {
		return nil, err
	}

	templateSource, err := resolveAWSTemplate(opts.Template, opts.Version, d.region)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve template: %w", err)
	}

	cfnParams, err := d.parseParametersToCFN(opts.Parameters, opts.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	stackName := opts.Name
	stackExists, err := d.checkStackExists(ctx, stackName)
	if err != nil {
		return nil, fmt.Errorf("failed to check stack status: %w", err)
	}

	result := &core.DeployResult{
		Name:    stackName,
		Outputs: make(map[string]string),
	}

	err = d.executeStackOperation(ctx, stackExists, stackName, templateSource, cfnParams, result)
	if err != nil {
		if strings.Contains(err.Error(), "No updates are to be performed") {
			result.NoChanges = true
			result.Status = "NO_CHANGES"
			return result, nil
		}
		return nil, fmt.Errorf("failed to %s stack: %w", strings.ToLower(result.OperationType), err)
	}

	if !opts.Wait {
		result.Status = core.StatusInProgress
		return result, nil
	}

	finalStatus, err := d.waitForStackOperation(ctx, stackName)
	if err != nil {
		return nil, fmt.Errorf("stack operation failed: %w", err)
	}
	result.Status = finalStatus

	outputs, err := d.getStackOutputs(ctx, stackName)
	if err != nil {
		return result, fmt.Errorf("stack deployment succeeded but failed to retrieve outputs: %w", err)
	}
	result.Outputs = outputs

	return result, nil
}

// parseParametersToCFN converts string parameters to CloudFormation parameter types.
func (d *AWSDeployer) parseParametersToCFN(params []string, version string) ([]types.Parameter, error) {
	paramMap := make(map[string]string)

	for _, param := range params {
		parts := strings.SplitN(param, "=", parameterSplitParts)
		if len(parts) != parameterSplitParts {
			return nil, fmt.Errorf("invalid parameter format: %s (expected KEY=VALUE)", param)
		}

		paramMap[parts[0]] = parts[1]
	}

	if _, exists := paramMap["LambdaCodeBucket"]; !exists {
		paramMap["LambdaCodeBucket"] = "runvoy-releases-" + d.region
	}

	if _, exists := paramMap["ReleaseVersion"]; !exists && version != "" {
		paramMap["ReleaseVersion"] = awscfg.NormalizeVersion(version)
	}

	cfnParams := make([]types.Parameter, 0, len(paramMap))
	for key, value := range paramMap {
		cfnParams = append(cfnParams, types.Parameter{
			ParameterKey:   aws.String(key),
			ParameterValue: aws.String(value),
		})
	}

	return cfnParams, nil
}

// CheckExists checks if a CloudFormation stack exists.
func (d *AWSDeployer) CheckExists(ctx context.Context, name string) (bool, error) {
	return d.checkStackExists(ctx, name)
}

// checkStackExists is the internal implementation for checking stack existence.
func (d *AWSDeployer) checkStackExists(ctx context.Context, stackName string) (bool, error) {
	_, err := d.client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return false, nil
		}
		return false, fmt.Errorf("failed to describe stacks: %w", err)
	}
	return true, nil
}

// createStack creates a new CloudFormation stack.
func (d *AWSDeployer) createStack(
	ctx context.Context,
	stackName string,
	template *core.TemplateSource,
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
	if err != nil {
		return fmt.Errorf("failed to create stack: %w", err)
	}
	return nil
}

// updateStack updates an existing CloudFormation stack.
func (d *AWSDeployer) updateStack(
	ctx context.Context,
	stackName string,
	template *core.TemplateSource,
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
	if err != nil {
		return fmt.Errorf("failed to update stack: %w", err)
	}
	return nil
}

// waitForStackOperation waits for a stack create/update to complete.
func (d *AWSDeployer) waitForStackOperation(ctx context.Context, stackName string) (string, error) {
	ticker := time.NewTicker(awsConstants.StackPollInterval)
	defer ticker.Stop()

	timeout := time.After(awsConstants.StackOperationTimeout)

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context canceled: %w", ctx.Err())
		case <-timeout:
			return "", errors.New("timeout waiting for stack operation")
		case <-ticker.C:
			status, statusReason, err := d.getStackStatus(ctx, stackName)
			if err != nil {
				return "", err
			}

			switch types.StackStatus(status) {
			case types.StackStatusCreateComplete, types.StackStatusUpdateComplete:
				return status, nil
			case types.StackStatusCreateFailed, types.StackStatusRollbackComplete,
				types.StackStatusRollbackFailed, types.StackStatusUpdateRollbackComplete,
				types.StackStatusUpdateRollbackFailed, types.StackStatusDeleteComplete,
				types.StackStatusDeleteFailed, types.StackStatusUpdateFailed:
				failureDetails := d.getFailedResourceEvents(ctx, stackName)
				if failureDetails != "" {
					return status, fmt.Errorf(
						"stack operation failed with status %s: %s\n\nResource failures:\n%s",
						status, statusReason, failureDetails)
				}
				return status, fmt.Errorf("stack operation failed with status %s: %s", status, statusReason)
			case types.StackStatusCreateInProgress, types.StackStatusRollbackInProgress,
				types.StackStatusDeleteInProgress, types.StackStatusUpdateInProgress,
				types.StackStatusUpdateCompleteCleanupInProgress,
				types.StackStatusUpdateRollbackInProgress,
				types.StackStatusUpdateRollbackCompleteCleanupInProgress,
				types.StackStatusReviewInProgress, types.StackStatusImportInProgress,
				types.StackStatusImportComplete, types.StackStatusImportRollbackInProgress,
				types.StackStatusImportRollbackFailed, types.StackStatusImportRollbackComplete:
			}
		}
	}
}

// getStackStatus returns the current status of a stack.
func (d *AWSDeployer) getStackStatus(ctx context.Context, stackName string) (status, reason string, err error) {
	result, err := d.client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return
	}

	if len(result.Stacks) == 0 {
		err = errors.New("stack not found")
		return
	}

	status = string(result.Stacks[0].StackStatus)
	reason = ""
	if result.Stacks[0].StackStatusReason != nil {
		reason = *result.Stacks[0].StackStatusReason
	}

	return
}

// getFailedResourceEvents retrieves detailed failure information from stack events.
func (d *AWSDeployer) getFailedResourceEvents(ctx context.Context, stackName string) string {
	result, err := d.client.DescribeStackEvents(ctx, &cloudformation.DescribeStackEventsInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return ""
	}

	var failures []string
	for i := range result.StackEvents {
		event := &result.StackEvents[i]
		if failureMsg := formatFailureEvent(event); failureMsg != "" {
			failures = append(failures, failureMsg)
		}
	}

	if len(failures) == 0 {
		return ""
	}

	return strings.Join(failures, "\n")
}

// formatFailureEvent formats a stack event as a failure message if it represents a failure.
func formatFailureEvent(event *types.StackEvent) string {
	status := string(event.ResourceStatus)
	isFailed := strings.Contains(status, "FAILED")
	isRollbackWithReason := strings.Contains(status, "ROLLBACK") && event.ResourceStatusReason != nil
	if !isFailed && !isRollbackWithReason {
		return ""
	}

	if event.ResourceStatusReason == nil || *event.ResourceStatusReason == "" {
		return ""
	}

	resourceID := ""
	if event.LogicalResourceId != nil {
		resourceID = *event.LogicalResourceId
	}
	resourceType := ""
	if event.ResourceType != nil {
		resourceType = *event.ResourceType
	}
	return fmt.Sprintf("  - %s (%s): %s", resourceID, resourceType, *event.ResourceStatusReason)
}

// GetOutputs retrieves the outputs from a CloudFormation stack.
func (d *AWSDeployer) GetOutputs(ctx context.Context, name string) (map[string]string, error) {
	return d.getStackOutputs(ctx, name)
}

// getStackOutputs is the internal implementation for retrieving stack outputs.
func (d *AWSDeployer) getStackOutputs(ctx context.Context, stackName string) (map[string]string, error) {
	result, err := d.client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe stacks: %w", err)
	}

	if len(result.Stacks) == 0 {
		return nil, errors.New("stack not found")
	}

	outputs := make(map[string]string)
	for _, out := range result.Stacks[0].Outputs {
		if out.OutputKey != nil && out.OutputValue != nil {
			outputs[*out.OutputKey] = *out.OutputValue
		}
	}

	return outputs, nil
}

// Destroy destroys the CloudFormation stack.
func (d *AWSDeployer) Destroy(ctx context.Context, opts *core.DestroyOptions) (*core.DestroyResult, error) {
	stackName := opts.Name
	result := &core.DestroyResult{
		Name: stackName,
	}

	stackExists, err := d.checkStackExists(ctx, stackName)
	if err != nil {
		return nil, fmt.Errorf("failed to check stack status: %w", err)
	}

	if !stackExists {
		result.NotFound = true
		result.Status = core.StatusNotFound
		return result, nil
	}

	err = d.deleteStack(ctx, stackName)
	if err != nil {
		return nil, fmt.Errorf("failed to delete stack: %w", err)
	}

	if !opts.Wait {
		result.Status = core.StatusInProgress
		return result, nil
	}

	finalStatus, err := d.waitForStackDeletion(ctx, stackName)
	if err != nil {
		return nil, fmt.Errorf("stack deletion failed: %w", err)
	}
	result.Status = finalStatus

	return result, nil
}

// deleteStack deletes a CloudFormation stack.
func (d *AWSDeployer) deleteStack(ctx context.Context, stackName string) error {
	_, err := d.client.DeleteStack(ctx, &cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete stack: %w", err)
	}
	return nil
}

// waitForStackDeletion waits for a stack deletion to complete.
func (d *AWSDeployer) waitForStackDeletion(ctx context.Context, stackName string) (string, error) {
	ticker := time.NewTicker(awsConstants.StackPollInterval)
	defer ticker.Stop()

	timeout := time.After(awsConstants.StackOperationTimeout)

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context canceled: %w", ctx.Err())
		case <-timeout:
			return "", errors.New("timeout waiting for stack deletion")
		case <-ticker.C:
			status, statusReason, err := d.getStackStatus(ctx, stackName)
			if err != nil {
				if strings.Contains(err.Error(), "does not exist") {
					return "DELETE_COMPLETE", nil
				}
				return "", err
			}

			switch types.StackStatus(status) {
			case types.StackStatusDeleteComplete:
				return status, nil
			case types.StackStatusDeleteFailed:
				failureDetails := d.getFailedResourceEvents(ctx, stackName)
				if failureDetails != "" {
					return status, fmt.Errorf(
						"stack deletion failed with status %s: %s\n\nResource failures:\n%s",
						status, statusReason, failureDetails)
				}
				return status, fmt.Errorf("stack deletion failed with status %s: %s", status, statusReason)
			case types.StackStatusDeleteInProgress:
			case types.StackStatusCreateInProgress, types.StackStatusCreateFailed, types.StackStatusCreateComplete,
				types.StackStatusRollbackInProgress, types.StackStatusRollbackFailed, types.StackStatusRollbackComplete,
				types.StackStatusUpdateInProgress, types.StackStatusUpdateCompleteCleanupInProgress,
				types.StackStatusUpdateComplete, types.StackStatusUpdateFailed,
				types.StackStatusUpdateRollbackInProgress, types.StackStatusUpdateRollbackFailed,
				types.StackStatusUpdateRollbackCompleteCleanupInProgress, types.StackStatusUpdateRollbackComplete,
				types.StackStatusReviewInProgress, types.StackStatusImportInProgress,
				types.StackStatusImportComplete, types.StackStatusImportRollbackInProgress,
				types.StackStatusImportRollbackFailed, types.StackStatusImportRollbackComplete:
				return status, fmt.Errorf("unexpected stack status during deletion: %s", status)
			}
		}
	}
}
