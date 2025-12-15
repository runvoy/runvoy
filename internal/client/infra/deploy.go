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

	// Operation types.
	operationTypeCreate = "CREATE"
	operationTypeUpdate = "UPDATE"

	// Status strings.
	statusInProgress     = "IN_PROGRESS"
	statusNotFound       = "NOT_FOUND"
	statusUpdateComplete = "UPDATE_COMPLETE"
	statusCreateComplete = "CREATE_COMPLETE"
)

// DeployOptions contains all options for deploying infrastructure.
type DeployOptions struct {
	Name       string   // Project/stack name (provider-specific: GCP project ID, AWS stack name)
	Template   string   // URL, S3 URI, or local file path (AWS only)
	Version    string   // Release version
	Parameters []string // KEY=VALUE format
	Wait       bool     // Wait for completion
	Region     string   // Provider region (optional)
	OrgID      string   // Organization ID for GCP (optional)
}

// DeployResult contains the result of a deployment operation.
type DeployResult struct {
	Name          string // Project/stack name
	OperationType string // "CREATE" or "UPDATE"
	Status        string
	Outputs       map[string]string
	NoChanges     bool // True if project/stack was already up to date
}

// DestroyOptions contains all options for destroying infrastructure.
type DestroyOptions struct {
	Name   string // Project/stack name
	Wait   bool   // Wait for completion
	Region string // Provider region (optional)
}

// DestroyResult contains the result of a destroy operation.
type DestroyResult struct {
	Name     string // Project/stack name
	Status   string
	NotFound bool // True if project/stack was already deleted
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
	// CheckExists checks if the infrastructure project/stack exists
	CheckExists(ctx context.Context, name string) (bool, error)
	// GetOutputs retrieves outputs from a deployed project/stack
	GetOutputs(ctx context.Context, name string) (map[string]string, error)
	// GetRegion returns the region being used
	GetRegion() string
}

// NewDeployer creates a Deployer for the specified provider.
// Currently supports: "aws", "gcp".
func NewDeployer(ctx context.Context, provider, region string) (Deployer, error) {
	switch strings.ToUpper(provider) {
	case string(constants.AWS):
		return NewAWSDeployer(ctx, region)
	case string(constants.GCP):
		return NewGCPDeployer(ctx, region)
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: %s)", provider, constants.ProvidersString())
	}
}

// ResolveTemplate determines the template source from the given input.
// Returns a TemplateSource with either URL or Body populated.
// region is the provider region to use for building default template URLs.
func ResolveTemplate(provider, template, version, region string) (*TemplateSource, error) {
	switch strings.ToUpper(provider) {
	case string(constants.AWS):
		return resolveAWSTemplate(template, version, region)
	case string(constants.GCP):
		// GCP project creation doesn't use templates
		return &TemplateSource{}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: %s)", provider, constants.ProvidersString())
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
