package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	apperrors "github.com/runvoy/runvoy/internal/errors"
	"github.com/runvoy/runvoy/internal/logger"
	awsconstants "github.com/runvoy/runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	createdByRequestIDIndexName  = "created_by_request_id-index"
	modifiedByRequestIDIndexName = "modified_by_request_id-index"
	createdByRequestIDAttrName   = "created_by_request_id"
	modifiedByRequestIDAttrName  = "modified_by_request_id"
)

// ExecutionRepository implements the database.ExecutionRepository interface using DynamoDB.
type ExecutionRepository struct {
	client    Client
	tableName string
	logger    *slog.Logger
}

// NewExecutionRepository creates a new DynamoDB-backed execution repository.
func NewExecutionRepository(client Client, tableName string, log *slog.Logger) *ExecutionRepository {
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
	ExecutionID         string   `dynamodbav:"execution_id"`
	StartedAt           int64    `dynamodbav:"started_at"`
	CreatedBy           string   `dynamodbav:"created_by"`
	OwnedBy             []string `dynamodbav:"owned_by"`
	Command             string   `dynamodbav:"command"`
	ImageID             string   `dynamodbav:"image_id"`
	Status              string   `dynamodbav:"status"`
	CompletedAt         *int64   `dynamodbav:"completed_at,omitempty"`
	ExitCode            int      `dynamodbav:"exit_code,omitempty"`
	DurationSecs        int      `dynamodbav:"duration_seconds,omitempty"`
	LogStreamName       string   `dynamodbav:"log_stream_name,omitempty"`
	CreatedByRequestID  string   `dynamodbav:"created_by_request_id,omitempty"`
	ModifiedByRequestID string   `dynamodbav:"modified_by_request_id,omitempty"`
	ComputePlatform     string   `dynamodbav:"compute_platform,omitempty"`
}

// toExecutionItem converts an api.Execution to an executionItem.
func toExecutionItem(e *api.Execution) *executionItem {
	item := &executionItem{
		ExecutionID:         e.ExecutionID,
		StartedAt:           e.StartedAt.Unix(),
		CreatedBy:           e.CreatedBy,
		OwnedBy:             e.OwnedBy,
		Command:             e.Command,
		ImageID:             e.ImageID,
		Status:              e.Status,
		ExitCode:            e.ExitCode,
		DurationSecs:        e.DurationSeconds,
		LogStreamName:       e.LogStreamName,
		CreatedByRequestID:  e.CreatedByRequestID,
		ModifiedByRequestID: e.ModifiedByRequestID,
		ComputePlatform:     e.ComputePlatform,
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
		ExecutionID:         e.ExecutionID,
		StartedAt:           time.Unix(e.StartedAt, 0).UTC(),
		CreatedBy:           e.CreatedBy,
		OwnedBy:             e.OwnedBy,
		Command:             e.Command,
		ImageID:             e.ImageID,
		Status:              e.Status,
		ExitCode:            e.ExitCode,
		DurationSeconds:     e.DurationSecs,
		LogStreamName:       e.LogStreamName,
		CreatedByRequestID:  e.CreatedByRequestID,
		ModifiedByRequestID: e.ModifiedByRequestID,
		ComputePlatform:     e.ComputePlatform,
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

	// Add _all field for the all-started_at GSI (sparse index pattern)
	av[awsconstants.DynamoDBAllAttribute] = &types.AttributeValueMemberS{Value: awsconstants.DynamoDBAllValue}

	reqLogger.Debug("calling external service", "context", map[string]string{
		"operation":    "DynamoDB.PutItem",
		"table":        r.tableName,
		"execution_id": execution.ExecutionID,
		"created_by":   execution.CreatedBy,
		"status":       execution.Status,
	})

	// Ensure uniqueness: only create if this execution_id does not already exist
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
		// This condition prevents overwriting an existing item with the same execution_id.
		// If an item exists, DynamoDB returns a ConditionalCheckFailedException, which we map to a conflict.
		ConditionExpression: aws.String("attribute_not_exists(execution_id)"),
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

	logArgs := []any{
		"operation", "DynamoDB.GetItem",
		"table", r.tableName,
		"execution_id", executionID,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(r.tableName),
		ConsistentRead: aws.Bool(true), // Use strongly consistent read to ensure we see the latest data
		Key: map[string]types.AttributeValue{
			"execution_id": &types.AttributeValueMemberS{Value: executionID},
		},
	})

	if err != nil {
		return nil, apperrors.ErrDatabaseError("failed to get execution", err)
	}

	if len(result.Item) == 0 {
		return nil, nil
	}

	var item executionItem
	if err = attributevalue.UnmarshalMap(result.Item, &item); err != nil {
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
		exprAttrValues[":completed_at"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(completedAt, 10)}
	}

	updateExpr += ", exit_code = :exit_code"
	exprAttrValues[":exit_code"] = &types.AttributeValueMemberN{Value: strconv.Itoa(execution.ExitCode)}

	if execution.DurationSeconds > 0 {
		updateExpr += ", duration_seconds = :duration_seconds"
		exprAttrValues[":duration_seconds"] = &types.AttributeValueMemberN{
			Value: strconv.Itoa(execution.DurationSeconds)}
	}

	if execution.LogStreamName != "" {
		updateExpr += ", log_stream_name = :log_stream_name"
		exprAttrValues[":log_stream_name"] = &types.AttributeValueMemberS{Value: execution.LogStreamName}
	}

	if execution.ModifiedByRequestID != "" {
		updateExpr += ", modified_by_request_id = :modified_by_request_id"
		exprAttrValues[":modified_by_request_id"] = &types.AttributeValueMemberS{Value: execution.ModifiedByRequestID}
	}

	return updateExpr, exprNames, exprAttrValues
}

