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
	cfg := config.MustLoadEnv()
	log := logger.Initialize(constants.Production, cfg.LogLevel)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)
	defer cancel()

	svc, err := app.Initialize(ctx, constants.AWS, cfg, log)
	if err != nil {
		log.Error("Failed to initialize service", "error", err)
		os.Exit(1)
	}

	log.Debug("Starting Lambda handler")
	lambda.Start(lambdaapi.NewHandler(svc).HandleRequest)
}
