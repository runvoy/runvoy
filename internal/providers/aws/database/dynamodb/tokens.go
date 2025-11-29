package dynamodb

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/database"
	appErrors "github.com/runvoy/runvoy/internal/errors"
	"github.com/runvoy/runvoy/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// TokenRepository implements the database.TokenRepository interface using DynamoDB.
type TokenRepository struct {
	client    Client
	tableName string
	logger    *slog.Logger
}

// NewTokenRepository creates a new DynamoDB-backed token repository.
func NewTokenRepository(
	client Client,
	tableName string,
	log *slog.Logger,
) database.TokenRepository {
	return &TokenRepository{
		client:    client,
		tableName: tableName,
		logger:    log,
	}
}

// tokenItem represents the structure stored in DynamoDB.
type tokenItem struct {
	Token       string `dynamodbav:"token"`
	ExecutionID string `dynamodbav:"execution_id"`
	UserEmail   string `dynamodbav:"user_email,omitempty"`
	ClientIP    string `dynamodbav:"client_ip_at_creation_time,omitempty"`
	ExpiresAt   int64  `dynamodbav:"expires_at"`
	CreatedAt   int64  `dynamodbav:"created_at"`
}

// toTokenItem converts an api.WebSocketToken to a tokenItem.
func toTokenItem(token *api.WebSocketToken) *tokenItem {
	return &tokenItem{
		Token:       token.Token,
		ExecutionID: token.ExecutionID,
		UserEmail:   token.UserEmail,
		ClientIP:    token.ClientIP,
		ExpiresAt:   token.ExpiresAt,
		CreatedAt:   token.CreatedAt,
	}
}

// CreateToken stores a new WebSocket authentication token with metadata.
func (r *TokenRepository) CreateToken(
	ctx context.Context,
	token *api.WebSocketToken,
) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	item := toTokenItem(token)

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return appErrors.ErrDatabaseError("failed to marshal token item", err)
	}

	logArgs := []any{
		"operation", "DynamoDB.PutItem",
		"table", r.tableName,
		"token", token.Token,
		"execution_id", token.ExecutionID,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	})
	if err != nil {
		return appErrors.ErrDatabaseError("failed to store token", err)
	}

	reqLogger.Debug("token stored successfully", "context", map[string]string{
		"token":        token.Token,
		"execution_id": token.ExecutionID,
	})
	return nil
}

// GetToken retrieves a token by its value.
// Returns nil if the token doesn't exist (DynamoDB TTL automatically removes expired tokens).
func (r *TokenRepository) GetToken(
	ctx context.Context,
	tokenValue string,
) (*api.WebSocketToken, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.GetItem",
		"table", r.tableName,
		"token", tokenValue,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"token": &types.AttributeValueMemberS{Value: tokenValue},
		},
	})
	if err != nil {
		return nil, appErrors.ErrDatabaseError("failed to retrieve token", err)
	}

	if result.Item == nil {
		return nil, nil // Token doesn't exist (either never existed or expired)
	}

	var item tokenItem
	if unmarshalErr := attributevalue.UnmarshalMap(result.Item, &item); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal token item: %w", unmarshalErr)
	}

	token := &api.WebSocketToken{
		Token:       item.Token,
		ExecutionID: item.ExecutionID,
		UserEmail:   item.UserEmail,
		ClientIP:    item.ClientIP,
		ExpiresAt:   item.ExpiresAt,
		CreatedAt:   item.CreatedAt,
	}

	reqLogger.Debug("token retrieved successfully", "context", map[string]string{
		"token":        token.Token,
		"execution_id": token.ExecutionID,
	})

	return token, nil
}

// DeleteToken removes a token from the database.
func (r *TokenRepository) DeleteToken(
	ctx context.Context,
	tokenValue string,
) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.DeleteItem",
		"table", r.tableName,
		"token", tokenValue,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"token": &types.AttributeValueMemberS{Value: tokenValue},
		},
	})
	if err != nil {
		return appErrors.ErrDatabaseError("failed to delete token", err)
	}

	reqLogger.Debug("token deleted successfully", "context", map[string]string{
		"token": tokenValue,
	})

	return nil
}
