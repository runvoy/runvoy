package dynamodb

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// LogRepository implements the database.LogRepository interface using DynamoDB.
type LogRepository struct {
	client    *dynamodb.Client
	tableName string
	logger    *slog.Logger
}

// NewLogRepository creates a new DynamoDB-backed log repository.
func NewLogRepository(client *dynamodb.Client, tableName string, log *slog.Logger) *LogRepository {
	return &LogRepository{
		client:    client,
		tableName: tableName,
		logger:    log,
	}
}

// logItem represents the structure stored in DynamoDB.
// This keeps the database schema separate from the API types.
type logItem struct {
	ExecutionID string `dynamodbav:"execution_id"`
	LogIndex    int64  `dynamodbav:"log_index"`
	Timestamp   int64  `dynamodbav:"timestamp"`
	Message     string `dynamodbav:"message"`
	ExpiresAt   int64  `dynamodbav:"expires_at,omitempty"`
}

// counterItem represents the atomic counter for tracking max_index per execution.
type counterItem struct {
	ExecutionID string `dynamodbav:"execution_id"`
	LogIndex    int64  `dynamodbav:"log_index"` // Always 0 for counter items
	MaxIndex    int64  `dynamodbav:"max_index"`
}

// StoreLogs stores log events in DynamoDB with sequential indexes.
// Uses atomic counter to prevent race conditions when multiple forwarders process logs simultaneously.
// Returns the highest index stored.
func (r *LogRepository) StoreLogs(ctx context.Context, executionID string, events []api.LogEvent) (int64, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	if len(events) == 0 {
		return 0, nil
	}

	// Atomically reserve index range
	startIndex, err := r.reserveIndexRange(ctx, executionID, int64(len(events)))
	if err != nil {
		return 0, err
	}

	// Prepare batch write items
	writeRequests := make([]types.WriteRequest, 0, len(events))
	for i, event := range events {
		logIndex := startIndex + int64(i)
		item := logItem{
			ExecutionID: executionID,
			LogIndex:    logIndex,
			Timestamp:   event.Timestamp,
			Message:     event.Message,
		}
		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			return 0, apperrors.ErrDatabaseError("failed to marshal log item", err)
		}
		writeRequests = append(writeRequests, types.WriteRequest{
			PutRequest: &types.PutRequest{Item: av},
		})
	}

	// Batch write (handle pagination if > 25 items)
	err = r.batchWriteItems(ctx, writeRequests)
	if err != nil {
		return 0, err
	}

	maxIndex := startIndex + int64(len(events)) - 1

	reqLogger.Debug("logs stored successfully", "context", map[string]any{
		"execution_id": executionID,
		"events_count": len(events),
		"start_index":  startIndex,
		"max_index":    maxIndex,
	})

	return maxIndex, nil
}

// reserveIndexRange atomically reserves a range of sequential indexes.
// Uses DynamoDB UpdateItem with ADD operation for atomic increment.
func (r *LogRepository) reserveIndexRange(ctx context.Context, executionID string, count int64) (int64, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Use UpdateItem with ADD to atomically increment
	result, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"execution_id": &types.AttributeValueMemberS{Value: executionID},
			"log_index":    &types.AttributeValueMemberN{Value: "0"}, // Counter item
		},
		UpdateExpression: aws.String("ADD max_index :count"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":count": &types.AttributeValueMemberN{Value: strconv.FormatInt(count, 10)},
		},
		ReturnValues: types.ReturnValueUpdatedNew,
	})

	if err != nil {
		// Handle case where counter doesn't exist yet
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			// Initialize counter if it doesn't exist
			return r.initializeCounter(ctx, executionID, count)
		}
		return 0, apperrors.ErrDatabaseError("failed to reserve index range", err)
	}

	// Extract new max_index
	maxIndexAttr, ok := result.Attributes["max_index"]
	if !ok {
		return 0, apperrors.ErrDatabaseError("counter item missing max_index", nil)
	}

	maxIndexMember, ok := maxIndexAttr.(*types.AttributeValueMemberN)
	if !ok {
		return 0, apperrors.ErrDatabaseError("invalid max_index type", nil)
	}

	newMaxIndex, parseErr := strconv.ParseInt(maxIndexMember.Value, 10, 64)
	if parseErr != nil {
		return 0, apperrors.ErrDatabaseError("failed to parse max_index", parseErr)
	}

	// Return starting index (newMaxIndex - count + 1)
	startIndex := newMaxIndex - count + 1

	reqLogger.Debug("index range reserved", "context", map[string]string{
		"execution_id": executionID,
		"count":        strconv.FormatInt(count, 10),
		"start_index":  strconv.FormatInt(startIndex, 10),
		"max_index":    strconv.FormatInt(newMaxIndex, 10),
	})

	return startIndex, nil
}

