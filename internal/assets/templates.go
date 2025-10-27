// Package assets provides access to embedded CloudFormation templates.
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
func GetCloudFormationLambdaBucketTemplate() (string, error) {
	data, err := awsFiles.ReadFile("aws/cloudformation-lambda-bucket.yaml")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
