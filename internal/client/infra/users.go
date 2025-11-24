package infra

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/auth"
	"runvoy/internal/providers/aws/database/dynamodb"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// SeedAdminUser seeds an admin user into the database and returns the generated API key.
// This function hides the DynamoDB implementation details from callers.
func SeedAdminUser(ctx context.Context, adminEmail, region, tableName string) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is required")
	}

	apiKey, err := auth.GenerateSecretToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}

	repo, err := createUserRepository(ctx, tableName, region)
	if err != nil {
		return "", err
	}

	err = checkAndCreateUser(ctx, repo, adminEmail, auth.HashAPIKey(apiKey))
	if err != nil {
		return "", err
	}

	return apiKey, nil
}

// createUserRepository creates a DynamoDB user repository
func createUserRepository(ctx context.Context, tableName, region string) (*dynamodb.UserRepository, error) {
	var awsOpts []func(*awsconfig.LoadOptions) error
	if region != "" {
		awsOpts = append(awsOpts, awsconfig.WithRegion(region))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := awsdynamodb.NewFromConfig(awsCfg)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	repo := dynamodb.NewUserRepository(dynamoClient, tableName, "", logger)

	return repo, nil
}

// checkAndCreateUser checks if user exists and creates it if not
func checkAndCreateUser(ctx context.Context, repo *dynamodb.UserRepository, adminEmail, apiKeyHash string) error {
	existingUser, err := repo.GetUserByEmail(ctx, adminEmail)
	if err != nil {
		return fmt.Errorf("failed to check if admin user exists: %w", err)
	}
	if existingUser != nil {
		return fmt.Errorf("admin user %s already exists in database", adminEmail)
	}

	user := &api.User{
		Email:     adminEmail,
		Role:      "admin",
		CreatedAt: time.Now().UTC(),
		Revoked:   false,
		// Seeded admin user has no request ID since it's created outside the normal request flow
		CreatedByRequestID:  "",
		ModifiedByRequestID: "",
	}

	createErr := repo.CreateUser(ctx, user, apiKeyHash, 0)
	if createErr != nil {
		return fmt.Errorf("failed to seed admin user: %w", createErr)
	}

	return nil
}
