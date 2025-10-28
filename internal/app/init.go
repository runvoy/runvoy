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

// MustInitialize creates a new Service configured for the specified cloud provider.
// It panics if initialization fails, making it suitable for application startup.
//
// Supported cloud providers:
//   - "aws": Uses DynamoDB for storage
//   - "gcp": (future) Uses Firestore for storage
//   - "azure": (future) Uses CosmosDB for storage
//   - "local": (future) Uses in-memory or local file storage
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
		panic(fmt.Sprintf("Unknown backend provider: %s (supported: aws)", provider))
	}

	log.Println("→ Service initialized successfully")
	return NewService(userRepo)
}

// mustInitializeAWS sets up AWS-specific dependencies
func mustInitializeAWS(ctx context.Context, cfg *config.Env) database.UserRepository {
	// Load AWS configuration from environment
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load AWS configuration: %v", err)
	}

	// Get required table name from configuration
	if cfg.APIKeysTable == "" {
		log.Fatal("API_KEYS_TABLE environment variable is required for AWS")
	}

	// Create DynamoDB client
	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	log.Printf("→ Connected to DynamoDB table: %s", cfg.APIKeysTable)
	return dynamorepo.NewUserRepository(dynamoClient, cfg.APIKeysTable)
}

// Future implementations would follow the same pattern:
//
// func mustInitializeGCP(ctx context.Context) database.UserRepository {
//     // Load GCP configuration
//     // Create Firestore client
//     // Return GCP-specific UserRepository implementation
// }
//
// func mustInitializeAzure(ctx context.Context) database.UserRepository {
//     // Load Azure configuration
//     // Create CosmosDB client
//     // Return Azure-specific UserRepository implementation
// }
//
// func mustInitializeLocal(ctx context.Context) database.UserRepository {
//     // Create in-memory or file-based storage
//     // Return local UserRepository implementation
// }
