// Package dynamodb implements DynamoDB-based storage for runvoy.
// It provides persistence for image-taskdef mappings using AWS DynamoDB.
package dynamodb

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"
	awsConstants "runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/smithy-go"
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
	ImageID               string  `dynamodbav:"image_id"`
	Image                 string  `dynamodbav:"image"`
	TaskRoleName          *string `dynamodbav:"task_role_name,omitempty"`
	TaskExecutionRoleName *string `dynamodbav:"task_execution_role_name,omitempty"`
	Cpu                   string  `dynamodbav:"cpu"` //nolint:revive // DynamoDB field name matches schema
	Memory                string  `dynamodbav:"memory"`
	RuntimePlatform       string  `dynamodbav:"runtime_platform"`
	TaskDefinitionFamily  string  `dynamodbav:"task_definition_family"`
	IsDefaultPlaceholder  *string `dynamodbav:"is_default_placeholder,omitempty"`
	ImageRegistry         string  `dynamodbav:"image_registry"`
	ImageName             string  `dynamodbav:"image_name"`
	ImageTag              string  `dynamodbav:"image_tag"`
	RegisteredBy          string  `dynamodbav:"registered_by,omitempty"`
	CreatedAt             int64   `dynamodbav:"created_at"`
	UpdatedAt             int64   `dynamodbav:"updated_at"`
}

const (
	defaultRoleName         = "default"
	defaultPlaceholderValue = "DEFAULT"
)

// isDefault derives the boolean default status from the placeholder field.
func (item *imageTaskDefItem) isDefault() bool {
	return item.IsDefaultPlaceholder != nil && *item.IsDefaultPlaceholder == defaultPlaceholderValue
}

// GenerateImageID generates a unique, human-readable ID for an image configuration.
// Format: {imageName}:{tag}-{first-8-chars-of-hash}
// Example: alpine:latest-a1b2c3d4 or golang:1.24.5-bookworm-19884ca2
func GenerateImageID(
	imageName, imageTag string,
	cpu, memory int,
	runtimePlatform string,
	taskRoleName, taskExecutionRoleName *string,
) string {
	// Build role composite inline
	taskRole := defaultRoleName
	execRole := defaultRoleName
	if taskRoleName != nil && *taskRoleName != "" {
		taskRole = *taskRoleName
	}
	if taskExecutionRoleName != nil && *taskExecutionRoleName != "" {
		execRole = *taskExecutionRoleName
	}
	roleComposite := fmt.Sprintf("%s#%s", taskRole, execRole)

	hashInput := fmt.Sprintf("%s:%s:%d:%d:%s:%s", imageName, imageTag, cpu, memory, runtimePlatform, roleComposite)
	hash := sha256.Sum256([]byte(hashInput))
	hashStr := fmt.Sprintf("%x", hash)
	shortHash := hashStr[:8]
	imageID := fmt.Sprintf("%s:%s-%s", imageName, imageTag, shortHash)
	return imageID
}

