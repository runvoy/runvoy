package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Lambda Function URL handler signature
func handler(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	log.Printf("Received request: %s %s", request.RequestContext.HTTP.Method, request.RawPath)
	log.Printf("Request body: %s", request.Body)

	response := map[string]interface{}{
		"message": "not implemented",
	}

	body, err := json.Marshal(response)
	if err != nil {
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"error": "Failed to marshal response"}`,
		}, err
	}

	return events.LambdaFunctionURLResponse{
		StatusCode: http.StatusNotImplemented,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func main() {
	lambda.Start(handler)
}
