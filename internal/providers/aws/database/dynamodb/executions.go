// Package dynamodb implements DynamoDB-based storage for runvoy.
// It provides persistence for execution records using AWS DynamoDB.
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ExecutionRepository implements the database.ExecutionRepository interface using DynamoDB.
type ExecutionRepository struct {
	client    *dynamodb.Client
	tableName string
	logger    *slog.Logger
}

// NewExecutionRepository creates a new DynamoDB-backed execution repository.
func NewExecutionRepository(client *dynamodb.Client, tableName string, log *slog.Logger) *ExecutionRepository {
	return &ExecutionRepository{
		client:    client,
		tableName: tableName,
		logger:    log,
	}
}

// executionItem represents the structure stored in DynamoDB.
// This keeps the database schema separate from the API types.
// StartedAt is stored as a Unix timestamp (number) as the sort key to avoid timestamp serialization issues.
// CompletedAt is also stored as a Unix timestamp (number) to maintain consistency.
type executionItem struct {
	ExecutionID     string `dynamodbav:"execution_id"`
	StartedAt       int64  `dynamodbav:"started_at"`
	UserEmail       string `dynamodbav:"user_email"`
	Command         string `dynamodbav:"command"`
	Status          string `dynamodbav:"status"`
	CompletedAt     *int64 `dynamodbav:"completed_at,omitempty"`
	ExitCode        int    `dynamodbav:"exit_code,omitempty"`
	DurationSecs    int    `dynamodbav:"duration_seconds,omitempty"`
	LogStreamName   string `dynamodbav:"log_stream_name,omitempty"`
	RequestID       string `dynamodbav:"request_id,omitempty"`
	ComputePlatform string `dynamodbav:"compute_platform,omitempty"`
}

// toExecutionItem converts an api.Execution to an executionItem.
func toExecutionItem(e *api.Execution) *executionItem {
	item := &executionItem{
		ExecutionID:     e.ExecutionID,
		StartedAt:       e.StartedAt.Unix(),
		UserEmail:       e.UserEmail,
		Command:         e.Command,
		Status:          e.Status,
		ExitCode:        e.ExitCode,
		DurationSecs:    e.DurationSeconds,
		LogStreamName:   e.LogStreamName,
		RequestID:       e.RequestID,
		ComputePlatform: e.ComputePlatform,
	}
	if e.CompletedAt != nil {
		completedAt := e.CompletedAt.Unix()
		item.CompletedAt = &completedAt
	}
	return item
}

// toAPIExecution converts an executionItem to an api.Execution.
func (e *executionItem) toAPIExecution() *api.Execution {
	exec := &api.Execution{
		ExecutionID:     e.ExecutionID,
		StartedAt:       time.Unix(e.StartedAt, 0).UTC(),
		UserEmail:       e.UserEmail,
		Command:         e.Command,
		Status:          e.Status,
		ExitCode:        e.ExitCode,
		DurationSeconds: e.DurationSecs,
		LogStreamName:   e.LogStreamName,
		ComputePlatform: e.ComputePlatform,
	}
	if e.CompletedAt != nil {
		completedAt := time.Unix(*e.CompletedAt, 0).UTC()
		exec.CompletedAt = &completedAt
	}
	return exec
}

// CreateExecution stores a new execution record in DynamoDB.
func (r *ExecutionRepository) CreateExecution(ctx context.Context, execution *api.Execution) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	item := toExecutionItem(execution)

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return apperrors.ErrDatabaseError("failed to marshal execution", err)
	}

	reqLogger.Debug("calling external service", "context", map[string]string{
		"operation":    "DynamoDB.PutItem",
		"table":        r.tableName,
		"execution_id": execution.ExecutionID,
		"user_email":   execution.UserEmail,
		"status":       execution.Status,
	})

	// Ensure uniqueness: only create if this PK does not already exist
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
		// This condition prevents overwriting an existing item with the same
		// composite key (execution_id, started_at). If an item exists, DynamoDB
		// returns a ConditionalCheckFailedException, which we map to a conflict.
		ConditionExpression: aws.String("attribute_not_exists(execution_id) AND attribute_not_exists(started_at)"),
	})

	if err != nil {
		// If the condition failed, surface a conflict indicating duplicate execution ID
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return apperrors.ErrConflict("execution already exists", err)
		}
		return apperrors.ErrDatabaseError("failed to create execution", err)
	}

	reqLogger.Debug("execution stored successfully", "execution_id", execution.ExecutionID)

	return nil
}

