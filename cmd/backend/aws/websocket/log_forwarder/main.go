// Package main implements the AWS Lambda log forwarder for runvoy WebSocket API.
// This is a placeholder implementation that will be completed in a future step.
package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handleRequest)
}

func handleRequest(_ context.Context, _ map[string]interface{}) error {
	return nil
}
