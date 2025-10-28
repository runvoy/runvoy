package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"runvoy/internal/app"
	dynamorepo "runvoy/internal/database/dynamodb"
	"runvoy/internal/lambdaapi"
)

func main() {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	// Create DynamoDB client
	dynamoClient := dynamodb.NewFromConfig(cfg)

	// Get table name from environment variable
	apiKeysTableName := os.Getenv("API_KEYS_TABLE")
	if apiKeysTableName == "" {
		log.Println("WARNING: API_KEYS_TABLE not set, user operations will not be available")
	}

	// Create repository (nil if table name not set)
	var userRepo *dynamorepo.UserRepository
	if apiKeysTableName != "" {
		userRepo = dynamorepo.NewUserRepository(dynamoClient, apiKeysTableName)
	}

	// Create service with repository
	svc := app.NewService(userRepo)

	// Create Lambda handler and start
	handler := lambdaapi.NewHandler(svc)
	lambda.Start(handler.HandleRequest)
}
