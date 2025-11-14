// Package dynamodb implements DynamoDB-based storage for runvoy.
// It provides persistence for image-taskdef mappings using AWS DynamoDB.
package dynamodb

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ImageTaskDefRepository implements image-taskdef mapping operations using DynamoDB.
type ImageTaskDefRepository struct {
	client    Client
	tableName string
	logger    *slog.Logger
}

// NewImageTaskDefRepository creates a new DynamoDB-backed image-taskdef repository.
func NewImageTaskDefRepository(client Client, tableName string, log *slog.Logger) *ImageTaskDefRepository {
	return &ImageTaskDefRepository{
		client:    client,
		tableName: tableName,
		logger:    log,
	}
}

// imageTaskDefItem represents the structure stored in DynamoDB.
type imageTaskDefItem struct {
	Image                 string  `dynamodbav:"image"`         // Partition key
	CompositeKey          string  `dynamodbav:"composite_key"` // Sort key: role_composite#cpu#memory#runtime_platform
	TaskRoleName          *string `dynamodbav:"task_role_name,omitempty"`
	TaskExecutionRoleName *string `dynamodbav:"task_execution_role_name,omitempty"`
	Cpu                   string  `dynamodbav:"cpu"`              //nolint:revive // DynamoDB field name matches schema
	Memory                string  `dynamodbav:"memory"`           // e.g., "512", "2048"
	RuntimePlatform       string  `dynamodbav:"runtime_platform"` // e.g., "Linux/ARM64", "Linux/X86_64"
	TaskDefinitionFamily  string  `dynamodbav:"task_definition_family"`
	IsDefaultPlaceholder  *string `dynamodbav:"is_default_placeholder,omitempty"` // "DEFAULT" if default, nil otherwise
	// Parsed image components
	ImageRegistry string `dynamodbav:"image_registry"` // Empty = Docker Hub
	ImageName     string `dynamodbav:"image_name"`     // e.g., "alpine", "hashicorp/terraform"
	ImageTag      string `dynamodbav:"image_tag"`      // e.g., "latest", "1.6"
	CreatedAt     int64  `dynamodbav:"created_at"`
	UpdatedAt     int64  `dynamodbav:"updated_at"`
}

const (
	defaultRoleName         = "default"
	defaultPlaceholderValue = "DEFAULT"
)

// Default values for new image registrations
const (
	// DefaultCPU is the minimum Fargate CPU value
	DefaultCPU = "256"
	// DefaultMemory is the minimum Fargate Memory value (compatible with 256 CPU)
	DefaultMemory = "512"
	// DefaultRuntimePlatform is the default architecture (Graviton2 - better price-performance)
	DefaultRuntimePlatform = "Linux/ARM64"
)

// isDefault derives the boolean default status from the placeholder field.
func (item *imageTaskDefItem) isDefault() bool {
	return item.IsDefaultPlaceholder != nil && *item.IsDefaultPlaceholder == defaultPlaceholderValue
}

// buildRoleComposite creates a composite sort key from role names.
// Returns "default#default" if both are nil, otherwise "roleName1#roleName2".
func buildRoleComposite(taskRoleName, taskExecutionRoleName *string) string {
	taskRole := defaultRoleName
	execRole := defaultRoleName
	if taskRoleName != nil && *taskRoleName != "" {
		taskRole = *taskRoleName
	}
	if taskExecutionRoleName != nil && *taskExecutionRoleName != "" {
		execRole = *taskExecutionRoleName
	}
	return fmt.Sprintf("%s#%s", taskRole, execRole)
}

// buildCompositeKey creates a composite sort key from roles, CPU, Memory, and RuntimePlatform.
// Format: role_composite#cpu#memory#runtime_platform
func buildCompositeKey(taskRoleName, taskExecutionRoleName *string, cpu, memory, runtimePlatform string) string {
	roleComposite := buildRoleComposite(taskRoleName, taskExecutionRoleName)
	return fmt.Sprintf("%s#%s#%s#%s", roleComposite, cpu, memory, runtimePlatform)
}

