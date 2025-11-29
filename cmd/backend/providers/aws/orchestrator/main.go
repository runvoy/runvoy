// Package main implements the AWS Lambda orchestrator for runvoy.
// It handles API requests and orchestrates ECS task executions.
package main

import (
	"context"
	"os"

	"github.com/runvoy/runvoy/internal/backend/orchestrator"
	"github.com/runvoy/runvoy/internal/config"
	"github.com/runvoy/runvoy/internal/constants"
	"github.com/runvoy/runvoy/internal/logger"
	"github.com/runvoy/runvoy/internal/providers/aws/lambdaapi"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	cfg := config.MustLoadOrchestrator()
	log := logger.Initialize(constants.Production, cfg.GetLogLevel())
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)

	svc, err := orchestrator.Initialize(ctx, cfg, log)
	cancel()
	if err != nil {
		log.Error("failed to initialize orchestrator", "error", err)
		os.Exit(1)
	}

	log.With("version", *constants.GetVersion()).Debug("starting orchestrator Lambda handler")
	handler := lambdaapi.NewHandler(svc, cfg.RequestTimeout, cfg.CORSAllowedOrigins)
	lambda.Start(handler)
}