// GetExecution retrieves an execution by its execution ID.
func (r *ExecutionRepository) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB Query
	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"execution_id", executionID,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		ConsistentRead:         aws.Bool(true), // Use strongly consistent read to ensure we see the latest data
		KeyConditionExpression: aws.String("execution_id = :execution_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":execution_id": &types.AttributeValueMemberS{Value: executionID},
		},
		ScanIndexForward: aws.Bool(false), // sort descending by started_at
		Limit:            aws.Int32(1),
	})

	if err != nil {
		return nil, apperrors.ErrDatabaseError("failed to query execution", err)
	}

	if len(result.Items) == 0 {
		return nil, nil
	}

	var item executionItem
	if err = attributevalue.UnmarshalMap(result.Items[0], &item); err != nil {
		return nil, apperrors.ErrDatabaseError("failed to unmarshal execution", err)
	}

	return item.toAPIExecution(), nil
}

// buildUpdateExpression builds a DynamoDB update expression for an execution.
func buildUpdateExpression(
	execution *api.Execution,
) (updateExpr string, exprNames map[string]string, exprValues map[string]types.AttributeValue) {
	updateExpr = "SET #status = :status"
	exprNames = map[string]string{
		"#status": "status",
	}
	exprAttrValues := map[string]types.AttributeValue{
		":status": &types.AttributeValueMemberS{Value: execution.Status},
	}

	if execution.CompletedAt != nil {
		updateExpr += ", completed_at = :completed_at"
		completedAt := execution.CompletedAt.Unix()
		exprAttrValues[":completed_at"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", completedAt)}
	}

	updateExpr += ", exit_code = :exit_code"
	exprAttrValues[":exit_code"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", execution.ExitCode)}

	if execution.DurationSeconds > 0 {
		updateExpr += ", duration_seconds = :duration_seconds"
		exprAttrValues[":duration_seconds"] = &types.AttributeValueMemberN{
			Value: fmt.Sprintf("%d", execution.DurationSeconds)}
	}

	if execution.LogStreamName != "" {
		updateExpr += ", log_stream_name = :log_stream_name"
		exprAttrValues[":log_stream_name"] = &types.AttributeValueMemberS{Value: execution.LogStreamName}
	}

	return updateExpr, exprNames, exprAttrValues
}

// UpdateExecution updates an existing execution record.
func (r *ExecutionRepository) UpdateExecution(ctx context.Context, execution *api.Execution) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	const conditionExpr = "attribute_exists(execution_id) AND attribute_exists(started_at)"
	startedAtStr := fmt.Sprintf("%d", execution.StartedAt.Unix()) // DynamoDB requires Number values as strings

	updateExpr, exprNames, exprValues := buildUpdateExpression(execution)

	updateLogArgs := []any{
		"operation", "DynamoDB.UpdateItem",
		"table", r.tableName,
		"execution_id", execution.ExecutionID,
		"status", execution.Status,
		"update_expression", updateExpr,
	}
	updateLogArgs = append(updateLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(updateLogArgs))

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"execution_id": &types.AttributeValueMemberS{Value: execution.ExecutionID},
			"started_at":   &types.AttributeValueMemberN{Value: startedAtStr},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprValues,
		ConditionExpression:       aws.String(conditionExpr),
	}

	_, updateErr := r.client.UpdateItem(ctx, input)

	if updateErr != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(updateErr, &ccfe) {
			return apperrors.ErrNotFound("execution not found", updateErr)
		}
		reqLogger.Error("update item failed", "context", map[string]any{
			"error":        updateErr.Error(),
			"execution_id": execution.ExecutionID,
			"started_at":   startedAtStr,
		})
		return apperrors.ErrDatabaseError("failed to update execution", updateErr)
	}

	return nil
}

// ListExecutions scans the executions table to return all execution records.
// Results are sorted by StartedAt descending in-memory to provide a reasonable default ordering.
func (r *ExecutionRepository) ListExecutions(ctx context.Context) ([]*api.Execution, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)
	executions := make([]*api.Execution, 0, constants.ExecutionsSliceInitialCapacity)
	var lastKey map[string]types.AttributeValue
	pageCount := 0

	reqLogger.Debug("calling external service", "context", map[string]string{
		"operation": "DynamoDB.Scan",
		"table":     r.tableName,
		"paginated": "true",
	})

	for {
		pageCount++

		out, err := r.client.Scan(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(r.tableName),
			ExclusiveStartKey: lastKey,
		})
		if err != nil {
			return nil, apperrors.ErrDatabaseError("failed to scan executions", err)
		}

		for _, it := range out.Items {
			var item executionItem
			if err = attributevalue.UnmarshalMap(it, &item); err != nil {
				return nil, apperrors.ErrDatabaseError("failed to unmarshal execution", err)
			}
			executions = append(executions, item.toAPIExecution())
		}

		if len(out.LastEvaluatedKey) == 0 {
			break
		}
		lastKey = out.LastEvaluatedKey
	}

	// Sort by StartedAt descending (newest first)
	slices.SortFunc(executions, func(a, b *api.Execution) int {
		if a.StartedAt.Equal(b.StartedAt) {
			return 0
		}
		if a.StartedAt.After(b.StartedAt) {
			return -1
		}
		return 1
	})

	return executions, nil
}
