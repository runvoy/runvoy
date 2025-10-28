package app

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"runvoy/internal/database"
	dynamorepo "runvoy/internal/database/dynamodb"
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
//   AWS:
//     - API_KEYS_TABLE: DynamoDB table name for API keys
//     - AWS credentials via standard AWS SDK environment variables
//
// Example:
//
//	svc := app.MustInitialize(context.Background(), "aws")
func MustInitialize(ctx context.Context, cloud string) *Service {
	log.Printf("→ Initializing service for cloud provider: %s", cloud)

	var userRepo database.UserRepository

	switch cloud {
	case "aws":
		userRepo = mustInitializeAWS(ctx)
	case "gcp":
		log.Fatal("GCP support not yet implemented")
	case "azure":
		log.Fatal("Azure support not yet implemented")
	case "local":
		log.Fatal("Local storage support not yet implemented")
	default:
		log.Fatalf("Unknown cloud provider: %s (supported: aws, gcp, azure, local)", cloud)
	}

	log.Println("→ Service initialized successfully")
	return NewService(userRepo)
}

// mustInitializeAWS sets up AWS-specific dependencies
func mustInitializeAWS(ctx context.Context) database.UserRepository {
	// Load AWS configuration from environment
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load AWS configuration: %v", err)
	}

	// Get required table name from environment
	apiKeysTable := os.Getenv("API_KEYS_TABLE")
	if apiKeysTable == "" {
		log.Fatal("API_KEYS_TABLE environment variable is required for AWS")
	}

	// Create DynamoDB client
	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	log.Printf("→ Connected to DynamoDB table: %s", apiKeysTable)
	return dynamorepo.NewUserRepository(dynamoClient, apiKeysTable)
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
