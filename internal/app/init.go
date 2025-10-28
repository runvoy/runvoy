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

// Initialize creates a new Service configured for the specified backend provider.
// It returns an error if the context is cancelled, timed out, or if an unknown provider is specified.
// Callers should handle errors and potentially panic if initialization fails during startup.
//
// Supported cloud providers:
//   - "aws": Uses DynamoDB for storage
//   - "gcp": (future) E.g. using Google Cloud Run and Firestore for storage
func Initialize(ctx context.Context, provider constants.BackendProvider, cfg *config.Env) (*Service, error) {
	log.Printf("→ Initializing service for backend provider: %s", provider)

	var (
		userRepo database.UserRepository
		err      error
	)

	switch provider {
	case constants.AWS:
		userRepo, err = initializeAWSBackend(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", provider, constants.AWS)
	}

	log.Printf("→ Service initialized successfully for backend provider: %s", provider)

	return NewService(userRepo), nil
}

// initializeAWSBackend sets up AWS-specific dependencies
func initializeAWSBackend(ctx context.Context, cfg *config.Env) (database.UserRepository, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	log.Printf("→ Pointing to DynamoDB table: %s", cfg.APIKeysTable)

	return dynamorepo.NewUserRepository(dynamoClient, cfg.APIKeysTable), nil
}