// PutImageTaskDef stores or updates an image-taskdef mapping.
//
//nolint:funlen // Complex item construction with multiple fields
func (r *ImageTaskDefRepository) PutImageTaskDef(
	ctx context.Context,
	image string,
	imageRegistry string,
	imageName string,
	imageTag string,
	taskRoleName *string,
	taskExecutionRoleName *string,
	cpu string,
	memory string,
	runtimePlatform string,
	taskDefFamily string,
	isDefault bool,
) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	now := time.Now().Unix()
	compositeKey := buildCompositeKey(taskRoleName, taskExecutionRoleName, cpu, memory, runtimePlatform)

	item := &imageTaskDefItem{
		Image:                 image,
		CompositeKey:          compositeKey,
		TaskRoleName:          taskRoleName,
		TaskExecutionRoleName: taskExecutionRoleName,
		Cpu:                   cpu,
		Memory:                memory,
		RuntimePlatform:       runtimePlatform,
		TaskDefinitionFamily:  taskDefFamily,
		ImageRegistry:         imageRegistry,
		ImageName:             imageName,
		ImageTag:              imageTag,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	// Set placeholder for GSI if this is default
	if isDefault {
		placeholder := defaultPlaceholderValue
		item.IsDefaultPlaceholder = &placeholder
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return apperrors.ErrInternalError("failed to marshal image-taskdef item", err)
	}

	logArgs := []any{
		"operation", "DynamoDB.PutItem",
		"table", r.tableName,
		"image", image,
		"composite_key", compositeKey,
		"cpu", cpu,
		"memory", memory,
		"runtime_platform", runtimePlatform,
		"is_default", isDefault,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	})
	if err != nil {
		return apperrors.ErrInternalError("failed to put image-taskdef mapping", err)
	}

	return nil
}

// GetImageTaskDef retrieves a specific image-taskdef mapping by image, roles, CPU, Memory, and RuntimePlatform.
//
//nolint:funlen // Complex lookup with default value handling
func (r *ImageTaskDefRepository) GetImageTaskDef(
	ctx context.Context,
	image string,
	taskRoleName *string,
	taskExecutionRoleName *string,
	cpu *string,
	memory *string,
	runtimePlatform *string,
) (*api.ImageInfo, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Apply defaults if not provided
	cpuVal := DefaultCPU
	if cpu != nil && *cpu != "" {
		cpuVal = *cpu
	}
	memoryVal := DefaultMemory
	if memory != nil && *memory != "" {
		memoryVal = *memory
	}
	runtimePlatformVal := DefaultRuntimePlatform
	if runtimePlatform != nil && *runtimePlatform != "" {
		runtimePlatformVal = *runtimePlatform
	}

	compositeKey := buildCompositeKey(taskRoleName, taskExecutionRoleName, cpuVal, memoryVal, runtimePlatformVal)

	logArgs := []any{
		"operation", "DynamoDB.GetItem",
		"table", r.tableName,
		"image", image,
		"composite_key", compositeKey,
		"cpu", cpuVal,
		"memory", memoryVal,
		"runtime_platform", runtimePlatformVal,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"image":         &types.AttributeValueMemberS{Value: image},
			"composite_key": &types.AttributeValueMemberS{Value: compositeKey},
		},
	})
	if err != nil {
		return nil, apperrors.ErrInternalError("failed to get image-taskdef mapping", err)
	}

	if result.Item == nil {
		return nil, nil
	}

	var item imageTaskDefItem
	if unmarshalErr := attributevalue.UnmarshalMap(result.Item, &item); unmarshalErr != nil {
		return nil, apperrors.ErrInternalError("failed to unmarshal image-taskdef item", unmarshalErr)
	}

	isDefault := item.isDefault()
	return &api.ImageInfo{
		Image:                 item.Image,
		TaskDefinitionName:    item.TaskDefinitionFamily,
		IsDefault:             &isDefault,
		TaskRoleName:          item.TaskRoleName,
		TaskExecutionRoleName: item.TaskExecutionRoleName,
		Cpu:                   item.Cpu,
		Memory:                item.Memory,
		RuntimePlatform:       item.RuntimePlatform,
		ImageRegistry:         item.ImageRegistry,
		ImageName:             item.ImageName,
		ImageTag:              item.ImageTag,
	}, nil
}

