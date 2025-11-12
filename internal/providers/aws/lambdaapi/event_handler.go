// Package lambdaapi provides Lambda handler creation for AWS Lambda,
// integrating cloud-provider agnostic components with AWS-specific entry points.
package lambdaapi

import (
	"context"
	"encoding/json"
	"net/http"

	"runvoy/internal/app/events"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// NewEventProcessorHandler creates a new Lambda handler for event processing.
// It wraps the generic event processor and converts responses to AWS Lambda types,
// allowing the core processor to remain cloud-provider agnostic while still
// supporting AWS Lambda's specific response formats.
func NewEventProcessorHandler(processor events.Processor) lambda.Handler {
	return lambda.NewHandler(func(
		ctx context.Context,
		rawEvent *json.RawMessage,
	) (awsevents.APIGatewayProxyResponse, error) {
		result, err := processor.Handle(ctx, rawEvent)
		if err != nil {
			return awsevents.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       err.Error(),
			}, err
		}

		if result != nil {
			if awsResp, ok := result.(awsevents.APIGatewayProxyResponse); ok {
				return awsResp, nil
			}
		}

		// Some events like logs don't expect a response, so we return a 200 OK
		return awsevents.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
		}, nil
	})
}
