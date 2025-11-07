// Package main implements the local async event processor server for runvoy.
// It runs the event processor service locally for testing and development.
// This allows testing of async Lambda events without deploying to AWS.
package main

import (
	"context"
	"os"

	"runvoy/cmd/local-async/server"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/logger"
)

func main() {
	cfg := config.MustLoadEventProcessor()
	log := logger.Initialize(constants.Development, cfg.GetLogLevel())

	if runErr := server.Run(context.Background(), cfg, log); runErr != nil {
		log.Error("failed to run async processor server", "error", runErr)
		os.Exit(1)
	}
}
