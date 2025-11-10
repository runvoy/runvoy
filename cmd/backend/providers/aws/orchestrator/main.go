// Package main implements the AWS Lambda orchestrator for runvoy.
// It handles API requests and orchestrates ECS task executions.
package main

import (
	"context"
	"log/slog"
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
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	log := logger.Initialize(constants.Production, level)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)

	svc, err := app.Initialize(ctx, constants.AWS, cfg, log)
	cancel()
	if err != nil {
		log.Error("failed to initialize orchestrator", "error", err)
		os.Exit(1)
	}

	log.With("version", *constants.GetVersion()).Debug("starting orchestrator Lambda handler")
	handler := lambdaapi.NewHandler(svc, cfg.RequestTimeout)
	lambda.Start(handler)
}
