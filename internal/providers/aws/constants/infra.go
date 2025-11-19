// Package constants provides AWS-specific constants for infrastructure deployment.
package constants

const (
	// DefaultInfraStackName is the default CloudFormation stack name for AWS infra deployments
	DefaultInfraStackName = "runvoy-backend"

	// ReleasesBucket is the S3 bucket name for runvoy releases
	ReleasesBucket = "runvoy-releases"

	// ReleasesBucketRegion is the AWS region where the releases bucket is located
	ReleasesBucketRegion = "us-east-2"

	// CloudFormationTemplateFile is the filename of the CloudFormation template in releases
	CloudFormationTemplateFile = "cloudformation-backend.yaml"
)
