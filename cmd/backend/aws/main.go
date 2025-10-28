package main

import (
	"context"

	"runvoy/internal/app"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/lambdaapi"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	// Load environment configuration
	cfg := config.MustLoadEnv()

	// Initialize service
	svc := app.MustInitialize(context.Background(), constants.AWS, cfg)
	handler := lambdaapi.NewHandler(svc)
	lambda.Start(handler.HandleRequest)
}
