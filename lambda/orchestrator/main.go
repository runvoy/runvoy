package main

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
)

var cfg *Config

func init() {
	var err error
	cfg, err = InitConfig()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize config: %v", err))
	}
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Authenticate
    apiKey := request.Headers["x-api-key"]
    if apiKey == "" {
        apiKey = request.Headers["X-Api-Key"] // Try capitalized version
    }

	user, err := authenticate(ctx, cfg, apiKey)
    if err != nil {
		return errorResponse(401, fmt.Sprintf("unauthorized: %v", err))
	}

	// Route based on HTTP method and path
	method := request.HTTPMethod
	path := request.Path

	// Remove /prod prefix if present (API Gateway stage)
	if len(path) > 5 && path[:5] == "/prod" {
		path = path[5:]
	}

	fmt.Printf("[DEBUG] Routing: %s %s\n", method, path)

	var resp interface{}

	switch {
	case method == "POST" && path == "/executions":
		resp, err = handleCreateExecution(ctx, cfg, user, request)
	case method == "GET" && path == "/executions":
		resp, err = handleListExecutions(ctx, cfg, user, request)
	case method == "GET" && len(path) > 12 && path[:12] == "/executions/":
		// Extract execution ID from path
		parts := splitPath(path)
		if len(parts) >= 2 {
			executionID := parts[1]
			if len(parts) == 3 && parts[2] == "logs" {
				resp, err = handleGetExecutionLogs(ctx, cfg, user, executionID)
			} else if len(parts) == 2 {
				resp, err = handleGetExecution(ctx, cfg, user, executionID)
			} else {
				return errorResponse(404, "not found")
			}
		} else {
			return errorResponse(404, "not found")
		}
	case method == "GET" && path == "/locks":
		resp, err = handleListLocks(ctx, cfg, user)
	default:
		return errorResponse(404, fmt.Sprintf("not found: %s %s", method, path))
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

// splitPath splits a path into parts, removing empty strings
func splitPath(path string) []string {
	parts := []string{}
	current := ""
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(path[i])
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func main() {
	lambda.Start(handler)
}