// PutImageTaskDef stores or updates an image-taskdef mapping.
//
//nolint:funlen // Complex item construction with multiple fields
func (r *ImageTaskDefRepository) PutImageTaskDef(
	ctx context.Context,
	imageID string,
	image string,
	imageRegistry string,
	imageName string,
	imageTag string,
	taskRoleName *string,
	taskExecutionRoleName *string,
	cpu int,
	memory int,
	runtimePlatform string,
	taskDefFamily string,
	isDefault bool,
) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	now := time.Now().Unix()
	cpuStr := fmt.Sprintf("%d", cpu)
	memoryStr := fmt.Sprintf("%d", memory)

	item := &imageTaskDefItem{
		ImageID:               imageID,
		Image:                 image,
		TaskRoleName:          taskRoleName,
		TaskExecutionRoleName: taskExecutionRoleName,
		Cpu:                   cpuStr,
		Memory:                memoryStr,
		RuntimePlatform:       runtimePlatform,
		TaskDefinitionFamily:  taskDefFamily,
		ImageRegistry:         imageRegistry,
		ImageName:             imageName,
		ImageTag:              imageTag,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

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
		"image_id", imageID,
		"image", image,
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

// parseImageReference parses a Docker image reference into name and tag.
// This is a simplified version to avoid import cycles.
func parseImageReference(image string) (name, tag string) {
	tag = "latest" // Default tag

	// Split on '@' to handle digest references
	var remainder string
	idx := strings.Index(image, "@")
	if idx != -1 {
		remainder = image[:idx]
		tag = image[idx+1:] // Everything after @ is the digest
	} else {
		remainder = image
		// Split on ':' to extract tag
		tagIdx := strings.LastIndex(remainder, ":")
		if tagIdx != -1 {
			// Check if this is a tag (not a port number in registry)
			firstSlash := strings.Index(remainder, "/")
			if firstSlash == -1 || tagIdx > firstSlash {
				// This is a tag, not a port
				tag = remainder[tagIdx+1:]
				remainder = remainder[:tagIdx]
			}
		}
	}

	// Now remainder is registry/name or just name
	// Extract name (everything after the first slash if it contains a registry)
	const splitLimit = 2
	parts := strings.SplitN(remainder, "/", splitLimit)

	if len(parts) == 1 {
		name = parts[0]
	} else {
		firstPart := parts[0]
		if strings.Contains(firstPart, ".") ||
			strings.Contains(firstPart, ":") ||
			firstPart == "localhost" {
			name = parts[1]
		} else {
			name = remainder
		}
	}

	return name, tag
}

// looksLikeImageID checks if a string looks like an ImageID format.
// ImageID format: {name}:{tag}-{8-char-hash}
func looksLikeImageID(s string) bool {
	const hashLength = 8
	lastDashIdx := strings.LastIndex(s, "-")
	if lastDashIdx == -1 {
		return false
	}
	hashPart := s[lastDashIdx+1:]
	if len(hashPart) == hashLength {
		for _, c := range hashPart {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
				return false
			}
		}
		beforeHash := s[:lastDashIdx]
		return strings.Contains(beforeHash, ":")
	}
	return false
}

// GetImageTaskDef retrieves a specific image-taskdef mapping by generating ImageID from the configuration.
func (r *ImageTaskDefRepository) GetImageTaskDef(
	ctx context.Context,
	image string,
	taskRoleName *string,
	taskExecutionRoleName *string,
	cpu *int,
	memory *int,
	runtimePlatform *string,
) (*api.ImageInfo, error) {
	imageName, imageTag := parseImageReference(image)

	cpuVal := awsConstants.DefaultCPU
	if cpu != nil {
		cpuVal = *cpu
	}
	memoryVal := awsConstants.DefaultMemory
	if memory != nil {
		memoryVal = *memory
	}
	runtimePlatformVal := awsConstants.DefaultRuntimePlatform
	if runtimePlatform != nil && *runtimePlatform != "" {
		runtimePlatformVal = *runtimePlatform
	}

	imageID := GenerateImageID(
		imageName,
		imageTag,
		cpuVal,
		memoryVal,
		runtimePlatformVal,
		taskRoleName,
		taskExecutionRoleName,
	)

	return r.GetImageTaskDefByID(ctx, imageID)
}

