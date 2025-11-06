package dynamodb

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/database"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	defaultLogsLimit  = 1000
	maxLogsLimit      = 10000
	dynamoDBBatchSize = 25
)

// LogsRepository implements the database.LogsRepository interface using DynamoDB.
type LogsRepository struct {
	client    *dynamodb.Client
	tableName string
	logger    *slog.Logger
	ttlDays   int
}

// NewLogsRepository creates a new DynamoDB-backed logs repository.
func NewLogsRepository(
	client *dynamodb.Client,
	tableName string,
	ttlDays int,
	log *slog.Logger,
) database.LogsRepository {
	return &LogsRepository{
		client:    client,
		tableName: tableName,
		logger:    log,
		ttlDays:   ttlDays,
	}
}

// logItem represents the structure stored in DynamoDB.
// This keeps the database schema separate from the API types.
type logItem struct {
	ExecutionID     string `dynamodbav:"execution_id"`
	TimestampLogIdx string `dynamodbav:"timestamp_log_index"` // Format: "{timestamp}#{sequence}"
	Timestamp       int64  `dynamodbav:"timestamp"`
	Message         string `dynamodbav:"message"`
	LineNumber      int    `dynamodbav:"line_number"`
	IngestedAt      int64  `dynamodbav:"ingested_at"`
	TTL             int64  `dynamodbav:"ttl"` // Unix timestamp for auto-expiration
}

// calculateTTL calculates the TTL timestamp for a log event.
func (r *LogsRepository) calculateTTL() int64 {
	return time.Now().AddDate(0, 0, r.ttlDays).Unix()
}

// getNextLineNumberAtomic atomically increments and returns the next line number
// for an execution using DynamoDB's ADD operation on a counter item.
// This ensures no two concurrent writes can get the same line number.
func (r *LogsRepository) getNextLineNumberAtomic(
	ctx context.Context,
	executionID string,
) (int, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Use a special counter item with execution_id as PK and a fixed SK for the counter
	counterSK := "LINE_NUMBER_COUNTER"

	logArgs := []any{
		"operation", "DynamoDB.UpdateItem",
		"table", r.tableName,
		"execution_id", executionID,
		"counter_sk", counterSK,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	// Atomically increment the counter and get the new value
	// The ADD operation increments the numeric attribute and returns the updated value
	result, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"execution_id":        &types.AttributeValueMemberS{Value: executionID},
			"timestamp_log_index": &types.AttributeValueMemberS{Value: counterSK},
		},
		UpdateExpression: aws.String("ADD line_number :inc SET ingested_at = :now, ttl = :ttl"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inc": &types.AttributeValueMemberN{Value: "1"},
			":now": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().UnixMilli())},
			":ttl": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", r.calculateTTL())},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		return 0, appErrors.ErrDatabaseError("failed to increment line number counter", err)
	}

	// Extract the new line_number from the result
	if result.Attributes == nil {
		return 0, appErrors.ErrDatabaseError("unexpected response from counter update", nil)
	}

	lineNumAttr, exists := result.Attributes["line_number"]
	if !exists {
		return 0, appErrors.ErrDatabaseError("line_number not found in counter response", nil)
	}

	lineNumStr := lineNumAttr.(*types.AttributeValueMemberN).Value
	var lineNumber int
	_, err = fmt.Sscanf(lineNumStr, "%d", &lineNumber)
	if err != nil {
		return 0, appErrors.ErrDatabaseError("failed to parse line number from counter", err)
	}

	reqLogger.Debug("line number assigned atomically", "context", map[string]any{
		"execution_id": executionID,
		"line_number":  lineNumber,
	})

	return lineNumber, nil
}

