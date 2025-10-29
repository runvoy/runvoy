package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"runvoy/internal/api"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// UserRepository implements the database.UserRepository interface using DynamoDB.
type UserRepository struct {
	client    *dynamodb.Client
	tableName string
	logger    *slog.Logger
}

// NewUserRepository creates a new DynamoDB-backed user repository.
func NewUserRepository(client *dynamodb.Client, tableName string, logger *slog.Logger) *UserRepository {
	return &UserRepository{
		client:    client,
		tableName: tableName,
		logger:    logger,
	}
}

// userItem represents the structure stored in DynamoDB.
// This keeps the database schema separate from the API types.
type userItem struct {
	APIKeyHash string    `dynamodbav:"api_key_hash"`
	UserEmail  string    `dynamodbav:"user_email"`
	CreatedAt  time.Time `dynamodbav:"created_at"`
	LastUsed   time.Time `dynamodbav:"last_used,omitempty"`
	Revoked    bool      `dynamodbav:"revoked"`
}

// CreateUser stores a new user with their hashed API key in DynamoDB.
func (r *UserRepository) CreateUser(ctx context.Context, user *api.User, apiKeyHash string) error {
	// Create the item to store
	item := userItem{
		APIKeyHash: apiKeyHash,
		UserEmail:  user.Email,
		CreatedAt:  user.CreatedAt,
		Revoked:    false,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}

	// Use ConditionExpression to ensure we don't overwrite existing users
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(api_key_hash)"),
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return errors.New("user with this API key already exists")
		}
		return err
	}

	return nil
}

// GetUserByEmail retrieves a user by their email using the GSI.
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*api.User, error) {
	r.logger.Debug("querying user by email", "email", email)

	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("user_email-index"),
		KeyConditionExpression: aws.String("user_email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query user by email: %w", err)
	}

	if len(result.Items) == 0 {
		r.logger.Debug("user not found", "email", email)

		return nil, nil
	}

	var item userItem
	if err := attributevalue.UnmarshalMap(result.Items[0], &item); err != nil {
		return nil, err
	}

	return &api.User{
		Email:     item.UserEmail,
		CreatedAt: item.CreatedAt,
		Revoked:   item.Revoked,
		LastUsed:  item.LastUsed,
		// Note: APIKey is intentionally omitted for security
	}, nil
}

// GetUserByAPIKeyHash retrieves a user by their hashed API key (primary key).
func (r *UserRepository) GetUserByAPIKeyHash(ctx context.Context, apiKeyHash string) (*api.User, error) {
	r.logger.Debug("querying user by API key hash", "apiKeyHash", apiKeyHash)

	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"api_key_hash": &types.AttributeValueMemberS{Value: apiKeyHash},
		},
	})

	if err != nil {
		r.logger.Debug("failed to get user by API key hash", "error", err)

		return nil, err
	}

	if result.Item == nil {
		r.logger.Debug("user not found", "apiKeyHash", apiKeyHash)

		return nil, nil
	}

	var item userItem
	if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		return nil, err
	}

	return &api.User{
		Email:     item.UserEmail,
		CreatedAt: item.CreatedAt,
		Revoked:   item.Revoked,
		LastUsed:  item.LastUsed,
	}, nil
}

// UpdateLastUsed updates the last_used timestamp for a user.
func (r *UserRepository) UpdateLastUsed(ctx context.Context, email string) error {
	// First, query to get the api_key_hash for this email
	user, err := r.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}

	// We need the api_key_hash to update, but we don't have it from the query
	// In practice, we'd store it during authentication
	// For now, we'll need to modify the signature or add a method that takes the hash
	// This is a limitation that shows we might want to query by email and update

	// Alternative approach: use GSI to find the item, then update by hash
	// For simplicity, this implementation assumes we have the hash during auth
	return errors.New("UpdateLastUsed requires refactoring to store hash during auth")
}

// RevokeUser marks a user's API key as revoked.
func (r *UserRepository) RevokeUser(ctx context.Context, email string) error {
	// Query to find the api_key_hash for this email
	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("user_email-index"),
		KeyConditionExpression: aws.String("user_email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
	})

	if err != nil {
		return err
	}

	if len(result.Items) == 0 {
		return errors.New("user not found")
	}

	// Extract the api_key_hash to use as the primary key for update
	var apiKeyHash string
	if v, ok := result.Items[0]["api_key_hash"]; ok {
		if s, ok := v.(*types.AttributeValueMemberS); ok {
			apiKeyHash = s.Value
		}
	}

	// Update the item to mark as revoked
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"api_key_hash": &types.AttributeValueMemberS{Value: apiKeyHash},
		},
		UpdateExpression: aws.String("SET revoked = :revoked"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":revoked": &types.AttributeValueMemberBOOL{Value: true},
		},
	})

	return err
}
