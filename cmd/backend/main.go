package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"runvoy/cmd/backend/aws"
	"runvoy/internal/handlers"
	"runvoy/internal/services"
)

var h *handlers.Handlers

func init() {
	// Initialize AWS services
	storage := aws.NewDynamoDBStorage()
	ecs := aws.NewECSService()
	lock := aws.NewLockService(storage)
	logService := aws.NewLogService()
	auth := aws.NewAuthService(storage)

	// Initialize execution service
	execution := services.NewExecutionService(storage, ecs, lock, logService)

	// Initialize handlers
	h = handlers.NewHandlers(auth, execution)
}

// Handler is the Lambda function handler
func Handler(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	log.Printf("Received request: %s %s", request.RequestContext.HTTP.Method, request.RawPath)

	// Create HTTP request from Lambda event
	req, err := aws.LambdaToHTTPRequest(request)
	if err != nil {
		log.Printf("Error converting Lambda request: %v", err)
		return events.LambdaFunctionURLResponse{
			StatusCode: 500,
			Body:       `{"error": "Internal server error"}`,
		}, nil
	}

	// Create response writer
	w := aws.NewLambdaResponseWriter()

	// Route the request
	switch {
	case request.RawPath == "/executions" && request.RequestContext.HTTP.Method == "POST":
		h.ExecutionHandler(w, req)
	case request.RawPath == "/health" && request.RequestContext.HTTP.Method == "GET":
		h.HealthHandler(w, req)
	case len(request.RawPath) > 8 && request.RawPath[:8] == "/status/" && request.RequestContext.HTTP.Method == "GET":
		h.StatusHandler(w, req)
	default:
		w.WriteHeader(404)
		w.Write([]byte(`{"error": "Not found"}`))
	}

	// Convert response back to Lambda format
	response := w.ToLambdaResponse()
	log.Printf("Response: %d", response.StatusCode)

	return response, nil
}

func main() {
	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Start Lambda handler
	lambda.Start(Handler)
}