// GetImageTaskDefByID retrieves an image-taskdef mapping by ImageID.
func (r *ImageTaskDefRepository) GetImageTaskDefByID(ctx context.Context, imageID string) (*api.ImageInfo, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.GetItem",
		"table", r.tableName,
		"image_id", imageID,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"image_id": &types.AttributeValueMemberS{Value: imageID},
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

	cpuInt, parseErr := strconv.Atoi(item.Cpu)
	if parseErr != nil {
		return nil, apperrors.ErrInternalError("failed to parse CPU value", parseErr)
	}
	memoryInt, parseErr := strconv.Atoi(item.Memory)
	if parseErr != nil {
		return nil, apperrors.ErrInternalError("failed to parse Memory value", parseErr)
	}

	isDefault := item.isDefault()
	return &api.ImageInfo{
		ImageID:               item.ImageID,
		Image:                 item.Image,
		TaskDefinitionName:    item.TaskDefinitionFamily,
		IsDefault:             &isDefault,
		TaskRoleName:          item.TaskRoleName,
		TaskExecutionRoleName: item.TaskExecutionRoleName,
		CPU:                   cpuInt,
		Memory:                memoryInt,
		RuntimePlatform:       item.RuntimePlatform,
		ImageRegistry:         item.ImageRegistry,
		ImageName:             item.ImageName,
		ImageTag:              item.ImageTag,
		RegisteredBy:          item.RegisteredBy,
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

	allImages, convertErr := r.convertItemsToImageInfo(items)
	if convertErr != nil {
		return nil, convertErr
	}

	sort.Slice(allImages, func(i, j int) bool {
		if allImages[i].Image != allImages[j].Image {
			return allImages[i].Image < allImages[j].Image
		}
		return allImages[i].ImageID < allImages[j].ImageID
	})

	return allImages, nil
}

// convertItemsToImageInfo converts DynamoDB items to ImageInfo structs.
func (r *ImageTaskDefRepository) convertItemsToImageInfo(items []imageTaskDefItem) ([]api.ImageInfo, error) {
	allImages := make([]api.ImageInfo, 0, len(items))
	for i := range items {
		item := &items[i]
		isDefault := item.isDefault()

		cpuInt, parseErr := strconv.Atoi(item.Cpu)
		if parseErr != nil {
			return nil, apperrors.ErrInternalError("failed to parse CPU value", parseErr)
		}
		memoryInt, parseErr := strconv.Atoi(item.Memory)
		if parseErr != nil {
			return nil, apperrors.ErrInternalError("failed to parse Memory value", parseErr)
		}

		allImages = append(allImages, api.ImageInfo{
			ImageID:               item.ImageID,
			Image:                 item.Image,
			TaskDefinitionName:    item.TaskDefinitionFamily,
			IsDefault:             &isDefault,
			TaskRoleName:          item.TaskRoleName,
			TaskExecutionRoleName: item.TaskExecutionRoleName,
			CPU:                   cpuInt,
			Memory:                memoryInt,
			RuntimePlatform:       item.RuntimePlatform,
			ImageRegistry:         item.ImageRegistry,
			ImageName:             item.ImageName,
			ImageTag:              item.ImageTag,
		})
	}
	return allImages, nil
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

	// Convert from strings to ints
	cpuInt, err := strconv.Atoi(item.Cpu)
	if err != nil {
		return nil, apperrors.ErrInternalError("failed to parse CPU value", err)
	}
	memoryInt, err := strconv.Atoi(item.Memory)
	if err != nil {
		return nil, apperrors.ErrInternalError("failed to parse Memory value", err)
	}

	isDefault := item.isDefault()
	return &api.ImageInfo{
		ImageID:               item.ImageID,
		Image:                 item.Image,
		TaskDefinitionName:    item.TaskDefinitionFamily,
		IsDefault:             &isDefault,
		TaskRoleName:          item.TaskRoleName,
		TaskExecutionRoleName: item.TaskExecutionRoleName,
		CPU:                   cpuInt,
		Memory:                memoryInt,
		RuntimePlatform:       item.RuntimePlatform,
		ImageRegistry:         item.ImageRegistry,
		ImageName:             item.ImageName,
		ImageTag:              item.ImageTag,
		RegisteredBy:          item.RegisteredBy,
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

	for i := range items {
		item := &items[i]
		logArgs := []any{
			"operation", "DynamoDB.UpdateItem",
			"table", r.tableName,
			"image_id", item.ImageID,
		}
		logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

		_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(r.tableName),
			Key: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: item.ImageID},
			},
			UpdateExpression: aws.String("SET updated_at = :now REMOVE is_default_placeholder"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":now": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Unix())},
			},
		})
		if err != nil {
			return apperrors.ErrInternalError("failed to unmark default image", err)
		}
	}

	return nil
}

// findItemsByImageID finds items by ImageID format.
func (r *ImageTaskDefRepository) findItemsByImageID(ctx context.Context, image string) ([]imageTaskDefItem, error) {
	imageInfo, getErr := r.GetImageTaskDefByID(ctx, image)
	if getErr != nil {
		return nil, apperrors.ErrInternalError("failed to get image by ImageID", getErr)
	}
	if imageInfo == nil {
		return nil, apperrors.ErrNotFound("image not found", fmt.Errorf("image with ImageID %q not found", image))
	}

	item := &imageTaskDefItem{
		ImageID:              imageInfo.ImageID,
		Image:                imageInfo.Image,
		TaskDefinitionFamily: imageInfo.TaskDefinitionName,
		ImageName:            imageInfo.ImageName,
		ImageTag:             imageInfo.ImageTag,
		Cpu:                  fmt.Sprintf("%d", imageInfo.CPU),
		Memory:               fmt.Sprintf("%d", imageInfo.Memory),
		RuntimePlatform:      imageInfo.RuntimePlatform,
	}
	if imageInfo.TaskRoleName != nil {
		item.TaskRoleName = imageInfo.TaskRoleName
	}
	if imageInfo.TaskExecutionRoleName != nil {
		item.TaskExecutionRoleName = imageInfo.TaskExecutionRoleName
	}
	return []imageTaskDefItem{*item}, nil
}

