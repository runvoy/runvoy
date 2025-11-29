// seed-admin-user is a utility script to seed the admin user into the database.
// This script is intentionally kept for operational purposes (initial setup, recovery, etc.).
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/auth"
	"github.com/runvoy/runvoy/internal/config"
	"github.com/runvoy/runvoy/internal/constants"
	"github.com/runvoy/runvoy/internal/providers/aws/database/dynamodb"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func setupAPIKeyAndConfig() (cfg *config.Config, apiKey, apiKeyHash string) {
	var err error
	apiKey, err = auth.GenerateSecretToken()
	if err != nil {
		log.Fatalf("error: failed to generate API key: %v", err)
	}

	cfg, err = config.Load()
	if err != nil {
		cfg = &config.Config{
			APIKey:      apiKey,
			APIEndpoint: "",
		}
	} else {
		cfg.APIKey = apiKey
	}

	apiKeyHash = auth.HashAPIKey(apiKey)
	return cfg, apiKey, apiKeyHash
}

func seedAdminUser(ctx context.Context, dynamoClient *awsdynamodb.Client, tableName, adminEmail, apiKeyHash string) {
	// Create a UserRepository instance
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	repo := dynamodb.NewUserRepository(dynamoClient, tableName, "", logger)

	// Check if user already exists
	existingUser, err := repo.GetUserByEmail(ctx, adminEmail)
	if err != nil {
		log.Fatalf("error: failed to check if admin user exists: %v", err)
	}
	if existingUser != nil {
		log.Fatalf("error: admin user %s already exists in DynamoDB", adminEmail)
	}

	// Create user struct
	user := &api.User{
		Email:     adminEmail,
		Role:      "admin",
		CreatedAt: time.Now().UTC(),
		Revoked:   false,
	}

	log.Printf("seeding admin user %s into table %s...", adminEmail, tableName)

	// Use UserRepository's CreateUser method (expiresAtUnix = 0 for permanent user)
	err = repo.CreateUser(ctx, user, apiKeyHash, 0)
	if err != nil {
		log.Fatalf("error: failed to seed admin user: %v", err)
	}

	log.Println("admin user created in DynamoDB")
}

func main() {
	if len(os.Args) != constants.ExpectedArgsSeedAdminUser {
		log.Fatalf("error: usage: %s <admin-email> <stack-name>", os.Args[0])
	}

	adminEmail := os.Args[1]
	stackName := os.Args[2]
	if adminEmail == "" || stackName == "" {
		log.Fatalf("error: admin email and stack name are required")
	}

	cfg, _, apiKeyHash := setupAPIKeyAndConfig()

	ctx, cancel := context.WithTimeout(context.Background(), constants.ScriptContextTimeout)
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	cancel()
	if err != nil {
		log.Fatalf("error: failed to load AWS configuration: %v", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), constants.ScriptContextTimeout)
	cfnClient := cloudformation.NewFromConfig(awsCfg)
	tableName, err := getTableNameFromStack(ctx2, cfnClient, stackName)
	cancel2()
	if err != nil {
		log.Fatalf("error: failed to resolve API keys table name from CloudFormation outputs: %v", err)
	}

	dynamoClient := awsdynamodb.NewFromConfig(awsCfg)
	seedAdminUser(context.Background(), dynamoClient, tableName, adminEmail, apiKeyHash)

	if err = config.Save(cfg); err != nil {
		log.Fatalf(
			"error: failed to save config file: %v. "+
				"Please save the key manually or store it somewhere safe: %s",
			err, cfg.APIKey,
		)
	}
	log.Println("config file saved")
}

func getTableNameFromStack(ctx context.Context, client *cloudformation.Client, stackName string) (string, error) {
	output, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe stack: %w", err)
	}

	if len(output.Stacks) == 0 {
		return "", fmt.Errorf("stack %s not found", stackName)
	}

	stack := output.Stacks[0]
	for _, out := range stack.Outputs {
		if out.OutputKey != nil && *out.OutputKey == "APIKeysTableName" {
			if out.OutputValue != nil {
				return *out.OutputValue, nil
			}
		}
	}

	return "", fmt.Errorf("APIKeysTableName output not found in stack %s", stackName)
}
