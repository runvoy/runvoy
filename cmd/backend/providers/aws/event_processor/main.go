// Package main implements the AWS Lambda event processor for runvoy.
// It processes various AWS Lambda events including CloudWatch events related to ECS task completions
// and API Gateway WebSocket events.
package main

import (
	"context"
	"log/slog"
	"os"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/events"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	cfg := config.MustLoadEventProcessor()
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	log := logger.Initialize(constants.Production, level)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)

	processor, err := events.Initialize(ctx, cfg, log)
	cancel()
	if err != nil {
		log.Error("failed to initialize event processor", "error", err)
		os.Exit(1)
	}

	log.With("version", *constants.GetVersion()).Debug("starting event processor Lambda handler")
	lambda.Start(processor.Handle)
}