// findItemsByNameTag finds items by matching name:tag components.
func (r *ImageTaskDefRepository) findItemsByNameTag(ctx context.Context, image string) ([]imageTaskDefItem, error) {
	queryName, queryTag := parseImageReference(image)
	queryNameTag := fmt.Sprintf("%s:%s", queryName, queryTag)

	allResult, allScanErr := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(r.tableName),
	})
	if allScanErr != nil {
		return nil, apperrors.ErrInternalError("failed to scan image mappings", allScanErr)
	}

	var allItems []imageTaskDefItem
	if unmarshalErr := attributevalue.UnmarshalListOfMaps(allResult.Items, &allItems); unmarshalErr != nil {
		return nil, apperrors.ErrInternalError("failed to unmarshal image items", unmarshalErr)
	}

	var items []imageTaskDefItem
	for i := range allItems {
		storedNameTag := fmt.Sprintf("%s:%s", allItems[i].ImageName, allItems[i].ImageTag)
		if storedNameTag == queryNameTag {
			items = append(items, allItems[i])
		}
	}
	return items, nil
}

// findItemsByImage finds items by exact image match, with fallback to name:tag matching.
func (r *ImageTaskDefRepository) findItemsByImage(
	ctx context.Context,
	reqLogger *slog.Logger,
	image string,
) ([]imageTaskDefItem, error) {
	logArgs := []any{
		"operation", "DynamoDB.Scan",
		"table", r.tableName,
		"image", image,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, scanErr := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("image = :image"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":image": &types.AttributeValueMemberS{Value: image},
		},
	})
	if scanErr != nil {
		return nil, apperrors.ErrInternalError("failed to scan image mappings", scanErr)
	}

	var items []imageTaskDefItem
	if unmarshalErr := attributevalue.UnmarshalListOfMaps(result.Items, &items); unmarshalErr != nil {
		return nil, apperrors.ErrInternalError("failed to unmarshal image items", unmarshalErr)
	}

	if len(items) == 0 {
		return r.findItemsByNameTag(ctx, image)
	}

	return items, nil
}

// DeleteImage removes all task definition mappings for a specific image.
// Supports exact matching on both the image field and image_id field (for ImageID format).
// Returns ErrNotFound if no matching images are found.
func (r *ImageTaskDefRepository) DeleteImage(ctx context.Context, image string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	var items []imageTaskDefItem
	var err error

	if looksLikeImageID(image) {
		items, err = r.findItemsByImageID(ctx, image)
		if err != nil {
			return err
		}
	} else {
		items, err = r.findItemsByImage(ctx, reqLogger, image)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return apperrors.ErrNotFound("image not found", fmt.Errorf("image %q not found", image))
		}
	}

	for i := range items {
		item := &items[i]
		deleteLogArgs := []any{
			"operation", "DynamoDB.DeleteItem",
			"table", r.tableName,
			"image_id", item.ImageID,
		}
		deleteLogArgs = append(deleteLogArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("calling external service", "context", logger.SliceToMap(deleteLogArgs))

		_, err = r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(r.tableName),
			Key: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: item.ImageID},
			},
		})
		if err != nil {
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) && apiErr.ErrorCode() == "ValidationException" {
				reqLogger.Warn("failed to delete item (may not exist or wrong schema)", "context", map[string]any{
					"image_id": item.ImageID,
					"error":    err.Error(),
				})
				continue
			}
			return apperrors.ErrInternalError("failed to delete image mapping", err)
		}
	}

	return nil
}

