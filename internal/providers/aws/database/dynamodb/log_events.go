package dynamodb

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/api"
	"runvoy/internal/database"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// LogEventRepository implements database.LogEventRepository using DynamoDB.
type LogEventRepository struct {
	client    Client
	tableName string
	logger    *slog.Logger
}

// NewLogEventRepository constructs a new repository for storing execution log events.
func NewLogEventRepository(client Client, tableName string, log *slog.Logger) database.LogEventRepository {
	return &LogEventRepository{
		client:    client,
		tableName: tableName,
		logger:    log,
	}
}

type logEventItem struct {
	ExecutionID string `dynamodbav:"execution_id"`
	EventKey    string `dynamodbav:"event_key"`
	EventID     string `dynamodbav:"event_id"`
	Timestamp   int64  `dynamodbav:"timestamp"`
	Message     string `dynamodbav:"message"`
}

// SaveLogEvents writes all provided log events for an execution.
func (r *LogEventRepository) SaveLogEvents(ctx context.Context, executionID string, logEvents []api.LogEvent) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	if executionID == "" {
		return fmt.Errorf("execution ID is required")
	}

	if len(logEvents) == 0 {
		return nil
	}

	requests := make([]types.WriteRequest, 0, len(logEvents))
	for i, event := range logEvents {
		item := &logEventItem{
			ExecutionID: executionID,
			EventKey:    buildEventKey(event, i),
			EventID:     event.EventID,
			Timestamp:   event.Timestamp,
			Message:     event.Message,
		}

		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			return appErrors.ErrDatabaseError("failed to marshal log event", err)
		}

		requests = append(requests, types.WriteRequest{
			PutRequest: &types.PutRequest{Item: av},
		})
	}

	if err := r.batchWrite(ctx, requests); err != nil {
		return err
	}

	reqLogger.Debug("log events stored", "context", map[string]any{
		"execution_id": executionID,
		"event_count":  len(logEvents),
	})

	return nil
}

// DeleteLogEvents removes all stored events for an execution.
func (r *LogEventRepository) DeleteLogEvents(ctx context.Context, executionID string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	if executionID == "" {
		return fmt.Errorf("execution ID is required")
	}

	exprValues := map[string]types.AttributeValue{
		":execution_id": &types.AttributeValueMemberS{Value: executionID},
	}

	var startKey map[string]types.AttributeValue

	for {
		queryOutput, err := r.client.Query(ctx, &dynamodb.QueryInput{
			TableName:                 aws.String(r.tableName),
			KeyConditionExpression:    aws.String("execution_id = :execution_id"),
			ExpressionAttributeValues: exprValues,
			ProjectionExpression:      aws.String("execution_id, event_key"),
			ExclusiveStartKey:         startKey,
		})
		if err != nil {
			return appErrors.ErrDatabaseError("failed to query log events for deletion", err)
		}

		if len(queryOutput.Items) == 0 {
			return nil
		}

		deleteRequests := make([]types.WriteRequest, 0, len(queryOutput.Items))
		for _, item := range queryOutput.Items {
			deleteRequests = append(deleteRequests, types.WriteRequest{
				DeleteRequest: &types.DeleteRequest{Key: item},
			})
		}

		if err := r.batchWrite(ctx, deleteRequests); err != nil {
			return err
		}

		reqLogger.Debug("log events deleted", "context", map[string]any{
			"execution_id":  executionID,
			"deleted_count": len(deleteRequests),
		})

		if len(queryOutput.LastEvaluatedKey) == 0 {
			return nil
		}

		startKey = queryOutput.LastEvaluatedKey
	}
}

func (r *LogEventRepository) batchWrite(ctx context.Context, requests []types.WriteRequest) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	const batchSize = 25
	for i := 0; i < len(requests); i += batchSize {
		end := min(i+batchSize, len(requests))
		batch := requests[i:end]

		logArgs := []any{
			"operation", "DynamoDB.BatchWriteItem",
			"table", r.tableName,
			"request_count", len(batch),
		}
		logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

		_, err := r.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{r.tableName: batch},
		})
		if err != nil {
			return appErrors.ErrDatabaseError("failed to write log events batch", err)
		}
	}

	return nil
}

func buildEventKey(event api.LogEvent, index int) string {
	if event.EventID != "" {
		return fmt.Sprintf("%013d#%s", event.Timestamp, event.EventID)
	}

	return fmt.Sprintf("%013d#%06d", event.Timestamp, index)
}
