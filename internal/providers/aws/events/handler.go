// Package aws provides AWS-specific event processing implementations.
package aws

import (
	"context"
	"encoding/json"

	"runvoy/internal/events"

	awsevents "github.com/aws/aws-lambda-go/events"
)

// LambdaHandler wraps an events.Processor to handle AWS Lambda-specific conversions.
// It converts generic WebSocketResponse types to AWS API Gateway responses.
type LambdaHandler struct {
	processor *events.Processor
}

// NewLambdaHandler creates a new AWS Lambda-specific handler wrapper.
func NewLambdaHandler(processor *events.Processor) *LambdaHandler {
	return &LambdaHandler{
		processor: processor,
	}
}

// Handle processes the event and converts responses to AWS Lambda format.
// This allows the core processor to remain provider-agnostic while still
// satisfying AWS Lambda's expected response format.
func (h *LambdaHandler) Handle(ctx context.Context, rawEvent *json.RawMessage) (any, error) {
	result, err := h.processor.Handle(ctx, rawEvent)
	if err != nil {
		return nil, err
	}

	// If the result is a WebSocketResponse, convert it to AWS API Gateway format
	if wsResp, ok := result.(events.WebSocketResponse); ok {
		return awsevents.APIGatewayProxyResponse{
			StatusCode: wsResp.StatusCode,
			Headers:    wsResp.Headers,
			Body:       wsResp.Body,
		}, nil
	}

	// Return other results as-is (nil for non-WebSocket events)
	return result, nil
}
