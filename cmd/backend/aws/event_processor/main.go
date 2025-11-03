// Package main implements the AWS Lambda event processor for runvoy.
// It processes CloudWatch events related to ECS task completions.
package main

import (
	"context"
	"os"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/events"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	cfg := config.MustLoadEventProcessor()
	log := logger.Initialize(constants.Production, cfg.GetLogLevel())
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)

	processor, err := events.NewProcessor(ctx, cfg, log)
	cancel()
	if err != nil {
		log.Error("Failed to create event processor", "error", err)
		os.Exit(1)
	}

	log.Debug("starting Lambda handler")
	lambda.Start(processor.HandleEvent)
}