// CreateLogEvent stores a new log event in the cache table.
// The lineNumber is assigned atomically using DynamoDB's ADD operation on a counter.
// This ensures no two concurrent writes can be assigned the same line number.
func (r *LogsRepository) CreateLogEvent(
	ctx context.Context,
	executionID string,
	logEvent *api.LogEvent,
) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Get the next line number atomically (guarantees uniqueness across concurrent writes)
	nextLineNum, err := r.getNextLineNumberAtomic(ctx, executionID)
	if err != nil {
		return err
	}

	// Calculate timestamp#log_index composite sort key
	// Format: "{timestamp}#{sequence}" for proper sorting
	// Since we can have multiple logs at the same timestamp, we add a sequence number
	timestampLogIdx := fmt.Sprintf("%d#%d", logEvent.Timestamp, time.Now().Nanosecond())

	item := &logItem{
		ExecutionID:     executionID,
		TimestampLogIdx: timestampLogIdx,
		Timestamp:       logEvent.Timestamp,
		Message:         logEvent.Message,
		LineNumber:      nextLineNum,
		IngestedAt:      time.Now().UnixMilli(),
		TTL:             r.calculateTTL(),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return appErrors.ErrDatabaseError("failed to marshal log item", err)
	}

	logArgs := []any{
		"operation", "DynamoDB.PutItem",
		"table", r.tableName,
		"execution_id", executionID,
		"timestamp", logEvent.Timestamp,
		"line_number", nextLineNum,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	})
	if err != nil {
		return appErrors.ErrDatabaseError("failed to store log event", err)
	}

	reqLogger.Debug("log event stored successfully", "context", map[string]any{
		"execution_id": executionID,
		"line_number":  nextLineNum,
	})
	return nil
}

// buildLineNumberQuery builds a DynamoDB query input for pagination by line number.
func (r *LogsRepository) buildLineNumberQuery(
	executionID string,
	limit int,
	afterLine int,
) *dynamodb.QueryInput {
	// Ensure limit fits safely in int32
	effectiveLimit := min(limit, maxLogsLimit)
	limitInt32 := int32(effectiveLimit) //nolint:gosec // Safe: effectiveLimit <= maxLogsLimit (10000 < max int32)

	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("execution_id-line_number-index"),
		KeyConditionExpression: aws.String("execution_id = :exec_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":exec_id": &types.AttributeValueMemberS{Value: executionID},
		},
		Limit:            aws.Int32(limitInt32),
		ScanIndexForward: aws.Bool(true),
	}

	if afterLine > 0 {
		queryInput.KeyConditionExpression = aws.String("execution_id = :exec_id AND line_number > :after_line")
		queryInput.ExpressionAttributeValues[":after_line"] = &types.AttributeValueMemberN{
			Value: fmt.Sprintf("%d", afterLine),
		}
	}

	return queryInput
}

// GetLogsByExecutionID retrieves logs for a given execution ID with optional pagination.
func (r *LogsRepository) GetLogsByExecutionID(
	ctx context.Context,
	executionID string,
	limit int,
	afterLine int,
) ([]*api.LogEvent, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	if limit <= 0 {
		limit = defaultLogsLimit
	}
	if limit > maxLogsLimit {
		limit = maxLogsLimit
	}

	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"execution_id", executionID,
		"limit", limit,
		"after_line", afterLine,
		"index", "execution_id-line_number-index",
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	queryInput := r.buildLineNumberQuery(executionID, limit, afterLine)
	result, err := r.client.Query(ctx, queryInput)
	if err != nil {
		return nil, appErrors.ErrDatabaseError("failed to query logs", err)
	}

	var logItems []logItem
	err = attributevalue.UnmarshalListOfMaps(result.Items, &logItems)
	if err != nil {
		return nil, appErrors.ErrDatabaseError("failed to unmarshal log items", err)
	}

	events := make([]*api.LogEvent, len(logItems))
	for i, item := range logItems {
		events[i] = &api.LogEvent{
			Timestamp: item.Timestamp,
			Message:   item.Message,
		}
	}

	reqLogger.Debug("logs retrieved successfully", "context", map[string]any{
		"execution_id": executionID,
		"count":        len(events),
	})
	return events, nil
}

