package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"runvoy/internal/app"
	"runvoy/internal/lambdaapi"
)

func main() {
	// Initialize service with DynamoDB support
	// MustInitialize will panic on fatal errors during cold start
	svc := app.MustInitialize(context.Background(), &app.InitConfig{
		EnableDynamoDB: true,
	})

	// Create Lambda handler and start
	handler := lambdaapi.NewHandler(svc)
	lambda.Start(handler.HandleRequest)
}