// initializeCounter creates the counter item if it doesn't exist.
func (r *LogRepository) initializeCounter(ctx context.Context, executionID string, count int64) (int64, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Try to create counter with initial value
	counter := counterItem{
		ExecutionID: executionID,
		LogIndex:    0,
		MaxIndex:    count,
	}

	av, err := attributevalue.MarshalMap(counter)
	if err != nil {
		return 0, apperrors.ErrDatabaseError("failed to marshal counter item", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
		ConditionExpression: aws.String("attribute_not_exists(execution_id)"),
	})

	if err != nil {
		// Another forwarder created it, retry the update
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return r.reserveIndexRange(ctx, executionID, count)
		}
		return 0, apperrors.ErrDatabaseError("failed to initialize counter", err)
	}

	reqLogger.Debug("counter initialized", "context", map[string]string{
		"execution_id": executionID,
		"max_index":    strconv.FormatInt(count, 10),
	})

	// Return starting index (count - count + 1 = 1)
	return 1, nil
}

// GetLogsSinceIndex retrieves logs starting from a specific index (exclusive).
// Returns logs sorted by log_index ascending.
func (r *LogRepository) GetLogsSinceIndex(ctx context.Context, executionID string, lastIndex int64) ([]api.LogEvent, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"execution_id", executionID,
		"last_index", lastIndex,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	var logEvents []api.LogEvent
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		queryInput := &dynamodb.QueryInput{
			TableName:              aws.String(r.tableName),
			KeyConditionExpression: aws.String("execution_id = :execution_id AND log_index > :last_index"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":execution_id": &types.AttributeValueMemberS{Value: executionID},
				":last_index":   &types.AttributeValueMemberN{Value: strconv.FormatInt(lastIndex, 10)},
			},
			ScanIndexForward: aws.Bool(true), // Sort ascending by log_index
		}

		if lastEvaluatedKey != nil {
			queryInput.ExclusiveStartKey = lastEvaluatedKey
		}

		result, err := r.client.Query(ctx, queryInput)
		if err != nil {
			return nil, apperrors.ErrDatabaseError("failed to query logs", err)
		}

		// Convert items to LogEvent
		for _, item := range result.Items {
			var logItem logItem
			if err := attributevalue.UnmarshalMap(item, &logItem); err != nil {
				return nil, apperrors.ErrDatabaseError("failed to unmarshal log item", err)
			}

			// Skip counter items (log_index = 0)
			if logItem.LogIndex == 0 {
				continue
			}

			logEvents = append(logEvents, api.LogEvent{
				Timestamp: logItem.Timestamp,
				Message:   logItem.Message,
				Index:     logItem.LogIndex,
			})
		}

		// Check if there are more items
		if result.LastEvaluatedKey == nil || len(result.LastEvaluatedKey) == 0 {
			break
		}
		lastEvaluatedKey = result.LastEvaluatedKey
	}

	reqLogger.Debug("logs retrieved successfully", "context", map[string]any{
		"execution_id": executionID,
		"last_index":   lastIndex,
		"events_count": len(logEvents),
	})

	return logEvents, nil
}

