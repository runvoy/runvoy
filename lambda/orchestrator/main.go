package main

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Parse request body
	var req Request
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return errorResponse(400, fmt.Sprintf("invalid request: %v", err))
	}

    // Authenticate
    apiKey := request.Headers["x-api-key"]
    if apiKey == "" {
        apiKey = request.Headers["X-Api-Key"] // Try capitalized version
    }

    if !authenticate(apiKey) {
		return errorResponse(401, "unauthorized")
	}

	// Route to handler
	var resp Response
	var err error

    switch req.Action {
    case "exec":
        resp, err = handleExec(ctx, req)
    case "status":
        resp, err = handleStatus(ctx, req)
    case "logs":
        resp, err = handleLogs(ctx, req)
    default:
        return errorResponse(400, "invalid action")
    }

	if err != nil {
		return errorResponse(500, err.Error())
	}

	body, _ := json.Marshal(resp)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func main() {
	lambda.Start(handler)
}
