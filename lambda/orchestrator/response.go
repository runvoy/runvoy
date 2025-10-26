package main

import (
	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
)

func errorResponse(statusCode int, message string) (events.APIGatewayProxyResponse, error) {
	resp := Response{Error: message}
	body, _ := json.Marshal(resp)
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}
