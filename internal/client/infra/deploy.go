package infra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/runvoy/runvoy/internal/client/infra/core"
	"github.com/runvoy/runvoy/internal/client/infra/gcp"
	awsconfig "github.com/runvoy/runvoy/internal/config/aws"
	"github.com/runvoy/runvoy/internal/constants"
)

const (
	// parameterSplitParts is the expected number of parts when splitting a KEY=VALUE parameter.
	parameterSplitParts = 2
)

// NewDeployer creates a Deployer for the specified provider.
// Currently supports: "aws", "gcp".
func NewDeployer(ctx context.Context, provider, region string) (core.Deployer, error) {
	switch strings.ToUpper(provider) {
	case string(constants.AWS):
		return NewAWSDeployer(ctx, region)
	case string(constants.GCP):
		deployer, err := gcp.NewDeployer(ctx, region)
		if err != nil {
			return nil, fmt.Errorf("create gcp deployer: %w", err)
		}
		return deployer, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: %s)", provider, constants.ProvidersString())
	}
}

// ResolveTemplate determines the template source from the given input.
// Returns a TemplateSource with either URL or Body populated.
// region is the provider region to use for building default template URLs.
func ResolveTemplate(provider, template, version, region string) (*core.TemplateSource, error) {
	switch strings.ToUpper(provider) {
	case string(constants.AWS):
		return resolveAWSTemplate(template, version, region)
	case string(constants.GCP):
		// GCP project creation doesn't use templates
		return &core.TemplateSource{}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: %s)", provider, constants.ProvidersString())
	}
}

// resolveAWSTemplate resolves template for AWS provider.
func resolveAWSTemplate(template, version, region string) (*core.TemplateSource, error) {
	if template == "" {
		// Use default S3 URL with region
		url := awsconfig.BuildTemplateURL(version, region)
		return &core.TemplateSource{URL: url}, nil
	}

	// Check if it's a URL (starts with http:// or https://)
	if strings.HasPrefix(template, "http://") || strings.HasPrefix(template, "https://") {
		return &core.TemplateSource{URL: template}, nil
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
		return &core.TemplateSource{URL: url}, nil
	}

	// Treat as local file
	cleanPath := filepath.Clean(template)
	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	return &core.TemplateSource{Body: string(content)}, nil
}
