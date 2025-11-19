// Package infra provides infrastructure deployment functionality.
package infra

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

const (
	// DefaultS3Bucket is the default bucket for runvoy releases
	DefaultS3Bucket = "runvoy-releases"
	// DefaultStackName is the default CloudFormation stack name
	DefaultStackName = "runvoy"
	// TemplateFileName is the CloudFormation template file name
	TemplateFileName = "cloudformation-backend.yaml"
	// StackPollInterval is how often to check stack status
	StackPollInterval = 5 * time.Second
	// StackOperationTimeout is the maximum time to wait for stack operations
	StackOperationTimeout = 30 * time.Minute
)

// CloudFormationClient defines the interface for CloudFormation operations.
// This interface enables mocking for unit tests.
type CloudFormationClient interface {
	DescribeStacks(ctx context.Context, params *cloudformation.DescribeStacksInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error)
	CreateStack(ctx context.Context, params *cloudformation.CreateStackInput, optFns ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error)
	UpdateStack(ctx context.Context, params *cloudformation.UpdateStackInput, optFns ...func(*cloudformation.Options)) (*cloudformation.UpdateStackOutput, error)
}

// DeployOptions contains all options for deploying infrastructure
type DeployOptions struct {
	StackName  string
	Template   string   // URL, S3 URI, or local file path
	Version    string   // Release version
	Parameters []string // KEY=VALUE format
	Wait       bool     // Wait for completion
}

// DeployResult contains the result of a deployment operation
type DeployResult struct {
	StackName     string
	OperationType string // "CREATE" or "UPDATE"
	Status        string
	Outputs       map[string]string
	NoChanges     bool // True if stack was already up to date
}

// TemplateSource represents the resolved template source
type TemplateSource struct {
	URL  string // For S3/HTTPS templates
	Body string // For local file templates
}

// DeployService handles infrastructure deployment logic
type DeployService struct {
	client CloudFormationClient
}

// NewDeployService creates a new DeployService
func NewDeployService(client CloudFormationClient) *DeployService {
	return &DeployService{
		client: client,
	}
}

// ResolveTemplate determines the template source from the given input.
// Returns a TemplateSource with either URL or Body populated.
func ResolveTemplate(template, version string) (*TemplateSource, error) {
	if template == "" {
		// Use default S3 URL
		url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s/%s",
			DefaultS3Bucket, version, TemplateFileName)
		return &TemplateSource{URL: url}, nil
	}

	// Check if it's a URL (starts with http:// or https://)
	if strings.HasPrefix(template, "http://") || strings.HasPrefix(template, "https://") {
		return &TemplateSource{URL: template}, nil
	}

	// Check if it's an S3 URI (starts with s3://)
	if strings.HasPrefix(template, "s3://") {
		// Convert s3://bucket/key to https://bucket.s3.amazonaws.com/key
		parts := strings.SplitN(strings.TrimPrefix(template, "s3://"), "/", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid S3 URI: %s", template)
		}
		bucket := parts[0]
		key := parts[1]
		url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucket, key)
		return &TemplateSource{URL: url}, nil
	}

	// Treat as local file
	content, err := os.ReadFile(template)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	return &TemplateSource{Body: string(content)}, nil
}

// ParseParameters parses KEY=VALUE parameter strings into CloudFormation parameters
func ParseParameters(params []string) ([]types.Parameter, error) {
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

// Deploy deploys or updates the CloudFormation stack
func (s *DeployService) Deploy(ctx context.Context, opts DeployOptions) (*DeployResult, error) {
	// Resolve template
	templateSource, err := ResolveTemplate(opts.Template, opts.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve template: %w", err)
	}

	// Parse parameters
	cfnParams, err := ParseParameters(opts.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	// Check if stack exists
	stackExists, err := s.CheckStackExists(ctx, opts.StackName)
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
		err = s.updateStack(ctx, opts.StackName, templateSource, cfnParams)
	} else {
		result.OperationType = "CREATE"
		err = s.createStack(ctx, opts.StackName, templateSource, cfnParams)
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
	finalStatus, err := s.WaitForStackOperation(ctx, opts.StackName)
	if err != nil {
		return nil, fmt.Errorf("stack operation failed: %w", err)
	}
	result.Status = finalStatus

	// Get stack outputs
	outputs, err := s.GetStackOutputs(ctx, opts.StackName)
	if err != nil {
		// Don't fail, just return without outputs
		return result, nil
	}
	result.Outputs = outputs

	return result, nil
}

// CheckStackExists checks if a CloudFormation stack exists
func (s *DeployService) CheckStackExists(ctx context.Context, stackName string) (bool, error) {
	_, err := s.client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
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
func (s *DeployService) createStack(ctx context.Context, stackName string, template *TemplateSource, params []types.Parameter) error {
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

	_, err := s.client.CreateStack(ctx, input)
	return err
}

// updateStack updates an existing CloudFormation stack
func (s *DeployService) updateStack(ctx context.Context, stackName string, template *TemplateSource, params []types.Parameter) error {
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

	_, err := s.client.UpdateStack(ctx, input)
	return err
}

// WaitForStackOperation waits for a stack create/update to complete
func (s *DeployService) WaitForStackOperation(ctx context.Context, stackName string) (string, error) {
	ticker := time.NewTicker(StackPollInterval)
	defer ticker.Stop()

	timeout := time.After(StackOperationTimeout)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for stack operation")
		case <-ticker.C:
			status, statusReason, err := s.getStackStatus(ctx, stackName)
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
				types.StackStatusDeleteFailed:
				return status, fmt.Errorf("stack operation failed with status %s: %s", status, statusReason)
			}
			// Still in progress, continue polling
		}
	}
}

// getStackStatus returns the current status of a stack
func (s *DeployService) getStackStatus(ctx context.Context, stackName string) (string, string, error) {
	result, err := s.client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return "", "", err
	}

	if len(result.Stacks) == 0 {
		return "", "", fmt.Errorf("stack not found")
	}

	status := string(result.Stacks[0].StackStatus)
	reason := ""
	if result.Stacks[0].StackStatusReason != nil {
		reason = *result.Stacks[0].StackStatusReason
	}

	return status, reason, nil
}

// GetStackOutputs retrieves the outputs from a CloudFormation stack
func (s *DeployService) GetStackOutputs(ctx context.Context, stackName string) (map[string]string, error) {
	result, err := s.client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
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

// GetCurrentStackStatus returns the current status of a stack for progress reporting
func (s *DeployService) GetCurrentStackStatus(ctx context.Context, stackName string) (string, error) {
	status, _, err := s.getStackStatus(ctx, stackName)
	return status, err
}
