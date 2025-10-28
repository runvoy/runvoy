package app

import (
	"context"
	"fmt"
	"log"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	dynamorepo "runvoy/internal/database/dynamodb"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
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
//
// Example:
//
//	cfg := config.MustLoadEnv()
//	svc := app.MustInitialize(context.Background(), "aws", cfg)
func MustInitialize(ctx context.Context, provider constants.BackendProvider, cfg *config.Env) *Service {
	log.Printf("→ Initializing service for cloud provider: %s", provider)

	var userRepo database.UserRepository

	switch provider {
	case constants.AWS:
		userRepo = mustInitializeAWS(ctx, cfg)
	default:
		panic(fmt.Sprintf("Unknown backend provider: %s (supported: %s)", provider, constants.AWS))
	}

	log.Println("→ Service initialized successfully")
	return NewService(userRepo)
}

// mustInitializeAWS sets up AWS-specific dependencies
func mustInitializeAWS(ctx context.Context, cfg *config.Env) database.UserRepository {
	// Load AWS configuration from environment
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to load AWS configuration: %v", err))
	}

	// Get required table name from configuration
	if cfg.APIKeysTable == "" {
		log.Fatal("API_KEYS_TABLE environment variable is required for AWS")
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	log.Printf("→ Connected to DynamoDB table: %s", cfg.APIKeysTable)
	return dynamorepo.NewUserRepository(dynamoClient, cfg.APIKeysTable)
}
