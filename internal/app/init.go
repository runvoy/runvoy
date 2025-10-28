package app

import (
	"context"
	"fmt"
	"log"
	"os"

	"runvoy/internal/constants"
	"runvoy/internal/database"
	dynamorepo "runvoy/internal/database/dynamodb"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// MustInitialize creates a new Service configured for the specified backend provider.
// It panics if initialization fails, making it suitable for application startup.
//
// Supported cloud providers:
//   - "aws": Uses DynamoDB for storage
//   - "gcp": (future) Uses Firestore for storage
//
// Environment variables required per cloud:
//
//	AWS:
//	  - API_KEYS_TABLE: DynamoDB table name for API keys
//	  - AWS credentials via standard AWS SDK environment variables
func MustInitialize(ctx context.Context, provider constants.BackendProvider) *Service {
	log.Printf("→ Initializing service for cloud provider: %s", provider)

	var userRepo database.UserRepository

	switch provider {
	case constants.AWS:
		userRepo = mustInitializeAWS(ctx)
	default:
		panic(fmt.Sprintf("Unknown backend provider: %s (supported: %s)", provider, constants.AWS))
	}

	log.Println("→ Service initialized successfully")
	return NewService(userRepo)
}

// mustInitializeAWS sets up AWS-specific dependencies
func mustInitializeAWS(ctx context.Context) database.UserRepository {
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to load AWS configuration: %v", err))
	}

	apiKeysTable := os.Getenv("API_KEYS_TABLE")
	if apiKeysTable == "" {
		panic("API_KEYS_TABLE environment variable is required for AWS")
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	log.Printf("→ Connected to DynamoDB table: %s", apiKeysTable)
	return dynamorepo.NewUserRepository(dynamoClient, apiKeysTable)
}