// UpdateExecution updates an existing execution record.
func (r *ExecutionRepository) UpdateExecution(ctx context.Context, execution *api.Execution) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	const conditionExpr = "attribute_exists(execution_id)"

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
		})
		return apperrors.ErrDatabaseError("failed to update execution", updateErr)
	}

	return nil
}

const statusAttrName = "status"

// buildStatusFilterExpression builds a DynamoDB FilterExpression for status filtering.
// Returns the filter expression string and updates the expression attribute names/values maps.
func buildStatusFilterExpression(
	statuses []string,
	exprNames map[string]string,
	exprValues map[string]types.AttributeValue,
) string {
	if len(statuses) == 0 {
		return ""
	}

	if len(statuses) == 1 {
		exprNames["#status"] = statusAttrName
		exprValues[":status"] = &types.AttributeValueMemberS{Value: statuses[0]}
		return "#status = :status"
	}

	exprNames["#status"] = statusAttrName
	placeholders := make([]string, len(statuses))
	for i, status := range statuses {
		placeholder := fmt.Sprintf(":status%d", i)
		exprValues[placeholder] = &types.AttributeValueMemberS{Value: status}
		placeholders[i] = placeholder
	}

	return fmt.Sprintf("#status IN (%s)", strings.Join(placeholders, ", "))
}

// processQueryResults processes DynamoDB query results and appends executions to the slice.
// Returns true if the limit has been reached, false otherwise.
func processQueryResults(
	items []map[string]types.AttributeValue,
	executions []*api.Execution,
	limit int,
) ([]*api.Execution, bool, error) {
	for _, it := range items {
		var item executionItem
		if err := attributevalue.UnmarshalMap(it, &item); err != nil {
			return nil, false, apperrors.ErrDatabaseError("failed to unmarshal execution", err)
		}

		executions = append(executions, item.toAPIExecution())

		if limit > 0 && len(executions) >= limit {
			return executions, true, nil
		}
	}

	return executions, false, nil
}

// buildQueryLimit calculates a safe int32 limit value for DynamoDB queries.
// Multiplies the limit by 2 to account for filtering, with overflow protection.
func buildQueryLimit(limit int) int32 {
	const multiplier = 2
	const maxInt32 = 2147483647

	calculated := int64(limit) * multiplier
	if calculated > maxInt32 {
		return int32(maxInt32)
	}

	return int32(calculated)
}

// buildQueryInput constructs a DynamoDB QueryInput for listing executions.
func (r *ExecutionRepository) buildQueryInput(
	filterExpr string,
	exprNames map[string]string,
	exprValues map[string]types.AttributeValue,
	lastKey map[string]types.AttributeValue,
	limit int,
) *dynamodb.QueryInput {
	queryInput := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		IndexName:                 aws.String("all-started_at"),
		KeyConditionExpression:    aws.String("#all = :all"),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprValues,
		ScanIndexForward:          aws.Bool(false), // Sort descending by started_at (newest first)
		ExclusiveStartKey:         lastKey,
	}

	if limit > 0 {
		queryInput.Limit = aws.Int32(buildQueryLimit(limit))
	}

	if filterExpr != "" {
		queryInput.FilterExpression = aws.String(filterExpr)
	}

	return queryInput
}

