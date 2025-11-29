package infra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	awsconfig "github.com/runvoy/runvoy/internal/config/aws"
	"github.com/runvoy/runvoy/internal/constants"
)

const (
	// parameterSplitParts is the expected number of parts when splitting a KEY=VALUE parameter.
	parameterSplitParts = 2
)

// DeployOptions contains all options for deploying infrastructure.
type DeployOptions struct {
	StackName  string
	Template   string   // URL, S3 URI, or local file path
	Version    string   // Release version
	Parameters []string // KEY=VALUE format
	Wait       bool     // Wait for completion
	Region     string   // Provider region (optional)
}

// DeployResult contains the result of a deployment operation.
type DeployResult struct {
	StackName     string
	OperationType string // "CREATE" or "UPDATE"
	Status        string
	Outputs       map[string]string
	NoChanges     bool // True if stack was already up to date
}

// DestroyOptions contains all options for destroying infrastructure.
type DestroyOptions struct {
	StackName string
	Wait      bool   // Wait for completion
	Region    string // Provider region (optional)
}

// DestroyResult contains the result of a destroy operation.
type DestroyResult struct {
	StackName string
	Status    string
	NotFound  bool // True if stack was already deleted
}

// TemplateSource represents the resolved template source.
type TemplateSource struct {
	URL  string // For remote templates (S3/HTTPS)
	Body string // For local file templates
}

// Deployer defines the interface for infrastructure deployment.
// Different cloud providers implement this interface.
type Deployer interface {
	// Deploy deploys or updates infrastructure
	Deploy(ctx context.Context, opts *DeployOptions) (*DeployResult, error)
	// Destroy destroys infrastructure
	Destroy(ctx context.Context, opts *DestroyOptions) (*DestroyResult, error)
	// CheckStackExists checks if the infrastructure stack exists
	CheckStackExists(ctx context.Context, stackName string) (bool, error)
	// GetStackOutputs retrieves outputs from a deployed stack
	GetStackOutputs(ctx context.Context, stackName string) (map[string]string, error)
	// GetRegion returns the region being used
	GetRegion() string
}

// NewDeployer creates a Deployer for the specified provider.
// Currently supports: "aws".
func NewDeployer(ctx context.Context, provider, region string) (Deployer, error) {
	providerLower := strings.ToLower(provider)
	awsProvider := strings.ToLower(string(constants.AWS))
	switch providerLower {
	case awsProvider:
		return NewAWSDeployer(ctx, region)
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: %s)", provider, awsProvider)
	}
}

// ResolveTemplate determines the template source from the given input.
// Returns a TemplateSource with either URL or Body populated.
// region is the provider region to use for building default template URLs.
func ResolveTemplate(provider, template, version, region string) (*TemplateSource, error) {
	providerLower := strings.ToLower(provider)
	awsProvider := strings.ToLower(string(constants.AWS))
	switch providerLower {
	case awsProvider:
		return resolveAWSTemplate(template, version, region)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// resolveAWSTemplate resolves template for AWS provider.
func resolveAWSTemplate(template, version, region string) (*TemplateSource, error) {
	if template == "" {
		// Use default S3 URL with region
		url := awsconfig.BuildTemplateURL(version, region)
		return &TemplateSource{URL: url}, nil
	}

	// Check if it's a URL (starts with http:// or https://)
	if strings.HasPrefix(template, "http://") || strings.HasPrefix(template, "https://") {
		return &TemplateSource{URL: template}, nil
	}

	// Check if it's an S3 URI (starts with s3://)
	if s3Path, ok := strings.CutPrefix(template, "s3://"); ok {
		// Convert s3://bucket/key to https://bucket.s3.amazonaws.com/key
		parts := strings.SplitN(s3Path, "/", parameterSplitParts)
		if len(parts) < parameterSplitParts {
			return nil, fmt.Errorf("invalid S3 URI: %s", template)
		}
		bucket := parts[0]
		key := parts[1]
		url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucket, key)
		return &TemplateSource{URL: url}, nil
	}

	// Treat as local file
	cleanPath := filepath.Clean(template)
	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	return &TemplateSource{Body: string(content)}, nil
}

// ParseParameters parses KEY=VALUE parameter strings.
func ParseParameters(params []string) (map[string]string, error) {
	result := make(map[string]string)

	for _, param := range params {
		parts := strings.SplitN(param, "=", parameterSplitParts)
		if len(parts) != parameterSplitParts {
			return nil, fmt.Errorf("invalid parameter format: %s (expected KEY=VALUE)", param)
		}
		result[parts[0]] = parts[1]
	}

	return result, nil
}
