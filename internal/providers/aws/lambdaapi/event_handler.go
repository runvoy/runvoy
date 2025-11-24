// Package lambdaapi provides Lambda handler creation for AWS Lambda,
// integrating cloud-provider agnostic components with AWS-specific entry points.
package lambdaapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"runvoy/internal/backend/processor"

	"github.com/aws/aws-lambda-go/lambda"
)

// NewEventProcessorHandler creates a new Lambda handler for event processing.
// It wraps the generic event processor and converts responses to AWS Lambda types,
// allowing the core processor to remain cloud-provider agnostic while still
// supporting AWS Lambda's specific response formats.
func NewEventProcessorHandler(proc processor.Processor) lambda.Handler {
	if proc == nil {
		panic("processor is required")
	}
	return lambda.NewHandler(func(
		ctx context.Context,
		rawEvent *json.RawMessage,
	) (json.RawMessage, error) {
		result, err := proc.Handle(ctx, rawEvent)
		if err != nil {
			return json.RawMessage(
				fmt.Sprintf(`{"status_code": %d, "body": %q}`,
					http.StatusInternalServerError, err.Error(),
				),
			), err
		}

		if result != nil {
			return *result, nil
		}

		// Some events like logs don't expect a response, so we return a 200 OK
		return json.RawMessage(fmt.Sprintf(`{"status_code": %d, "body": "OK"}`, http.StatusOK)), nil
	})
}
