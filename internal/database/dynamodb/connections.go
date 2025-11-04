package dynamodb

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/database"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ConnectionRepository implements the database.ConnectionRepository interface using DynamoDB.
type ConnectionRepository struct {
	client    *dynamodb.Client
	tableName string
	logger    *slog.Logger
}

// NewConnectionRepository creates a new DynamoDB-backed connection repository.
func NewConnectionRepository(
	client *dynamodb.Client,
	tableName string,
	log *slog.Logger,
) database.ConnectionRepository {
	return &ConnectionRepository{
		client:    client,
		tableName: tableName,
		logger:    log,
	}
}

// connectionItem represents the structure stored in DynamoDB.
type connectionItem struct {
	ConnectionID string `dynamodbav:"connection_id"`
	ExecutionID  string `dynamodbav:"execution_id"`
	ExpiresAt    int64  `dynamodbav:"expires_at"`
}

// CreateConnection stores a new WebSocket connection record in DynamoDB.
func (r *ConnectionRepository) CreateConnection(
	ctx context.Context,
	connectionID, executionID string,
	expiresAt int64,
) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	item := connectionItem{
		ConnectionID: connectionID,
		ExecutionID:  executionID,
		ExpiresAt:    expiresAt,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return apperrors.ErrDatabaseError("failed to marshal connection item", err)
	}

	logArgs := []any{
		"operation", "DynamoDB.PutItem",
		"table", r.tableName,
		"connectionID", connectionID,
		"executionID", executionID,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	})
	if err != nil {
		return apperrors.ErrDatabaseError("failed to store connection", err)
	}

	reqLogger.Debug("connection stored successfully", "connectionID", connectionID)
	return nil
}

// DeleteConnection removes a WebSocket connection from DynamoDB by connection ID.
func (r *ConnectionRepository) DeleteConnection(ctx context.Context, connectionID string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	key := map[string]interface{}{
		"connection_id": connectionID,
	}

	keyAV, err := attributevalue.MarshalMap(key)
	if err != nil {
		return apperrors.ErrDatabaseError("failed to marshal connection key", err)
	}

	logArgs := []any{
		"operation", "DynamoDB.DeleteItem",
		"table", r.tableName,
		"connectionID", connectionID,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err = r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       keyAV,
	})
	if err != nil {
		return apperrors.ErrDatabaseError("failed to delete connection", err)
	}

	reqLogger.Debug("connection deleted successfully", "connectionID", connectionID)
	return nil
}

// GetConnectionsByExecutionID retrieves all active WebSocket connections for a given execution ID
// using the execution_id-index GSI.
func (r *ConnectionRepository) GetConnectionsByExecutionID(
	ctx context.Context,
	executionID string,
) ([]string, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"index", "execution_id-index",
		"executionID", executionID,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("execution_id-index"),
		KeyConditionExpression: aws.String("execution_id = :execution_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":execution_id": &types.AttributeValueMemberS{Value: executionID},
		},
	})
	if err != nil {
		return nil, apperrors.ErrDatabaseError("failed to query connections by execution ID", err)
	}

	if len(result.Items) == 0 {
		reqLogger.Debug("no connections found for execution", "executionID", executionID)
		return []string{}, nil
	}

	connectionIDs := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		var connItem connectionItem
		if unmarshalErr := attributevalue.UnmarshalMap(item, &connItem); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to unmarshal connection item: %w", unmarshalErr)
		}
		connectionIDs = append(connectionIDs, connItem.ConnectionID)
	}

	reqLogger.Debug("connections retrieved successfully",
		"executionID", executionID,
		"count", len(connectionIDs),
	)

	return connectionIDs, nil
}
