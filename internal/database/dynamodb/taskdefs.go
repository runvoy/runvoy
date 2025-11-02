package dynamodb

import (
	"context"
	stderrors "errors"
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

// TaskDefinitionRepository implements task definition registry operations using DynamoDB.
type TaskDefinitionRepository struct {
	client    *dynamodb.Client
	tableName string
	logger    *slog.Logger
}

// NewTaskDefinitionRepository creates a new DynamoDB-backed task definition repository.
func NewTaskDefinitionRepository(client *dynamodb.Client, tableName string, logger *slog.Logger) *TaskDefinitionRepository {
	return &TaskDefinitionRepository{
		client:    client,
		tableName: tableName,
		logger:    logger,
	}
}

// taskDefinitionItem represents the structure stored in DynamoDB.
type taskDefinitionItem struct {
	TaskKey          string    `dynamodbav:"task_key"`          // Hash of (image + hasGit)
	Image            string    `dynamodbav:"image"`             // Docker image name
	HasGit           bool      `dynamodbav:"has_git"`           // Whether this task def supports git
	TaskDefinitionARN string    `dynamodbav:"task_definition_arn"` // ECS task definition ARN
	CreatedAt        time.Time `dynamodbav:"created_at"`        // When this was registered
	LastUsed         time.Time `dynamodbav:"last_used,omitempty"` // Last time this was used
	CreatedBy        string    `dynamodbav:"created_by,omitempty"` // User who registered it (optional)
}

// GetTaskDefinition retrieves a task definition by its key (image + hasGit hash).
func (r *TaskDefinitionRepository) GetTaskDefinition(ctx context.Context, taskKey string) (*api.TaskDefinition, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB GetItem
	logArgs := []any{
		"operation", "DynamoDB.GetItem",
		"table", r.tableName,
		"taskKey", taskKey,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"task_key": &types.AttributeValueMemberS{Value: taskKey},
		},
	})

	if err != nil {
		reqLogger.Debug("failed to get task definition", "error", err)
		return nil, apperrors.ErrDatabaseError("failed to get task definition", err)
	}

	if result.Item == nil {
		reqLogger.Debug("task definition not found", "taskKey", taskKey)
		return nil, nil
	}

	var item taskDefinitionItem
	if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		return nil, apperrors.ErrInternalError("failed to unmarshal task definition", err)
	}

	return &api.TaskDefinition{
		TaskKey:          item.TaskKey,
		Image:            item.Image,
		HasGit:           item.HasGit,
		TaskDefinitionARN: item.TaskDefinitionARN,
		CreatedAt:        item.CreatedAt,
		LastUsed:         item.LastUsed,
		CreatedBy:        item.CreatedBy,
	}, nil
}

// CreateTaskDefinition stores a new task definition registry entry.
func (r *TaskDefinitionRepository) CreateTaskDefinition(ctx context.Context, taskDef *api.TaskDefinition) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Create the item to store
	item := taskDefinitionItem{
		TaskKey:           taskDef.TaskKey,
		Image:             taskDef.Image,
		HasGit:            taskDef.HasGit,
		TaskDefinitionARN: taskDef.TaskDefinitionARN,
		CreatedAt:         taskDef.CreatedAt,
		CreatedBy:         taskDef.CreatedBy,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return apperrors.ErrInternalError("failed to marshal task definition", err)
	}

	// Log before calling DynamoDB PutItem
	logArgs := []any{
		"operation", "DynamoDB.PutItem",
		"table", r.tableName,
		"taskKey", taskDef.TaskKey,
		"image", taskDef.Image,
		"hasGit", taskDef.HasGit,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	// Use ConditionExpression to ensure we don't overwrite existing entries
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(task_key)"),
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if stderrors.As(err, &ccf) {
			return apperrors.ErrConflict("task definition with this key already exists", nil)
		}
		return apperrors.ErrDatabaseError("failed to create task definition", err)
	}

	return nil
}

// UpdateLastUsed updates the last_used timestamp for a task definition.
func (r *TaskDefinitionRepository) UpdateLastUsed(ctx context.Context, taskKey string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	now := time.Now().UTC()

	// Log before calling DynamoDB UpdateItem
	logArgs := []any{
		"operation", "DynamoDB.UpdateItem",
		"table", r.tableName,
		"taskKey", taskKey,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"task_key": &types.AttributeValueMemberS{Value: taskKey},
		},
		UpdateExpression: aws.String("SET last_used = :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now": &types.AttributeValueMemberS{
				Value: now.Format(time.RFC3339Nano),
			},
		},
	})

	if err != nil {
		return apperrors.ErrDatabaseError("failed to update last_used", err)
	}

	return nil
}

// ListTaskDefinitions returns all registered task definitions.
func (r *TaskDefinitionRepository) ListTaskDefinitions(ctx context.Context) ([]*api.TaskDefinition, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Log before calling DynamoDB Scan
	logArgs := []any{
		"operation", "DynamoDB.Scan",
		"table", r.tableName,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(r.tableName),
	})

	if err != nil {
		return nil, apperrors.ErrDatabaseError("failed to list task definitions", err)
	}

	var taskDefs []*api.TaskDefinition
	for _, item := range result.Items {
		var dbItem taskDefinitionItem
		if err := attributevalue.UnmarshalMap(item, &dbItem); err != nil {
			reqLogger.Warn("failed to unmarshal task definition item", "error", err)
			continue
		}

		taskDefs = append(taskDefs, &api.TaskDefinition{
			TaskKey:           dbItem.TaskKey,
			Image:             dbItem.Image,
			HasGit:            dbItem.HasGit,
			TaskDefinitionARN: dbItem.TaskDefinitionARN,
			CreatedAt:         dbItem.CreatedAt,
			LastUsed:          dbItem.LastUsed,
			CreatedBy:         dbItem.CreatedBy,
		})
	}

	return taskDefs, nil
}