// ListImages retrieves all registered images with their task definitions.
func (r *ImageTaskDefRepository) ListImages(ctx context.Context) ([]api.ImageInfo, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

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
		return nil, apperrors.ErrInternalError("failed to scan image-taskdef table", err)
	}

	var items []imageTaskDefItem
	if unmarshalErr := attributevalue.UnmarshalListOfMaps(result.Items, &items); unmarshalErr != nil {
		return nil, apperrors.ErrInternalError("failed to unmarshal image-taskdef items", unmarshalErr)
	}

	images := make([]api.ImageInfo, 0, len(items))
	for i := range items {
		item := &items[i]
		isDefault := item.isDefault()
		images = append(images, api.ImageInfo{
			Image:                 item.Image,
			TaskDefinitionName:    item.TaskDefinitionFamily,
			IsDefault:             &isDefault,
			TaskRoleName:          item.TaskRoleName,
			TaskExecutionRoleName: item.TaskExecutionRoleName,
			Cpu:                   item.Cpu,
			Memory:                item.Memory,
			RuntimePlatform:       item.RuntimePlatform,
			ImageRegistry:         item.ImageRegistry,
			ImageName:             item.ImageName,
			ImageTag:              item.ImageTag,
		})
	}

	// Sort by image name, then by role composite for consistency
	sort.Slice(images, func(i, j int) bool {
		if images[i].Image != images[j].Image {
			return images[i].Image < images[j].Image
		}
		// Secondary sort by roles
		roleI := buildRoleComposite(images[i].TaskRoleName, images[i].TaskExecutionRoleName)
		roleJ := buildRoleComposite(images[j].TaskRoleName, images[j].TaskExecutionRoleName)
		return roleI < roleJ
	})

	return images, nil
}

// GetDefaultImage retrieves the image marked as default.
func (r *ImageTaskDefRepository) GetDefaultImage(ctx context.Context) (*api.ImageInfo, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"index", "is_default-index",
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("is_default-index"),
		KeyConditionExpression: aws.String("is_default_placeholder = :placeholder"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":placeholder": &types.AttributeValueMemberS{Value: "DEFAULT"},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, apperrors.ErrInternalError("failed to query default image", err)
	}

	if len(result.Items) == 0 {
		return nil, nil
	}

	var item imageTaskDefItem
	if unmarshalErr := attributevalue.UnmarshalMap(result.Items[0], &item); unmarshalErr != nil {
		return nil, apperrors.ErrInternalError("failed to unmarshal default image item", unmarshalErr)
	}

	isDefault := item.isDefault()
	return &api.ImageInfo{
		Image:                 item.Image,
		TaskDefinitionName:    item.TaskDefinitionFamily,
		IsDefault:             &isDefault,
		TaskRoleName:          item.TaskRoleName,
		TaskExecutionRoleName: item.TaskExecutionRoleName,
		Cpu:                   item.Cpu,
		Memory:                item.Memory,
		RuntimePlatform:       item.RuntimePlatform,
		ImageRegistry:         item.ImageRegistry,
		ImageName:             item.ImageName,
		ImageTag:              item.ImageTag,
	}, nil
}

