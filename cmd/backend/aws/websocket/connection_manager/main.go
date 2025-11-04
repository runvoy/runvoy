// Package main implements the AWS Lambda connection manager for runvoy WebSocket API.
// This is a placeholder implementation that will be completed in a future step.
package main

import (
	"context"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handleRequest)
}

//nolint:gocritic // Lambda handler signature requires this parameter type
func handleRequest(
	_ context.Context,
	_ events.APIGatewayWebsocketProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "OK",
	}, nil
}