// GetMaxIndex returns the highest index for an execution (or 0 if none exist).
func (r *LogRepository) GetMaxIndex(ctx context.Context, executionID string) (int64, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"execution_id", executionID,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	// Query for counter item (log_index = 0)
	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("execution_id = :execution_id AND log_index = :log_index"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":execution_id": &types.AttributeValueMemberS{Value: executionID},
			":log_index":    &types.AttributeValueMemberN{Value: "0"},
		},
		Limit: aws.Int32(1),
	})

	if err != nil {
		return 0, apperrors.ErrDatabaseError("failed to query max index", err)
	}

	if len(result.Items) == 0 {
		// No counter item means no logs exist
		return 0, nil
	}

	// Extract max_index from counter item
	var counter counterItem
	if err := attributevalue.UnmarshalMap(result.Items[0], &counter); err != nil {
		return 0, apperrors.ErrDatabaseError("failed to unmarshal counter item", err)
	}

	reqLogger.Debug("max index retrieved", "context", map[string]string{
		"execution_id": executionID,
		"max_index":    strconv.FormatInt(counter.MaxIndex, 10),
	})

	return counter.MaxIndex, nil
}

// SetExpiration sets TTL for all logs of an execution.
// Note: This requires scanning all log items for the execution and updating them.
// For efficiency, this should be called sparingly (e.g., when execution completes).
func (r *LogRepository) SetExpiration(ctx context.Context, executionID string, expiresAt int64) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Query all log items for this execution
	var lastEvaluatedKey map[string]types.AttributeValue
	updateCount := 0

	for {
		queryInput := &dynamodb.QueryInput{
			TableName:              aws.String(r.tableName),
			KeyConditionExpression: aws.String("execution_id = :execution_id"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":execution_id": &types.AttributeValueMemberS{Value: executionID},
			},
		}

		if lastEvaluatedKey != nil {
			queryInput.ExclusiveStartKey = lastEvaluatedKey
		}

		result, err := r.client.Query(ctx, queryInput)
		if err != nil {
			return apperrors.ErrDatabaseError("failed to query logs for expiration", err)
		}

		// Update each item with expires_at
		for _, item := range result.Items {
			var logItem logItem
			if err := attributevalue.UnmarshalMap(item, &logItem); err != nil {
				return apperrors.ErrDatabaseError("failed to unmarshal log item", err)
			}

			// Skip counter items
			if logItem.LogIndex == 0 {
				continue
			}

			_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
				TableName: aws.String(r.tableName),
				Key: map[string]types.AttributeValue{
					"execution_id": &types.AttributeValueMemberS{Value: executionID},
					"log_index":    &types.AttributeValueMemberN{Value: strconv.FormatInt(logItem.LogIndex, 10)},
				},
				UpdateExpression: aws.String("SET expires_at = :expires_at"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":expires_at": &types.AttributeValueMemberN{Value: strconv.FormatInt(expiresAt, 10)},
				},
			})

			if err != nil {
				return apperrors.ErrDatabaseError("failed to set expiration", err)
			}

			updateCount++
		}

		if result.LastEvaluatedKey == nil || len(result.LastEvaluatedKey) == 0 {
			break
		}
		lastEvaluatedKey = result.LastEvaluatedKey
	}

	reqLogger.Debug("expiration set for logs", "context", map[string]any{
		"execution_id": executionID,
		"expires_at":   expiresAt,
		"updated_count": updateCount,
	})

	return nil
}

// batchWriteItems processes write requests in batches respecting DynamoDB's 25-item limit.
func (r *LogRepository) batchWriteItems(ctx context.Context, writeRequests []types.WriteRequest) error {
	const batchSize = 25

	for i := 0; i < len(writeRequests); i += batchSize {
		end := min(i+batchSize, len(writeRequests))
		batchRequests := writeRequests[i:end]

		_, err := r.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				r.tableName: batchRequests,
			},
		})

		if err != nil {
			return apperrors.ErrDatabaseError("failed to batch write logs", err)
		}
	}

	return nil
}
