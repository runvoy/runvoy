// Package main implements the AWS Lambda event processor for runvoy.
// It processes various AWS Lambda events including CloudWatch events related to ECS task completions
// and API Gateway WebSocket events.
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/events"
	"runvoy/internal/logger"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	cfg := config.MustLoadEventProcessor()
	log := logger.Initialize(constants.Production, cfg.GetLogLevel())
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)

	processor, err := events.Initialize(ctx, cfg, log)
	cancel()
	if err != nil {
		log.Error("failed to initialize event processor", "error", err)
		os.Exit(1)
	}

	log.With("version", *constants.GetVersion()).Debug("starting event processor Lambda handler")
	// Wrap the processor to handle response conversion from generic to AWS-specific types
	lambda.Start(awsLambdaHandler(processor))
}

// awsLambdaHandler wraps the generic event processor and converts responses to AWS Lambda types.
// This allows the core event processor to remain cloud-provider agnostic while still
// supporting AWS Lambda's specific response formats.
func awsLambdaHandler(
	processor events.Processor,
) func(
	ctx context.Context,
	rawEvent *json.RawMessage,
) (awsevents.APIGatewayProxyResponse, error) {
	return func(ctx context.Context, rawEvent *json.RawMessage) (awsevents.APIGatewayProxyResponse, error) {
		result, err := processor.Handle(ctx, rawEvent)

		if err != nil {
			return awsevents.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       err.Error(),
			}, nil // Return nil error to Lambda so it doesn't treat it as a handler failure
		}

		// If result is an APIGatewayProxyResponse, return it directly
		if awsResp, ok := result.(awsevents.APIGatewayProxyResponse); ok {
			return awsResp, nil
		}

		// For non-WebSocket events, return a 200 OK with no body
		// (CloudWatch events and CloudWatch Logs are typically async and don't expect a response)
		return awsevents.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Body:       "OK",
		}, nil
	}
}
