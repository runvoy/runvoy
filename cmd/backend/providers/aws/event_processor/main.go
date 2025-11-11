// Package main implements the AWS Lambda event processor for runvoy.
// It processes various AWS Lambda events including CloudWatch events related to ECS task completions
// and API Gateway WebSocket events.
package main

import (
	"context"
	"fmt"
	"os"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/events"
	"runvoy/internal/logger"
	awsevents "runvoy/internal/providers/aws/events"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	cfg := config.MustLoadEventProcessor()
	log := logger.Initialize(constants.Production, cfg.GetLogLevel())

	log.Debug(fmt.Sprintf("initializing %s event processor", constants.ProjectName),
		"context", map[string]any{
			"provider":             cfg.BackendProvider,
			"version":              *constants.GetVersion(),
			"init_timeout_seconds": cfg.InitTimeout.Seconds(),
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)

	// Initialize AWS-specific backend
	backend, err := awsevents.Initialize(ctx, cfg, log)
	cancel()
	if err != nil {
		log.Error("failed to initialize AWS backend", "error", err)
		os.Exit(1)
	}

	// Create generic processor with AWS backend
	processor := events.NewProcessor(backend, log)

	// Wrap the processor with AWS-specific handler for response conversion
	awsHandler := awsevents.NewLambdaHandler(processor)

	log.Debug(constants.ProjectName + " event processor initialized successfully")
	log.With("version", *constants.GetVersion()).Debug("starting event processor Lambda handler")
	lambda.Start(awsHandler.Handle)
}
