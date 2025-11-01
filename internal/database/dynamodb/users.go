package dynamodb

import (
	"context"
	stderrors "errors"
	"log/slog"
	"time"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"

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
	ExpiresAt  int64     `dynamodbav:"expires_at,omitempty"` // Unix timestamp for TTL
}

// CreateUser stores a new user with their hashed API key in DynamoDB.
func (r *UserRepository) CreateUser(ctx context.Context, user *api.User, apiKeyHash string) error {
	return r.CreateUserWithExpiration(ctx, user, apiKeyHash, 0)
}

// CreateUserWithExpiration stores a new user with their hashed API key and optional TTL in DynamoDB.
// If expiresAtUnix is 0, no TTL is set. If expiresAtUnix is > 0, it sets the expires_at field for automatic deletion.
func (r *UserRepository) CreateUserWithExpiration(ctx context.Context, user *api.User, apiKeyHash string, expiresAtUnix int64) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Create the item to store
	item := userItem{
		APIKeyHash: apiKeyHash,
		UserEmail:  user.Email,
		CreatedAt:  user.CreatedAt,
		Revoked:    false,
	}

	// Only set ExpiresAt if provided
	if expiresAtUnix > 0 {
		item.ExpiresAt = expiresAtUnix
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}

	// Log before calling DynamoDB PutItem
	logArgs := []any{
		"operation", "DynamoDB.PutItem",
		"table", r.tableName,
		"userEmail", user.Email,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	// Use ConditionExpression to ensure we don't overwrite existing users
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(api_key_hash)"),
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if stderrors.As(err, &ccf) {
			return apperrors.ErrConflict("user with this API key already exists", nil)
		}
		return apperrors.ErrDatabaseError("failed to create user", err)
	}

	return nil
}

// GetUserByEmail retrieves a user by their email using the GSI.
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*api.User, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB Query
	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"index", "user_email-index",
		"email", email,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("user_email-index"),
		KeyConditionExpression: aws.String("user_email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		return nil, apperrors.ErrDatabaseError("failed to query user by email", err)
	}

	if len(result.Items) == 0 {
		reqLogger.Debug("user not found", "email", email)

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
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB GetItem
	logArgs := []any{
		"operation", "DynamoDB.GetItem",
		"table", r.tableName,
		"apiKeyHash", apiKeyHash,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"api_key_hash": &types.AttributeValueMemberS{Value: apiKeyHash},
		},
	})

	if err != nil {
		reqLogger.Debug("failed to get user by API key hash", "error", err)

		return nil, apperrors.ErrDatabaseError("failed to get user by API key hash", err)
	}

	if result.Item == nil {
		reqLogger.Debug("user not found", "apiKeyHash", apiKeyHash)

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
func (r *UserRepository) UpdateLastUsed(ctx context.Context, email string) (*time.Time, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB Query
	queryLogArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"index", "user_email-index",
		"email", email,
		"purpose", "last_used_update",
	}
	queryLogArgs = append(queryLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(queryLogArgs))

	// Query the GSI by email to obtain the api_key_hash (table PK)
	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("user_email-index"),
		KeyConditionExpression: aws.String("user_email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, apperrors.ErrDatabaseError("failed to query user by email for last_used update", err)
	}

	if len(result.Items) == 0 {
		return nil, apperrors.ErrNotFound("user not found", nil)
	}

	// Extract api_key_hash from the first (and only) item for this email
	var apiKeyHash string
	if v, ok := result.Items[0]["api_key_hash"]; ok {
		if s, ok := v.(*types.AttributeValueMemberS); ok {
			apiKeyHash = s.Value
		}
	}
	if apiKeyHash == "" {
		return nil, apperrors.ErrDatabaseError("user record missing api_key_hash attribute", nil)
	}

	now := time.Now().UTC()

	// Log before calling DynamoDB UpdateItem
	updateLogArgs := []any{
		"operation", "DynamoDB.UpdateItem",
		"table", r.tableName,
		"email", email,
		"apiKeyHash", apiKeyHash,
	}
	updateLogArgs = append(updateLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(updateLogArgs))

	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"api_key_hash": &types.AttributeValueMemberS{Value: apiKeyHash},
		},
		UpdateExpression: aws.String("SET last_used = :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now": &types.AttributeValueMemberS{
				Value: now.Format(time.RFC3339Nano)}},
	})
	if err != nil {
		return nil, apperrors.ErrDatabaseError("failed to update last_used", err)
	}

	return &now, nil
}

// RevokeUser marks a user's API key as revoked.
func (r *UserRepository) RevokeUser(ctx context.Context, email string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB Query
	queryLogArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"index", "user_email-index",
		"email", email,
		"purpose", "revoke_user",
	}
	queryLogArgs = append(queryLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(queryLogArgs))

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
		return apperrors.ErrDatabaseError("failed to query user by email for revocation", err)
	}

	if len(result.Items) == 0 {
		return apperrors.ErrNotFound("user not found", nil)
	}

	// Extract the api_key_hash to use as the primary key for update
	var apiKeyHash string
	if v, ok := result.Items[0]["api_key_hash"]; ok {
		if s, ok := v.(*types.AttributeValueMemberS); ok {
			apiKeyHash = s.Value
		}
	}

	// Log before calling DynamoDB UpdateItem
	updateLogArgs := []any{
		"operation", "DynamoDB.UpdateItem",
		"table", r.tableName,
		"email", email,
		"apiKeyHash", apiKeyHash,
		"action", "revoke",
	}
	updateLogArgs = append(updateLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(updateLogArgs))

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

	if err != nil {
		return apperrors.ErrDatabaseError("failed to revoke user", err)
	}

	return nil
}

// RemoveExpiration removes the expires_at field from a user record, making them permanent.
func (r *UserRepository) RemoveExpiration(ctx context.Context, email string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// First, get the user's api_key_hash
	queryLogArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"index", "user_email-index",
		"email", email,
		"purpose", "remove_expiration",
	}
	queryLogArgs = append(queryLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(queryLogArgs))

	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("user_email-index"),
		KeyConditionExpression: aws.String("user_email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return apperrors.ErrDatabaseError("failed to query user by email for expiration removal", err)
	}

	if len(result.Items) == 0 {
		return apperrors.ErrNotFound("user not found", nil)
	}

	var apiKeyHash string
	if v, ok := result.Items[0]["api_key_hash"]; ok {
		if s, ok := v.(*types.AttributeValueMemberS); ok {
			apiKeyHash = s.Value
		}
	}

	// Log before calling DynamoDB UpdateItem
	updateLogArgs := []any{
		"operation", "DynamoDB.UpdateItem",
		"table", r.tableName,
		"email", email,
		"apiKeyHash", apiKeyHash,
		"action", "remove_expiration",
	}
	updateLogArgs = append(updateLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(updateLogArgs))

	// Remove the expires_at attribute
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"api_key_hash": &types.AttributeValueMemberS{Value: apiKeyHash},
		},
		UpdateExpression: aws.String("REMOVE expires_at"),
	})

	if err != nil {
		return apperrors.ErrDatabaseError("failed to remove expiration", err)
	}

	return nil
}
