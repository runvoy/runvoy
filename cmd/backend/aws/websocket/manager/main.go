// Package main implements the AWS Lambda WebSocket manager for runvoy WebSocket API.
// It handles WebSocket connection lifecycle events ($connect and $disconnect)
// and disconnect notifications from the event processor.
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

	manager, err := websocket.NewWebSocketManager(ctx, cfg, log)
	cancel()
	if err != nil {
		log.Error("Failed to create WebSocket manager", "error", err)
		os.Exit(1)
	}

	log.Debug("starting Lambda handler")
	lambda.Start(manager.HandleRequest)
}
