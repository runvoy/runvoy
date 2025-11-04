// Package main implements the AWS Lambda connection manager for runvoy WebSocket API.
// It handles WebSocket connection lifecycle events ($connect and $disconnect).
package main

import (
	"context"
	"os"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/logger"
	"runvoy/internal/websocket"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	cfg := config.MustLoadConnectionManager()
	log := logger.Initialize(constants.Production, cfg.GetLogLevel())
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)

	manager, err := websocket.NewConnectionManager(ctx, cfg, log)
	cancel()
	if err != nil {
		log.Error("Failed to create connection manager", "error", err)
		os.Exit(1)
	}

	log.Debug("starting Lambda handler")
	lambda.Start(manager.HandleRequest)
}
