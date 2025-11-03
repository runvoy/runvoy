// Package main implements the AWS Lambda orchestrator for runvoy.
// It handles API requests and orchestrates ECS task executions.
package main

import (
	"context"
	"os"

	"runvoy/internal/app"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/lambdaapi"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	cfg := config.MustLoadOrchestrator()
	log := logger.Initialize(constants.Production, cfg.GetLogLevel())
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)
	defer cancel()

	svc, err := app.Initialize(ctx, constants.AWS, cfg, log)
	if err != nil {
		cancel()
		log.Error("failed to initialize service", "error", err)
		os.Exit(1)
	}

	log.Debug("starting Lambda handler")
	handler := lambdaapi.NewHandler(svc, cfg.RequestTimeout)
	lambda.StartHandler(handler)
}
