package constants

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// rawReleaseRegions contains the comma-separated list of AWS regions for releases.
// This value is injected at build time via ldflags.
var rawReleaseRegions = "" // Updated by build system at build time

// GetReleaseRegions returns a slice of supported AWS regions for releases.
// The regions are parsed from the comma-separated string injected at build time.
func GetReleaseRegions() []string {
	if rawReleaseRegions == "" {
		return []string{}
	}

	regions := strings.Split(rawReleaseRegions, ",")
	// Trim whitespace from each region
	result := make([]string, 0, len(regions))
	for _, r := range regions {
		trimmed := strings.TrimSpace(r)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// ValidateRegion validates that the given region is in the supported regions list.
// Returns an error if the region is not supported, or if no regions are configured.
func ValidateRegion(region string) error {
	if region == "" {
		return errors.New("region cannot be empty")
	}

	supportedRegions := GetReleaseRegions()
	if len(supportedRegions) == 0 {
		// If no regions configured at build time, skip validation
		// (e.g., during development or if regions flag wasn't set)
		return nil
	}

	region = strings.TrimSpace(region)
	if slices.Contains(supportedRegions, region) {
		return nil
	}

	return fmt.Errorf(
		"region %q is not supported. Supported regions: %s",
		region,
		strings.Join(supportedRegions, ", "),
	)
}

const (
	// DefaultInfraStackName is the default CloudFormation stack name for AWS infra deployments.
	DefaultInfraStackName = "runvoy-backend"

	// ReleasesBucketRegion is the AWS region where the releases bucket is located.
	ReleasesBucketRegion = "us-east-1"

	// ReleasesBucket is the S3 bucket name for runvoy releases.
	ReleasesBucket = "runvoy-releases-" + ReleasesBucketRegion

	// CloudFormationTemplateFile is the filename of the CloudFormation template in releases.
	CloudFormationTemplateFile = "cloudformation-backend.yaml"

	// StackPollInterval is the interval at which to poll for CloudFormation stack status changes.
	StackPollInterval = 5 * time.Second

	// StackOperationTimeout is the maximum time to wait for a stack operation to complete.
	StackOperationTimeout = 30 * time.Minute
)
