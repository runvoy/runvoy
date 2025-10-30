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
	cfg := config.MustLoadEventProcessorEnv()
	log := logger.Initialize(constants.Production, cfg.LogLevel)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)
	defer cancel()

	processor, err := events.NewProcessor(ctx, cfg, log)
	if err != nil {
		log.Error("Failed to create event processor", "error", err)
		os.Exit(1)
	}

	log.Debug("starting Lambda handler")
	lambda.Start(processor.HandleEvent)
}
