// Package assets provides access to embedded CloudFormation templates and Lambda source code.
package assets

import (
	"embed"
)

// awsFiles embeds the AWS directory containing CloudFormation templates.
// Using embed.FS allows us to embed a directory tree without path traversal issues.
// This is organized under 'aws' to support future multi-cloud implementations.
//
//go:embed aws/*.yaml
var awsFiles embed.FS

// lambdaSource embeds the Lambda function source code.
// This allows the CLI to build the Lambda function at init time without requiring
// the backend/ directory to be present at runtime.
// Note: go.mod and go.sum are excluded to avoid module conflicts - they will be
// generated dynamically during the build process.
//
//go:embed lambda/**/*.go
var lambdaSource embed.FS

// GetCloudFormationBackendTemplate returns the main backend CloudFormation template.
// This template creates the Lambda function, ECS cluster, DynamoDB tables, VPC, and all
// required infrastructure for mycli.
func GetCloudFormationBackendTemplate() (string, error) {
	data, err := awsFiles.ReadFile("aws/cloudformation-backend.yaml")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetCloudFormationLambdaBucketTemplate returns the Lambda bucket CloudFormation template.
// This template creates the S3 bucket used to store the Lambda deployment package.
//
// NOTE: Currently, managing Lambda deployment with a single stack appears
// not feasible. This template creates the S3 bucket for Lambda code storage
// and handles its deletion. Ideally, Lambda deployment should be manageable
// fully within a single stack.
func GetCloudFormationLambdaBucketTemplate() (string, error) {
	data, err := awsFiles.ReadFile("aws/cloudformation-lambda-bucket.yaml")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetLambdaSourceFS returns the embedded filesystem containing Lambda source code.
// This can be used to extract and build the Lambda function at runtime.
func GetLambdaSourceFS() embed.FS {
	return lambdaSource
}
