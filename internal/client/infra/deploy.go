// Package infra provides infrastructure deployment functionality.
package infra

import (
	"context"
	"fmt"
	"os"
	"strings"
)

const (
	// DefaultStackName is the default CloudFormation stack name
	DefaultStackName = "runvoy"

	// ProviderAWS is the AWS provider identifier
	ProviderAWS = "aws"
)

// DeployOptions contains all options for deploying infrastructure
type DeployOptions struct {
	StackName  string
	Template   string   // URL, S3 URI, or local file path
	Version    string   // Release version
	Parameters []string // KEY=VALUE format
	Wait       bool     // Wait for completion
	Region     string   // Provider region (optional)
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
	URL  string // For remote templates (S3/HTTPS)
	Body string // For local file templates
}

// Deployer defines the interface for infrastructure deployment.
// Different cloud providers implement this interface.
type Deployer interface {
	// Deploy deploys or updates infrastructure
	Deploy(ctx context.Context, opts DeployOptions) (*DeployResult, error)
	// CheckStackExists checks if the infrastructure stack exists
	CheckStackExists(ctx context.Context, stackName string) (bool, error)
	// GetStackOutputs retrieves outputs from a deployed stack
	GetStackOutputs(ctx context.Context, stackName string) (map[string]string, error)
	// GetRegion returns the region being used
	GetRegion() string
}

// NewDeployer creates a Deployer for the specified provider.
// Currently supports: "aws"
func NewDeployer(ctx context.Context, provider string, region string) (Deployer, error) {
	switch strings.ToLower(provider) {
	case ProviderAWS:
		return NewAWSDeployer(ctx, region)
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: %s)", provider, ProviderAWS)
	}
}

// ResolveTemplate determines the template source from the given input.
// Returns a TemplateSource with either URL or Body populated.
func ResolveTemplate(provider, template, version string) (*TemplateSource, error) {
	switch strings.ToLower(provider) {
	case ProviderAWS:
		return resolveAWSTemplate(template, version)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// resolveAWSTemplate resolves template for AWS provider
func resolveAWSTemplate(template, version string) (*TemplateSource, error) {
	if template == "" {
		// Use default S3 URL
		url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s/%s",
			awsDefaultS3Bucket, version, awsTemplateFileName)
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

// ParseParameters parses KEY=VALUE parameter strings
func ParseParameters(params []string) (map[string]string, error) {
	result := make(map[string]string)

	for _, param := range params {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid parameter format: %s (expected KEY=VALUE)", param)
		}
		result[parts[0]] = parts[1]
	}

	return result, nil
}
