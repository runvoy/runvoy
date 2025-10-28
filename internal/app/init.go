package app

import (
	"context"
	"fmt"
	"log/slog"

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
func Initialize(ctx context.Context, provider constants.BackendProvider, cfg *config.Env, logger *slog.Logger) (*Service, error) {
	logger.Debug("Initializing service", "provider", provider)

	var (
		userRepo database.UserRepository
		err      error
	)

	switch provider {
	case constants.AWS:
		userRepo, err = initializeAWSBackend(ctx, cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", provider, constants.AWS)
	}

	logger.Debug("Service initialized successfully", "provider", provider)

	return NewService(userRepo, logger), nil
}

// initializeAWSBackend sets up AWS-specific dependencies
func initializeAWSBackend(ctx context.Context, cfg *config.Env, logger *slog.Logger) (database.UserRepository, error) {
	if cfg.APIKeysTable == "" {
		return nil, fmt.Errorf("APIKeysTable cannot be empty")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	logger.Debug("Using DynamoDB backend", "table", cfg.APIKeysTable)

	return dynamorepo.NewUserRepository(dynamoClient, cfg.APIKeysTable, logger), nil
}
