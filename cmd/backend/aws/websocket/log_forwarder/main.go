// Package main implements the AWS Lambda log forwarder for runvoy WebSocket API.
// It forwards CloudWatch Logs events to connected WebSocket clients.
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
	cfg := config.MustLoadLogForwarder()
	log := logger.Initialize(constants.Production, cfg.GetLogLevel())
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)

	forwarder, err := websocket.NewLogForwarder(ctx, cfg, log)
	cancel()
	if err != nil {
		log.Error("Failed to create log forwarder", "error", err)
		os.Exit(1)
	}

	log.Debug("starting Lambda handler")
	lambda.Start(forwarder.Handle)
}
