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

// PendingAPIKeyRepository implements the database.PendingAPIKeyRepository interface using DynamoDB.
type PendingAPIKeyRepository struct {
	client    *dynamodb.Client
	tableName string
	logger    *slog.Logger
}

// NewPendingAPIKeyRepository creates a new DynamoDB-backed pending API key repository.
func NewPendingAPIKeyRepository(client *dynamodb.Client, tableName string, logger *slog.Logger) *PendingAPIKeyRepository {
	return &PendingAPIKeyRepository{
		client:    client,
		tableName: tableName,
		logger:    logger,
	}
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
func (r *PendingAPIKeyRepository) CreatePendingAPIKey(ctx context.Context, pending *api.PendingAPIKey) error {
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
		"table", r.tableName,
		"userEmail", pending.UserEmail,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	})

	if err != nil {
		return apperrors.ErrDatabaseError("failed to create pending API key", err)
	}

	return nil
}

// GetPendingAPIKey retrieves a pending API key by its secret token.
func (r *PendingAPIKeyRepository) GetPendingAPIKey(ctx context.Context, secretToken string) (*api.PendingAPIKey, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB GetItem
	logArgs := []any{
		"operation", "DynamoDB.GetItem",
		"table", r.tableName,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
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
func (r *PendingAPIKeyRepository) MarkAsViewed(ctx context.Context, secretToken string, ipAddress string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB UpdateItem
	logArgs := []any{
		"operation", "DynamoDB.UpdateItem",
		"table", r.tableName,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	viewedAt := time.Now().Unix()
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"secret_token": &types.AttributeValueMemberS{Value: secretToken},
		},
		UpdateExpression:    aws.String("SET viewed = :true, viewed_at = :viewedAt, viewed_from_ip = :ip"),
		ConditionExpression: aws.String("attribute_exists(secret_token) AND viewed = :false"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":true":     &types.AttributeValueMemberBOOL{Value: true},
			":false":    &types.AttributeValueMemberBOOL{Value: false},
			":viewedAt": &types.AttributeValueMemberN{Value: int64ToString(viewedAt)},
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
func (r *PendingAPIKeyRepository) DeletePendingAPIKey(ctx context.Context, secretToken string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB DeleteItem
	logArgs := []any{
		"operation", "DynamoDB.DeleteItem",
		"table", r.tableName,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"secret_token": &types.AttributeValueMemberS{Value: secretToken},
		},
	})

	if err != nil {
		return apperrors.ErrDatabaseError("failed to delete pending API key", err)
	}

	return nil
}

// Helper function to convert int64 to string for DynamoDB
func int64ToString(n int64) string {
	return fmt.Sprintf("%d", n)
}