// ListExecutions queries the executions table using the all-started_at GSI
// to return execution records sorted by StartedAt descending (newest first).
// This uses Query instead of Scan for better performance and native sorting by DynamoDB.
// Status filtering and limiting are handled natively by DynamoDB using FilterExpression and Limit.
//
// Parameters:
//   - limit: maximum number of executions to return. Use 0 to return all executions.
//   - statuses: optional slice of execution statuses to filter by.
//     If empty, all executions are returned.
func (r *ExecutionRepository) ListExecutions(
	ctx context.Context,
	limit int,
	statuses []string,
) ([]*api.Execution, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)
	initialCapacity := limit
	if initialCapacity <= 0 {
		initialCapacity = awsconstants.DefaultExecutionListCapacity
	}
	executions := make([]*api.Execution, 0, initialCapacity)
	var lastKey map[string]types.AttributeValue

	exprNames := map[string]string{
		"#all": awsconstants.DynamoDBAllAttribute,
	}
	exprValues := map[string]types.AttributeValue{
		":all": &types.AttributeValueMemberS{Value: awsconstants.DynamoDBAllValue},
	}

	filterExpr := buildStatusFilterExpression(statuses, exprNames, exprValues)

	reqLogger.Debug("calling external service", "context", map[string]string{
		"operation": "DynamoDB.Query",
		"table":     r.tableName,
		"index":     "all-started_at",
		"paginated": "true",
	})

	for {
		queryInput := r.buildQueryInput(filterExpr, exprNames, exprValues, lastKey, limit)

		out, err := r.client.Query(ctx, queryInput)
		if err != nil {
			return nil, apperrors.ErrDatabaseError("failed to query executions", err)
		}

		var reachedLimit bool
		executions, reachedLimit, err = processQueryResults(out.Items, executions, limit)
		if err != nil {
			return nil, err
		}

		if reachedLimit {
			return executions, nil
		}

		if len(out.LastEvaluatedKey) == 0 {
			break
		}
		lastKey = out.LastEvaluatedKey
	}

	return executions, nil
}

// queryExecutionsByRequestIDIndex queries a GSI by request ID and returns all matching executions.
func (r *ExecutionRepository) queryExecutionsByRequestIDIndex(
	ctx context.Context,
	indexName string,
	requestID string,
) ([]*api.Execution, error) {
	executions := make([]*api.Execution, 0)
	var lastKey map[string]types.AttributeValue

	var attributeName string
	switch indexName {
	case createdByRequestIDIndexName:
		attributeName = createdByRequestIDAttrName
	case modifiedByRequestIDIndexName:
		attributeName = modifiedByRequestIDAttrName
	default:
		return nil, apperrors.ErrDatabaseError(
			"unknown index name: "+indexName, nil)
	}

	exprNames := map[string]string{
		"#request_id": attributeName,
	}
	exprValues := map[string]types.AttributeValue{
		":request_id": &types.AttributeValueMemberS{Value: requestID},
	}

	for {
		queryInput := &dynamodb.QueryInput{
			TableName:                 aws.String(r.tableName),
			IndexName:                 aws.String(indexName),
			KeyConditionExpression:    aws.String("#request_id = :request_id"),
			ExpressionAttributeNames:  exprNames,
			ExpressionAttributeValues: exprValues,
			ScanIndexForward:          aws.Bool(false),
			ExclusiveStartKey:         lastKey,
		}

		result, err := r.client.Query(ctx, queryInput)
		if err != nil {
			return nil, apperrors.ErrDatabaseError(
				"failed to query executions by request ID from "+indexName, err)
		}

		for _, item := range result.Items {
			var execItem executionItem
			if err = attributevalue.UnmarshalMap(item, &execItem); err != nil {
				return nil, apperrors.ErrDatabaseError("failed to unmarshal execution item", err)
			}
			executions = append(executions, execItem.toAPIExecution())
		}

		if len(result.LastEvaluatedKey) == 0 {
			break
		}
		lastKey = result.LastEvaluatedKey
	}

	return executions, nil
}

// GetExecutionsByRequestID retrieves all executions created or modified by a specific request ID.
// This uses Query operations on two GSIs (created_by_request_id-index and modified_by_request_id-index)
// instead of Scan for better performance.
func (r *ExecutionRepository) GetExecutionsByRequestID(
	ctx context.Context, requestID string,
) ([]*api.Execution, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"request_id", requestID,
		"indexes", []string{createdByRequestIDIndexName, modifiedByRequestIDIndexName},
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	createdExecutions, err := r.queryExecutionsByRequestIDIndex(ctx, createdByRequestIDIndexName, requestID)
	if err != nil {
		return nil, err
	}

	modifiedExecutions, err := r.queryExecutionsByRequestIDIndex(ctx, modifiedByRequestIDIndexName, requestID)
	if err != nil {
		return nil, err
	}

	executionMap := make(map[string]*api.Execution)
	for _, exec := range createdExecutions {
		executionMap[exec.ExecutionID] = exec
	}
	for _, exec := range modifiedExecutions {
		executionMap[exec.ExecutionID] = exec
	}

	executions := make([]*api.Execution, 0, len(executionMap))
	for _, exec := range executionMap {
		executions = append(executions, exec)
	}

	return executions, nil
}