// GetAnyImageTaskDef retrieves any task definition configuration for a given image.
// Supports flexible matching: tries exact match on full image first, then matches by name:tag components.
// Returns the first matching item, preferring the default configuration if available.
//
//nolint:funlen // Complex logic with helper function
func (r *ImageTaskDefRepository) GetAnyImageTaskDef(ctx context.Context, image string) (*api.ImageInfo, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.Scan",
		"table", r.tableName,
		"image", image,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("image = :image"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":image": &types.AttributeValueMemberS{Value: image},
		},
		Limit: aws.Int32(100), //nolint:mnd // Get up to 100 items to find default if available
	})
	if err != nil {
		return nil, apperrors.ErrInternalError("failed to scan image-taskdef mappings", err)
	}

	var items []imageTaskDefItem
	if unmarshalErr := attributevalue.UnmarshalListOfMaps(result.Items, &items); unmarshalErr != nil {
		return nil, apperrors.ErrInternalError("failed to unmarshal image-taskdef items", unmarshalErr)
	}

	// If no exact match found, try matching by name:tag components
	if len(items) == 0 {
		queryName, queryTag := parseImageReference(image)
		queryNameTag := fmt.Sprintf("%s:%s", queryName, queryTag)

		// Scan all items and match by name:tag
		allResult, scanErr := r.client.Scan(ctx, &dynamodb.ScanInput{
			TableName: aws.String(r.tableName),
			Limit:     aws.Int32(100), //nolint:mnd // Get up to 100 items to find default if available
		})
		if scanErr != nil {
			return nil, apperrors.ErrInternalError("failed to scan image-taskdef mappings", scanErr)
		}

		var allItems []imageTaskDefItem
		if unmarshalErr := attributevalue.UnmarshalListOfMaps(allResult.Items, &allItems); unmarshalErr != nil {
			return nil, apperrors.ErrInternalError("failed to unmarshal image-taskdef items", unmarshalErr)
		}

		// Filter by name:tag match
		for i := range allItems {
			storedNameTag := fmt.Sprintf("%s:%s", allItems[i].ImageName, allItems[i].ImageTag)
			if storedNameTag == queryNameTag {
				items = append(items, allItems[i])
			}
		}

		if len(items) == 0 {
			return nil, nil
		}
	}

	convertItem := func(item *imageTaskDefItem) (*api.ImageInfo, error) {
		cpuInt, parseErr := strconv.Atoi(item.Cpu)
		if parseErr != nil {
			return nil, apperrors.ErrInternalError("failed to parse CPU value", parseErr)
		}
		memoryInt, parseErr := strconv.Atoi(item.Memory)
		if parseErr != nil {
			return nil, apperrors.ErrInternalError("failed to parse Memory value", parseErr)
		}

		isDefault := item.isDefault()
		return &api.ImageInfo{
			ImageID:               item.ImageID,
			Image:                 item.Image,
			TaskDefinitionName:    item.TaskDefinitionFamily,
			IsDefault:             &isDefault,
			TaskRoleName:          item.TaskRoleName,
			TaskExecutionRoleName: item.TaskExecutionRoleName,
			CPU:                   cpuInt,
			Memory:                memoryInt,
			RuntimePlatform:       item.RuntimePlatform,
			ImageRegistry:         item.ImageRegistry,
			ImageName:             item.ImageName,
			ImageTag:              item.ImageTag,
		}, nil
	}

	for i := range items {
		if items[i].isDefault() {
			return convertItem(&items[i])
		}
	}

	return convertItem(&items[0])
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

	uniqueMap := make(map[string]bool)
	for i := range images {
		uniqueMap[images[i].Image] = true
	}

	uniqueImages := make([]string, 0, len(uniqueMap))
	for img := range uniqueMap {
		uniqueImages = append(uniqueImages, img)
	}

	sort.Strings(uniqueImages)

	return uniqueImages, nil
}

// SetImageAsOnlyDefault marks a specific image configuration as the only default.
// It first unmarks all other defaults, then sets this one as default.
func (r *ImageTaskDefRepository) SetImageAsOnlyDefault(
	ctx context.Context,
	image string,
	taskRoleName *string,
	taskExecutionRoleName *string,
) error {
	if err := r.UnmarkAllDefaults(ctx); err != nil {
		return err
	}

	imageName, imageTag := parseImageReference(image)
	imageID := GenerateImageID(
		imageName,
		imageTag,
		awsConstants.DefaultCPU,
		awsConstants.DefaultMemory,
		awsConstants.DefaultRuntimePlatform,
		taskRoleName,
		taskExecutionRoleName,
	)

	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.UpdateItem",
		"table", r.tableName,
		"image_id", imageID,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"image_id": &types.AttributeValueMemberS{Value: imageID},
		},
		UpdateExpression: aws.String("SET is_default_placeholder = :placeholder, updated_at = :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":placeholder": &types.AttributeValueMemberS{Value: "DEFAULT"},
			":now":         &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Unix())},
		},
	})
	if err != nil {
		return apperrors.ErrInternalError("failed to set image as default", err)
	}

	return nil
}
