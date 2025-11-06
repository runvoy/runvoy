// Package main implements the AWS Lambda event processor for runvoy.
// It processes CloudWatch events related to ECS task completions and CloudWatch Logs.
package main

import (
	"context"
	"encoding/json"
	"os"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/events"
	"runvoy/internal/logger"
)

// LambdaEvent represents both EventBridge and direct CloudWatch Logs events
type LambdaEvent struct {
	// EventBridge fields
	DetailType string          `json:"detail-type"`
	Detail     json.RawMessage `json:"detail"`
	Source     string          `json:"source"`
	// Direct CloudWatch Logs fields
	AWSLogs struct {
		Data []byte `json:"data"`
	} `json:"awslogs"`
}

func main() {
	cfg := config.MustLoadEventProcessor()
	log := logger.Initialize(constants.Production, cfg.GetLogLevel())
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)

	processor, err := events.NewProcessor(ctx, cfg, log)
	cancel()
	if err != nil {
		log.Error("Failed to create event processor", "error", err)
		os.Exit(1)
	}

	log.Debug("starting Lambda handler")
	lambda.Start(func(ctx context.Context, rawEvent json.RawMessage) error {
		var event LambdaEvent
		if parseErr := json.Unmarshal(rawEvent, &event); parseErr != nil {
			log.Error("failed to unmarshal event", "error", parseErr)
			return parseErr
		}

		// Check if this is a direct CloudWatch Logs event
		if len(event.AWSLogs.Data) > 0 {
			return processor.HandleCloudWatchLogsEvent(ctx, event.AWSLogs.Data)
		}

		// Otherwise, treat as EventBridge event
		var ebEvent awsevents.CloudWatchEvent
		if ebParseErr := json.Unmarshal(rawEvent, &ebEvent); ebParseErr != nil {
			log.Error("failed to unmarshal EventBridge event", "error", ebParseErr)
			return ebParseErr
		}
		return processor.HandleEvent(ctx, &ebEvent)
	})
}
