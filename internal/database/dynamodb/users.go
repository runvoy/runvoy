package dynamodb

import (
	"context"
	stderrors "errors"
	"fmt"
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
	client           *dynamodb.Client
	tableName       string
	pendingTableName string
	logger          *slog.Logger
}

// NewUserRepository creates a new DynamoDB-backed user repository.
func NewUserRepository(
    client *dynamodb.Client,
    tableName string,
    pendingTableName string,
    logger *slog.Logger,
) *UserRepository {
	return &UserRepository{
		client:           client,
		tableName:       tableName,
		pendingTableName: pendingTableName,
		logger:          logger,
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
func (r *UserRepository) CreateUserWithExpiration(
    ctx context.Context,
    user *api.User,
    apiKeyHash string,
    expiresAtUnix int64,
) error {
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

// queryAPIKeyHashByEmail queries for the api_key_hash by email.
func (r *UserRepository) queryAPIKeyHashByEmail(ctx context.Context, email, purpose string) (string, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	queryLogArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"index", "user_email-index",
		"email", email,
		"purpose", purpose,
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
		return "", apperrors.ErrDatabaseError("failed to query user by email", err)
	}

	if len(result.Items) == 0 {
		return "", apperrors.ErrNotFound("user not found", nil)
	}

	var apiKeyHash string
	if v, ok := result.Items[0]["api_key_hash"]; ok {
		if s, ok := v.(*types.AttributeValueMemberS); ok {
			apiKeyHash = s.Value
		}
	}
	if apiKeyHash == "" {
		return "", apperrors.ErrDatabaseError("user record missing api_key_hash attribute", nil)
	}

	return apiKeyHash, nil
}

// UpdateLastUsed updates the last_used timestamp for a user.
func (r *UserRepository) UpdateLastUsed(ctx context.Context, email string) (*time.Time, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	apiKeyHash, err := r.queryAPIKeyHashByEmail(ctx, email, "last_used_update")
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

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

	apiKeyHash, err := r.queryAPIKeyHashByEmail(ctx, email, "revoke_user")
	if err != nil {
		return err
	}

	updateLogArgs := []any{
		"operation", "DynamoDB.UpdateItem",
		"table", r.tableName,
		"email", email,
		"apiKeyHash", apiKeyHash,
		"action", "revoke",
	}
	updateLogArgs = append(updateLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(updateLogArgs))

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

// pendingAPIKeyItem represents the structure stored in DynamoDB.
type pendingAPIKeyItem struct {
	SecretToken  string `dynamodbav:"secret_token"`
	APIKey       string `dynamodbav:"api_key"`
	UserEmail    string `dynamodbav:"user_email"`
	CreatedBy    string `dynamodbav:"created_by"`
	CreatedAt    int64  `dynamodbav:"created_at"` // Unix timestamp
	ExpiresAt    int64  `dynamodbav:"expires_at"` // Unix timestamp for TTL
	Viewed       bool   `dynamodbav:"viewed"`
	ViewedAt     *int64 `dynamodbav:"viewed_at,omitempty"` // Unix timestamp when viewed
	ViewedFromIP string `dynamodbav:"viewed_from_ip,omitempty"`
}

// CreatePendingAPIKey stores a pending API key with a secret token.
func (r *UserRepository) CreatePendingAPIKey(ctx context.Context, pending *api.PendingAPIKey) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Create the item to store
	item := pendingAPIKeyItem{
		SecretToken: pending.SecretToken,
		APIKey:      pending.APIKey,
		UserEmail:   pending.UserEmail,
		CreatedBy:   pending.CreatedBy,
		CreatedAt:   pending.CreatedAt.Unix(),
		ExpiresAt:   pending.ExpiresAt,
		Viewed:      false,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return apperrors.ErrInternalError("failed to marshal pending API key", err)
	}

	// Log before calling DynamoDB PutItem
	logArgs := []any{
		"operation", "DynamoDB.PutItem",
		"table", r.pendingTableName,
		"userEmail", pending.UserEmail,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.pendingTableName),
		Item:      av,
	})

	if err != nil {
		return apperrors.ErrDatabaseError("failed to create pending API key", err)
	}

	return nil
}

