// Package main implements the AWS Lambda event processor for runvoy.
// It processes various AWS Lambda events including CloudWatch events related to ECS task completions
// and API Gateway WebSocket events.
package main

import (
	"context"
	"os"

	"runvoy/internal/app/events"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/logger"
	"runvoy/internal/providers/aws/lambdaapi"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	cfg := config.MustLoadEventProcessor()
	log := logger.Initialize(constants.Production, cfg.GetLogLevel())
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)

	processor, err := events.Initialize(ctx, cfg, log)
	cancel()
	if err != nil {
		log.Error("failed to initialize event processor", "error", err)
		os.Exit(1)
	}

	log.With("version", *constants.GetVersion()).Debug("starting event processor Lambda handler")
	lambda.Start(lambdaapi.NewEventProcessorHandler(processor))
}