// UnmarkAllDefaults removes the default flag from all images.
func (r *ImageTaskDefRepository) UnmarkAllDefaults(ctx context.Context) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Query all items with is_default_placeholder = "DEFAULT"
	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("is_default-index"),
		KeyConditionExpression: aws.String("is_default_placeholder = :placeholder"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":placeholder": &types.AttributeValueMemberS{Value: "DEFAULT"},
		},
	})
	if err != nil {
		return apperrors.ErrInternalError("failed to query default images", err)
	}

	var items []imageTaskDefItem
	if unmarshalErr := attributevalue.UnmarshalListOfMaps(result.Items, &items); unmarshalErr != nil {
		return apperrors.ErrInternalError("failed to unmarshal default image items", unmarshalErr)
	}

	// Update each item to remove default status
	for i := range items {
		item := &items[i]
		roleComposite := buildRoleComposite(item.TaskRoleName, item.TaskExecutionRoleName)
		logArgs := []any{
			"operation", "DynamoDB.UpdateItem",
			"table", r.tableName,
			"image", item.Image,
			"role_composite", roleComposite,
		}
		logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

		_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(r.tableName),
			Key: map[string]types.AttributeValue{
				"image":         &types.AttributeValueMemberS{Value: item.Image},
				"composite_key": &types.AttributeValueMemberS{Value: item.CompositeKey},
			},
			UpdateExpression: aws.String("SET is_default = :false, updated_at = :now REMOVE is_default_placeholder"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":false": &types.AttributeValueMemberBOOL{Value: false},
				":now":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Unix())},
			},
		})
		if err != nil {
			return apperrors.ErrInternalError("failed to unmark default image", err)
		}
	}

	return nil
}

// DeleteImage removes all task definition mappings for a specific image.
func (r *ImageTaskDefRepository) DeleteImage(ctx context.Context, image string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Query all items for this image
	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"image", image,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("image = :image"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":image": &types.AttributeValueMemberS{Value: image},
		},
	})
	if err != nil {
		return apperrors.ErrInternalError("failed to query image mappings", err)
	}

	var items []imageTaskDefItem
	if unmarshalErr := attributevalue.UnmarshalListOfMaps(result.Items, &items); unmarshalErr != nil {
		return apperrors.ErrInternalError("failed to unmarshal image items", unmarshalErr)
	}

	// Delete each item
	for i := range items {
		item := &items[i]
		roleComposite := buildRoleComposite(item.TaskRoleName, item.TaskExecutionRoleName)
		deleteLogArgs := []any{
			"operation", "DynamoDB.DeleteItem",
			"table", r.tableName,
			"image", item.Image,
			"role_composite", roleComposite,
		}
		deleteLogArgs = append(deleteLogArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("calling external service", "context", logger.SliceToMap(deleteLogArgs))

		_, err = r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(r.tableName),
			Key: map[string]types.AttributeValue{
				"image":         &types.AttributeValueMemberS{Value: item.Image},
				"composite_key": &types.AttributeValueMemberS{Value: item.CompositeKey},
			},
		})
		if err != nil {
			return apperrors.ErrInternalError("failed to delete image mapping", err)
		}
	}

	return nil
}

