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
type executionItem struct {
	ExecutionID     string     `dynamodbav:"execution_id"`
	StartedAt       time.Time  `dynamodbav:"started_at"`
	UserEmail       string     `dynamodbav:"user_email"`
	Command         string     `dynamodbav:"command"`
	LockName        string     `dynamodbav:"lock_name,omitempty"`
	Status          string     `dynamodbav:"status"`
	CompletedAt     *time.Time `dynamodbav:"completed_at,omitempty"`
	ExitCode        int        `dynamodbav:"exit_code,omitempty"`
	DurationSecs    int        `dynamodbav:"duration_seconds,omitempty"`
	LogStreamName   string     `dynamodbav:"log_stream_name,omitempty"`
	RequestID       string     `dynamodbav:"request_id,omitempty"`
	ComputePlatform string     `dynamodbav:"compute_platform,omitempty"`
}

// toExecutionItem converts an api.Execution to an executionItem.
func toExecutionItem(e *api.Execution) *executionItem {
	return &executionItem{
		ExecutionID:     e.ExecutionID,
		StartedAt:       e.StartedAt,
		UserEmail:       e.UserEmail,
		Command:         e.Command,
		LockName:        e.LockName,
		Status:          e.Status,
		CompletedAt:     e.CompletedAt,
		ExitCode:        e.ExitCode,
		DurationSecs:    e.DurationSeconds,
		LogStreamName:   e.LogStreamName,
		RequestID:       e.RequestID,
		ComputePlatform: e.ComputePlatform,
	}
}

// toAPIExecution converts an executionItem to an api.Execution.
func (e *executionItem) toAPIExecution() *api.Execution {
	return &api.Execution{
		ExecutionID:     e.ExecutionID,
		StartedAt:       e.StartedAt,
		UserEmail:       e.UserEmail,
		Command:         e.Command,
		LockName:        e.LockName,
		Status:          e.Status,
		CompletedAt:     e.CompletedAt,
		ExitCode:        e.ExitCode,
		DurationSeconds: e.DurationSecs,
		LogStreamName:   e.LogStreamName,
		ComputePlatform: e.ComputePlatform,
	}
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
		"operation":   "DynamoDB.PutItem",
		"table":       r.tableName,
		"executionID": execution.ExecutionID,
		"userEmail":   execution.UserEmail,
		"status":      execution.Status,
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

	reqLogger.Debug("execution stored successfully", "executionID", execution.ExecutionID)

	return nil
}

// GetExecution retrieves an execution by its execution ID.
func (r *ExecutionRepository) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB Query
	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"executionID", executionID,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
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
	if err := attributevalue.UnmarshalMap(result.Items[0], &item); err != nil {
		return nil, apperrors.ErrDatabaseError("failed to unmarshal execution", err)
	}

	return item.toAPIExecution(), nil
}

// marshallTimestamp marshals a timestamp to a DynamoDB attribute value string.
func marshallTimestamp(t time.Time) (string, error) {
	timestampAV, err := attributevalue.Marshal(t)
	if err != nil {
		return "", apperrors.ErrDatabaseError("failed to marshal timestamp", err)
	}
	timestampStr, ok := timestampAV.(*types.AttributeValueMemberS)
	if !ok {
		return "", apperrors.ErrDatabaseError("timestamp is not a string attribute", nil)
	}
	return timestampStr.Value, nil
}

// buildUpdateExpression builds a DynamoDB update expression for an execution.
func buildUpdateExpression(
	execution *api.Execution,
) (updateExpr string, exprNames map[string]string, exprValues map[string]types.AttributeValue, err error) {
	updateExpr = "SET #status = :status"
	exprNames = map[string]string{
		"#status": "status",
	}
	exprAttrValues := map[string]types.AttributeValue{
		":status": &types.AttributeValueMemberS{Value: execution.Status},
	}

	if execution.CompletedAt != nil {
		updateExpr += ", completed_at = :completed_at"
		completedAtStr, err := marshallTimestamp(*execution.CompletedAt)
		if err != nil {
			return "", nil, nil, err
		}
		exprAttrValues[":completed_at"] = &types.AttributeValueMemberS{Value: completedAtStr}
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

	return updateExpr, exprNames, exprAttrValues, nil
}

// UpdateExecution updates an existing execution record.
func (r *ExecutionRepository) UpdateExecution(ctx context.Context, execution *api.Execution) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	updateExpr, exprAttrNames, exprAttrValues, err := buildUpdateExpression(execution)
	if err != nil {
		return err
	}

	startedAtStr, err := marshallTimestamp(execution.StartedAt)
	if err != nil {
		return err
	}

	updateLogArgs := []any{
		"operation", "DynamoDB.UpdateItem",
		"table", r.tableName,
		"executionID", execution.ExecutionID,
		"status", execution.Status,
		"updateExpression", updateExpr,
	}
	updateLogArgs = append(updateLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(updateLogArgs))

	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"execution_id": &types.AttributeValueMemberS{Value: execution.ExecutionID},
			"started_at":   &types.AttributeValueMemberS{Value: startedAtStr},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprAttrNames,
		ExpressionAttributeValues: exprAttrValues,
	})

	if err != nil {
		return apperrors.ErrDatabaseError("failed to update execution", err)
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
			if err := attributevalue.UnmarshalMap(it, &item); err != nil {
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
