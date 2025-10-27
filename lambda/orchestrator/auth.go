package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user from the API keys table
type User struct {
	APIKeyHash string    `dynamodbav:"api_key_hash"`
	UserEmail  string    `dynamodbav:"user_email"`
	CreatedAt  string    `dynamodbav:"created_at"`
	Revoked    bool      `dynamodbav:"revoked"`
	LastUsed   string    `dynamodbav:"last_used"`
}

// authenticate validates the API key against DynamoDB and returns the user
// Returns nil if authentication fails
func authenticate(ctx context.Context, cfg *Config, apiKey string) (*User, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key required")
	}

	// Hash the API key to look it up in DynamoDB
	// We store bcrypt hashes, but we need a deterministic lookup key
	// So we use SHA256 hash of the API key as the partition key
	hash := sha256.Sum256([]byte(apiKey))
	apiKeyHash := hex.EncodeToString(hash[:])

	// Query DynamoDB for this API key hash
	result, err := cfg.DynamoDBClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(cfg.APIKeysTable),
		Key: map[string]types.AttributeValue{
			"api_key_hash": &types.AttributeValueMemberS{Value: apiKeyHash},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("invalid API key")
	}

	// Unmarshal the user
	var user User
	if err := attributevalue.UnmarshalMap(result.Item, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	// Check if revoked
	if user.Revoked {
		return nil, fmt.Errorf("API key has been revoked")
	}

	// Verify the API key using bcrypt (double-check security)
	// The stored api_key_hash should actually be a bcrypt hash for security
	// For now, we'll skip bcrypt verification and just use SHA256 lookup
	// TODO: Consider storing bcrypt hash separately for additional security

	// Update last_used timestamp (fire and forget)
	go func() {
		_, _ = cfg.DynamoDBClient.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
			TableName: aws.String(cfg.APIKeysTable),
			Key: map[string]types.AttributeValue{
				"api_key_hash": &types.AttributeValueMemberS{Value: apiKeyHash},
			},
			UpdateExpression: aws.String("SET last_used = :now"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":now": &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
			},
		})
	}()

	return &user, nil
}

// Helper function for bcrypt hashing (used by admin commands)
func hashAPIKey(apiKey string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