// GetAnyImageTaskDef retrieves any task definition configuration for a given image.
// This is useful when you need to find a task definition for an image regardless of
// its specific CPU/Memory/RuntimePlatform configuration.
// It queries by image (partition key) and returns the first matching item,
// preferring the default configuration if available.
//
//nolint:funlen // Complex query with preference logic
func (r *ImageTaskDefRepository) GetAnyImageTaskDef(ctx context.Context, image string) (*api.ImageInfo, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"image", image,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("image = :image"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":image": &types.AttributeValueMemberS{Value: image},
		},
		Limit: aws.Int32(100), //nolint:mnd // Get up to 100 items to find default if available
	})
	if err != nil {
		return nil, apperrors.ErrInternalError("failed to query image-taskdef mappings", err)
	}

	if len(result.Items) == 0 {
		return nil, nil
	}

	var items []imageTaskDefItem
	if unmarshalErr := attributevalue.UnmarshalListOfMaps(result.Items, &items); unmarshalErr != nil {
		return nil, apperrors.ErrInternalError("failed to unmarshal image-taskdef items", unmarshalErr)
	}

	// Prefer default configuration if available
	for i := range items {
		if items[i].isDefault() {
			item := &items[i]
			isDefault := true
			return &api.ImageInfo{
				Image:                 item.Image,
				TaskDefinitionName:    item.TaskDefinitionFamily,
				IsDefault:             &isDefault,
				TaskRoleName:          item.TaskRoleName,
				TaskExecutionRoleName: item.TaskExecutionRoleName,
				Cpu:                   item.Cpu,
				Memory:                item.Memory,
				RuntimePlatform:       item.RuntimePlatform,
				ImageRegistry:         item.ImageRegistry,
				ImageName:             item.ImageName,
				ImageTag:              item.ImageTag,
			}, nil
		}
	}

	// If no default found, return the first one
	item := &items[0]
	isDefault := item.isDefault()
	return &api.ImageInfo{
		Image:                 item.Image,
		TaskDefinitionName:    item.TaskDefinitionFamily,
		IsDefault:             &isDefault,
		TaskRoleName:          item.TaskRoleName,
		TaskExecutionRoleName: item.TaskExecutionRoleName,
		Cpu:                   item.Cpu,
		Memory:                item.Memory,
		RuntimePlatform:       item.RuntimePlatform,
		ImageRegistry:         item.ImageRegistry,
		ImageName:             item.ImageName,
		ImageTag:              item.ImageTag,
	}, nil
}

// GetImagesCount returns the total number of unique image+role combinations.
func (r *ImageTaskDefRepository) GetImagesCount(ctx context.Context) (int, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.Scan",
		"table", r.tableName,
		"select", "COUNT",
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(r.tableName),
		Select:    types.SelectCount,
	})
	if err != nil {
		return 0, apperrors.ErrInternalError("failed to count images", err)
	}

	return int(result.Count), nil
}

// GetUniqueImages returns a list of unique image names (deduplicated across role combinations).
func (r *ImageTaskDefRepository) GetUniqueImages(ctx context.Context) ([]string, error) {
	images, err := r.ListImages(ctx)
	if err != nil {
		return nil, err
	}

	// Deduplicate image names
	uniqueMap := make(map[string]bool)
	for i := range images {
		uniqueMap[images[i].Image] = true
	}

	uniqueImages := make([]string, 0, len(uniqueMap))
	for img := range uniqueMap {
		uniqueImages = append(uniqueImages, img)
	}

	// Sort for consistency
	sort.Strings(uniqueImages)

	return uniqueImages, nil
}

// SetImageAsOnlyDefault marks a specific image+role combination as the only default.
// It first unmarksall other defaults, then sets this one as default.
func (r *ImageTaskDefRepository) SetImageAsOnlyDefault(
	ctx context.Context,
	image string,
	taskRoleName *string,
	taskExecutionRoleName *string,
) error {
	// First, unmark all existing defaults
	if err := r.UnmarkAllDefaults(ctx); err != nil {
		return err
	}

	// Then mark this one as default
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)
	roleComposite := buildRoleComposite(taskRoleName, taskExecutionRoleName)

	logArgs := []any{
		"operation", "DynamoDB.UpdateItem",
		"table", r.tableName,
		"image", image,
		"role_composite", roleComposite,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	compositeKey := buildCompositeKey(
		taskRoleName, taskExecutionRoleName, DefaultCPU, DefaultMemory, DefaultRuntimePlatform,
	)

	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"image":         &types.AttributeValueMemberS{Value: image},
			"composite_key": &types.AttributeValueMemberS{Value: compositeKey},
		},
		UpdateExpression: aws.String("SET is_default = :true, is_default_placeholder = :placeholder, updated_at = :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":true":        &types.AttributeValueMemberBOOL{Value: true},
			":placeholder": &types.AttributeValueMemberS{Value: "DEFAULT"},
			":now":         &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Unix())},
		},
	})
	if err != nil {
		return apperrors.ErrInternalError("failed to set image as default", err)
	}

	return nil
}
