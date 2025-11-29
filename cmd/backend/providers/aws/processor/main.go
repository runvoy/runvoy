// Package main implements the AWS Lambda event processor for runvoy.
// It processes various AWS Lambda events including CloudWatch events related to ECS task completions
// and API Gateway WebSocket events.
package main

import (
	"context"
	"os"

	"github.com/runvoy/runvoy/internal/backend/processor"
	"github.com/runvoy/runvoy/internal/config"
	"github.com/runvoy/runvoy/internal/constants"
	"github.com/runvoy/runvoy/internal/logger"
	"github.com/runvoy/runvoy/internal/providers/aws/lambdaapi"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	cfg := config.MustLoadEventProcessor()
	log := logger.Initialize(constants.Production, cfg.GetLogLevel())
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)

	proc, err := processor.Initialize(ctx, cfg, log)
	cancel()
	if err != nil {
		log.Error("failed to initialize event processor", "error", err)
		os.Exit(1)
	}

	log.With("version", *constants.GetVersion()).Debug("starting event processor Lambda handler")
	lambda.Start(lambdaapi.NewEventProcessorHandler(proc))
}
