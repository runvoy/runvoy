package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"time"

	"runvoy/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type userItem struct {
	APIKeyHash string    `dynamodbav:"api_key_hash"`
	UserEmail  string    `dynamodbav:"user_email"`
	CreatedAt  time.Time `dynamodbav:"created_at"`
	Revoked    bool      `dynamodbav:"revoked"`
}

func main() {
	// Get admin email from environment
	adminEmail := os.Getenv("RUNVOY_ADMIN_EMAIL")
	if adminEmail == "" {
		log.Fatalf("Error: RUNVOY_ADMIN_EMAIL environment variable not set")
	}

	// Generate a new API key
	apiKey, err := generateAPIKey()
	if err != nil {
		log.Fatalf("Error: failed to generate API key: %v", err)
	}

	// Load existing config (if it exists) to preserve api_endpoint
	cfg, err := config.Load()
	if err != nil {
		// Config doesn't exist, create a new one
		cfg = &config.Config{
			APIKey:      apiKey,
			APIEndpoint: "", // Will be empty, user can configure later
		}
	} else {
		// Config exists, update the API key but preserve the endpoint
		cfg.APIKey = apiKey
	}

	// Save the config file with the new API key
	if err := config.Save(cfg); err != nil {
		log.Fatalf("Error: failed to save config file: %v", err)
	}

	fmt.Printf("Generated API key and saved to config file\n")

	// Hash the API key with SHA256 and base64 encode
	apiKeyHash := hashAPIKey(apiKey)

	// Get CloudFormation stack name from environment (default to runvoy-backend)
	stackName := os.Getenv("RUNVOY_CLOUDFORMATION_BACKEND_STACK")
	if stackName == "" {
		stackName = "runvoy-backend"
	}

	// Initialize AWS config
	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Error: failed to load AWS configuration: %v", err)
	}

	// Get table name from CloudFormation stack outputs
	cfnClient := cloudformation.NewFromConfig(awsCfg)
	tableName, err := getTableNameFromStack(ctx, cfnClient, stackName)
	if err != nil {
		log.Fatalf("Error: failed to resolve API keys table name from CloudFormation outputs: %v", err)
	}

	// Check if admin user already exists
	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	existingUser, err := checkUserExists(ctx, dynamoClient, tableName, adminEmail)
	if err != nil {
		log.Fatalf("Error: failed to check if admin user exists: %v", err)
	}
	if existingUser {
		log.Fatalf("Error: admin user %s already exists in DynamoDB", adminEmail)
	}

	// Create the DynamoDB item
	item := userItem{
		APIKeyHash: apiKeyHash,
		UserEmail:  adminEmail,
		CreatedAt:  time.Now().UTC(),
		Revoked:    false,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		log.Fatalf("Error: failed to marshal DynamoDB item: %v", err)
	}

	// Put item with condition expression
	fmt.Printf("Seeding admin user %s into table %s...\n", adminEmail, tableName)

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(api_key_hash)"),
	})

	if err != nil {
		log.Fatalf("Error: failed to seed admin user: %v", err)
	}

	fmt.Println("Admin user created in DynamoDB.")
}

// generateAPIKey creates a cryptographically secure random API key.
// The key is base64-encoded and approximately 32 characters long.
func generateAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// hashAPIKey creates a SHA-256 hash of the API key for secure storage.
func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func checkUserExists(ctx context.Context, client *dynamodb.Client, tableName, email string) (bool, error) {
	result, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("user_email-index"),
		KeyConditionExpression: aws.String("user_email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return false, fmt.Errorf("failed to query user by email: %w", err)
	}

	return len(result.Items) > 0, nil
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
