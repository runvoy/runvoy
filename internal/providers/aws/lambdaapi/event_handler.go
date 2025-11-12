// Package lambdaapi provides Lambda handler creation for AWS Lambda,
// integrating cloud-provider agnostic components with AWS-specific entry points.
package lambdaapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"runvoy/internal/events"

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

		if awsResp, ok := result.(awsevents.APIGatewayProxyResponse); ok {
			return awsResp, nil
		}

		return awsevents.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Bad request",
		}, errors.New("result is not an APIGatewayProxyResponse")
	})
}