// GetPendingAPIKey retrieves a pending API key by its secret token.
func (r *UserRepository) GetPendingAPIKey(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB GetItem
	logArgs := []any{
		"operation", "DynamoDB.GetItem",
		"table", r.pendingTableName,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.pendingTableName),
		Key: map[string]types.AttributeValue{
			"secret_token": &types.AttributeValueMemberS{Value: secretToken},
		},
	})

	if err != nil {
		return nil, apperrors.ErrDatabaseError("failed to get pending API key", err)
	}

	if result.Item == nil {
		return nil, nil
	}

	var item pendingAPIKeyItem
	if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		return nil, apperrors.ErrInternalError("failed to unmarshal pending API key", err)
	}

	// Convert back to API type
	pending := &api.PendingAPIKey{
		SecretToken:  item.SecretToken,
		APIKey:       item.APIKey,
		UserEmail:    item.UserEmail,
		CreatedBy:    item.CreatedBy,
		CreatedAt:    time.Unix(item.CreatedAt, 0),
		ExpiresAt:    item.ExpiresAt,
		Viewed:       item.Viewed,
		ViewedFromIP: item.ViewedFromIP,
	}

	// Convert ViewedAt if present
	if item.ViewedAt != nil {
		viewedAt := time.Unix(*item.ViewedAt, 0)
		pending.ViewedAt = &viewedAt
	}

	return pending, nil
}

// MarkAsViewed atomically marks a pending key as viewed with the IP address.
func (r *UserRepository) MarkAsViewed(ctx context.Context, secretToken string, ipAddress string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB UpdateItem
	logArgs := []any{
		"operation", "DynamoDB.UpdateItem",
		"table", r.pendingTableName,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	viewedAt := time.Now().Unix()
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.pendingTableName),
		Key: map[string]types.AttributeValue{
			"secret_token": &types.AttributeValueMemberS{Value: secretToken},
		},
		UpdateExpression:    aws.String("SET viewed = :true, viewed_at = :viewedAt, viewed_from_ip = :ip"),
		ConditionExpression: aws.String("attribute_exists(secret_token) AND viewed = :false"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":true":     &types.AttributeValueMemberBOOL{Value: true},
			":false":    &types.AttributeValueMemberBOOL{Value: false},
			":viewedAt": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", viewedAt)},
			":ip":       &types.AttributeValueMemberS{Value: ipAddress},
		},
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if stderrors.As(err, &ccf) {
			return apperrors.ErrConflict("pending key already viewed or does not exist", nil)
		}
		return apperrors.ErrDatabaseError("failed to mark pending key as viewed", err)
	}

	return nil
}

// DeletePendingAPIKey removes a pending API key from the database.
func (r *UserRepository) DeletePendingAPIKey(ctx context.Context, secretToken string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB DeleteItem
	logArgs := []any{
		"operation", "DynamoDB.DeleteItem",
		"table", r.pendingTableName,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.pendingTableName),
		Key: map[string]types.AttributeValue{
			"secret_token": &types.AttributeValueMemberS{Value: secretToken},
		},
	})

	if err != nil {
		return apperrors.ErrDatabaseError("failed to delete pending API key", err)
	}

	return nil
}

// ListUsers returns all users in the system (excluding API key hashes for security).
func (r *UserRepository) ListUsers(ctx context.Context) ([]*api.User, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB Scan
	logArgs := []any{
		"operation", "DynamoDB.Scan",
		"table", r.tableName,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(r.tableName),
	})
	if err != nil {
		return nil, apperrors.ErrDatabaseError("failed to list users", err)
	}

	users := make([]*api.User, 0, len(result.Items))
	for _, item := range result.Items {
		var userItem userItem
		if err := attributevalue.UnmarshalMap(item, &userItem); err != nil {
			reqLogger.Warn("failed to unmarshal user item", "error", err)
			continue
		}

		users = append(users, &api.User{
			Email:     userItem.UserEmail,
			CreatedAt: userItem.CreatedAt,
			Revoked:   userItem.Revoked,
			LastUsed:  userItem.LastUsed,
			// Note: APIKey and APIKeyHash are intentionally omitted for security
		})
	}

	return users, nil
}