// GetLogsByTimeRange retrieves logs within a specific timestamp range for an execution.
func (r *LogsRepository) GetLogsByTimeRange(
	ctx context.Context,
	executionID string,
	afterTimestamp, beforeTimestamp int64,
) ([]*api.LogEvent, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"execution_id", executionID,
		"after_timestamp", afterTimestamp,
		"before_timestamp", beforeTimestamp,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	// Query by execution_id and filter by timestamp range
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("execution_id = :exec_id"),
		FilterExpression:       aws.String("timestamp BETWEEN :after_ts AND :before_ts"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":exec_id":   &types.AttributeValueMemberS{Value: executionID},
			":after_ts":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", afterTimestamp)},
			":before_ts": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", beforeTimestamp)},
		},
		ScanIndexForward: aws.Bool(true),
	}

	result, err := r.client.Query(ctx, queryInput)
	if err != nil {
		return nil, appErrors.ErrDatabaseError("failed to query logs by time range", err)
	}

	var logItems []logItem
	err = attributevalue.UnmarshalListOfMaps(result.Items, &logItems)
	if err != nil {
		return nil, appErrors.ErrDatabaseError("failed to unmarshal log items", err)
	}

	// Convert to api.LogEvent
	events := make([]*api.LogEvent, len(logItems))
	for i, item := range logItems {
		events[i] = &api.LogEvent{
			Timestamp: item.Timestamp,
			Message:   item.Message,
		}
	}

	return events, nil
}

// batchDeleteItems performs batch deletion of DynamoDB items.
func (r *LogsRepository) batchDeleteItems(
	ctx context.Context,
	items []map[string]types.AttributeValue,
) error {
	for i := 0; i < len(items); i += dynamoDBBatchSize {
		end := min(i+dynamoDBBatchSize, len(items))

		requests := make([]types.WriteRequest, end-i)
		for j, item := range items[i:end] {
			requests[j] = types.WriteRequest{
				DeleteRequest: &types.DeleteRequest{
					Key: item,
				},
			}
		}

		_, batchErr := r.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				r.tableName: requests,
			},
		})
		if batchErr != nil {
			return appErrors.ErrDatabaseError("failed to delete logs in batch", batchErr)
		}
	}
	return nil
}

// DeleteLogsByExecutionID removes all logs for a given execution.
func (r *LogsRepository) DeleteLogsByExecutionID(
	ctx context.Context,
	executionID string,
) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.Query+BatchWriteItem",
		"table", r.tableName,
		"execution_id", executionID,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("execution_id = :exec_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":exec_id": &types.AttributeValueMemberS{Value: executionID},
		},
		ProjectionExpression: aws.String("execution_id, timestamp_log_index"),
	}

	result, err := r.client.Query(ctx, queryInput)
	if err != nil {
		return appErrors.ErrDatabaseError("failed to query logs for deletion", err)
	}

	if len(result.Items) == 0 {
		return nil
	}

	deleteErr := r.batchDeleteItems(ctx, result.Items)
	if deleteErr != nil {
		return deleteErr
	}

	reqLogger.Debug("logs deleted successfully", "context", map[string]any{
		"execution_id": executionID,
		"count":        len(result.Items),
	})
	return nil
}

// CountLogsByExecutionID returns the total number of logs for an execution.
func (r *LogsRepository) CountLogsByExecutionID(
	ctx context.Context,
	executionID string,
) (int, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"execution_id", executionID,
		"select", "COUNT",
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("execution_id = :exec_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":exec_id": &types.AttributeValueMemberS{Value: executionID},
		},
		Select: types.SelectCount,
	}

	result, err := r.client.Query(ctx, queryInput)
	if err != nil {
		return 0, appErrors.ErrDatabaseError("failed to count logs", err)
	}

	return int(result.Count), nil
}
