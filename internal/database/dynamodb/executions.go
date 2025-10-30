package dynamodb

import (
	"context"
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

// ExecutionRepository implements the database.ExecutionRepository interface using DynamoDB.
type ExecutionRepository struct {
	client    *dynamodb.Client
	tableName string
	logger    *slog.Logger
}

// NewExecutionRepository creates a new DynamoDB-backed execution repository.
func NewExecutionRepository(client *dynamodb.Client, tableName string, logger *slog.Logger) *ExecutionRepository {
	return &ExecutionRepository{
		client:    client,
		tableName: tableName,
		logger:    logger,
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
	CostUSD         float64    `dynamodbav:"cost_usd,omitempty"`
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
		CostUSD:         e.CostUSD,
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
		CostUSD:         e.CostUSD,
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

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	})

	reqLogger.Debug("execution stored successfully", "executionID", execution.ExecutionID)

	if err != nil {
		return apperrors.ErrDatabaseError("failed to create execution", err)
	}

	return nil
}

// GetExecution retrieves an execution by its execution ID.
func (r *ExecutionRepository) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	// Note: We need both execution_id (hash) and started_at (range) to get an item
	// For now, we'll scan or query - this is a limitation we may need to address
	// For MVP, we'll use a simplified approach where we query by execution_id
	// But DynamoDB requires both keys, so we need to scan or use a GSI
	// Since execution_id should be unique, we'll scan with a filter (not ideal but works for MVP)

	result, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("execution_id = :execution_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":execution_id": &types.AttributeValueMemberS{Value: executionID},
		},
		Limit: aws.Int32(1),
	})

	if err != nil {
		return nil, apperrors.ErrDatabaseError("failed to get execution", err)
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

// UpdateExecution updates an existing execution record.
func (r *ExecutionRepository) UpdateExecution(ctx context.Context, execution *api.Execution) error {
	// Build update expression dynamically based on what fields are set
	updateExpr := "SET #status = :status"
	exprAttrNames := map[string]string{
		"#status": "status",
	}
	exprAttrValues := map[string]types.AttributeValue{
		":status": &types.AttributeValueMemberS{Value: execution.Status},
	}

	if execution.CompletedAt != nil {
		updateExpr += ", completed_at = :completed_at"
		// Use DynamoDB's marshaler to ensure the timestamp format matches exactly what DynamoDB expects
		completedAtAV, err := attributevalue.Marshal(*execution.CompletedAt)
		if err != nil {
			return apperrors.ErrDatabaseError("failed to marshal completed_at", err)
		}
		completedAtStr, ok := completedAtAV.(*types.AttributeValueMemberS)
		if !ok {
			return apperrors.ErrDatabaseError("completed_at is not a string attribute", nil)
		}
		exprAttrValues[":completed_at"] = &types.AttributeValueMemberS{Value: completedAtStr.Value}
	}

	if execution.ExitCode != 0 {
		updateExpr += ", exit_code = :exit_code"
		exprAttrValues[":exit_code"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", execution.ExitCode)}
	}

	if execution.DurationSeconds > 0 {
		updateExpr += ", duration_seconds = :duration_seconds"
		exprAttrValues[":duration_seconds"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", execution.DurationSeconds)}
	}

	if execution.LogStreamName != "" {
		updateExpr += ", log_stream_name = :log_stream_name"
		exprAttrValues[":log_stream_name"] = &types.AttributeValueMemberS{Value: execution.LogStreamName}
	}

	if execution.CostUSD > 0 {
		updateExpr += ", cost_usd = :cost_usd"
		exprAttrValues[":cost_usd"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", execution.CostUSD)}
	}

	// Note: DynamoDB requires both execution_id (hash) and started_at (range) keys
	// We ensure execution.StartedAt is set when creating, so it should be available here
	// Use DynamoDB's marshaler to ensure the timestamp format matches exactly what was stored
	startedAtAV, err := attributevalue.Marshal(execution.StartedAt)
	if err != nil {
		return apperrors.ErrDatabaseError("failed to marshal started_at", err)
	}
	startedAtStr, ok := startedAtAV.(*types.AttributeValueMemberS)
	if !ok {
		return apperrors.ErrDatabaseError("started_at is not a string attribute", nil)
	}

	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"execution_id": &types.AttributeValueMemberS{Value: execution.ExecutionID},
			"started_at":   &types.AttributeValueMemberS{Value: startedAtStr.Value},
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
