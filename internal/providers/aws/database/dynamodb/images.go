// Package dynamodb implements DynamoDB-based storage for runvoy.
// It provides persistence for image-taskdef mappings using AWS DynamoDB.
package dynamodb

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"

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
	ImageID               string  `dynamodbav:"image_id"` // Partition key
	Image                 string  `dynamodbav:"image"`    // Regular attribute for queries by image name
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
	// DefaultCPU is the minimum Fargate CPU value (in CPU units)
	DefaultCPU = 256
	// DefaultMemory is the minimum Fargate Memory value in MB (compatible with 256 CPU)
	DefaultMemory = 512
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

// sanitizeImageNameForID sanitizes an image name for use in ImageID.
// Replaces invalid characters (/, :, etc.) with hyphens and removes registry.
func sanitizeImageNameForID(imageName string) string {
	// Replace invalid characters with hyphens
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := re.ReplaceAllString(imageName, "-")
	// Collapse multiple hyphens
	re2 := regexp.MustCompile(`-+`)
	sanitized = re2.ReplaceAllString(sanitized, "-")
	// Trim hyphens from edges
	sanitized = strings.Trim(sanitized, "-")
	return sanitized
}

// GenerateImageID generates a unique, human-readable ID for an image configuration.
// Format: {sanitized-image-name}-{tag}-{first-8-chars-of-hash}
// Example: alpine-latest-a1b2c3d4
func GenerateImageID(
	imageName, imageTag string,
	cpu, memory int,
	runtimePlatform string,
	taskRoleName, taskExecutionRoleName *string,
) string {
	roleComposite := buildRoleComposite(taskRoleName, taskExecutionRoleName)

	// Create hash input from full configuration
	hashInput := fmt.Sprintf("%s:%s:%d:%d:%s:%s", imageName, imageTag, cpu, memory, runtimePlatform, roleComposite)

	// Compute SHA256 hash
	hash := sha256.Sum256([]byte(hashInput))
	hashStr := fmt.Sprintf("%x", hash)

	// Take first 8 characters of hash
	shortHash := hashStr[:8]

	// Sanitize image name (remove registry, replace invalid chars)
	sanitizedName := sanitizeImageNameForID(imageName)

	// Build ID: {sanitized-name}-{tag}-{hash}
	imageID := fmt.Sprintf("%s-%s-%s", sanitizedName, imageTag, shortHash)

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

	// Convert to strings for DynamoDB storage
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
		// Just a name, no registry
		name = parts[0]
	} else {
		// Check if first part is a registry
		firstPart := parts[0]
		if strings.Contains(firstPart, ".") ||
			strings.Contains(firstPart, ":") ||
			firstPart == "localhost" {
			// This is a registry
			name = parts[1]
		} else {
			// This is org/repo format (no registry)
			name = remainder
		}
	}

	return name, tag
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
	// Parse image to get name and tag
	imageName, imageTag := parseImageReference(image)

	// Apply defaults if not provided
	cpuVal := DefaultCPU
	if cpu != nil {
		cpuVal = *cpu
	}
	memoryVal := DefaultMemory
	if memory != nil {
		memoryVal = *memory
	}
	runtimePlatformVal := DefaultRuntimePlatform
	if runtimePlatform != nil && *runtimePlatform != "" {
		runtimePlatformVal = *runtimePlatform
	}

	// Generate ImageID from configuration
	imageID := GenerateImageID(
		imageName,
		imageTag,
		cpuVal,
		memoryVal,
		runtimePlatformVal,
		taskRoleName,
		taskExecutionRoleName,
	)

	// Query by ImageID
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

	// Convert from strings to ints
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
		Cpu:                   cpuInt,
		Memory:                memoryInt,
		RuntimePlatform:       item.RuntimePlatform,
		ImageRegistry:         item.ImageRegistry,
		ImageName:             item.ImageName,
		ImageTag:              item.ImageTag,
	}, nil
}

// ListImages retrieves all registered images with their task definitions.
// Shows all configurations (no deduplication).
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

	// Sort by image name, then by ImageID for consistency
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

		// Convert from strings to ints
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
			Cpu:                   cpuInt,
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
		Cpu:                   cpuInt,
		Memory:                memoryInt,
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

// DeleteImage removes all task definition mappings for a specific image.
// Returns success if image is not found (idempotent operation).
//
//nolint:funlen // Complex error handling for validation errors
func (r *ImageTaskDefRepository) DeleteImage(ctx context.Context, image string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Scan table and filter by image attribute
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
	})
	if err != nil {
		return apperrors.ErrInternalError("failed to scan image mappings", err)
	}

	var items []imageTaskDefItem
	if unmarshalErr := attributevalue.UnmarshalListOfMaps(result.Items, &items); unmarshalErr != nil {
		return apperrors.ErrInternalError("failed to unmarshal image items", unmarshalErr)
	}

	// If no items found, image doesn't exist - return success (idempotent operation)
	if len(items) == 0 {
		reqLogger.Debug("image not found, nothing to delete", "context", map[string]string{
			"image": image,
		})
		return nil
	}

	// Delete each item by image_id
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
			// Check if it's a validation error (item doesn't exist or wrong key schema)
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) && apiErr.ErrorCode() == "ValidationException" {
				// Item might not exist or have wrong schema - log and continue
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
// This is useful when you need to find a task definition for an image regardless of
// its specific CPU/Memory/RuntimePlatform configuration.
// It scans by image attribute and returns the first matching item,
// preferring the default configuration if available.
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

	// Scan table and filter by image attribute
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

	if len(result.Items) == 0 {
		return nil, nil
	}

	var items []imageTaskDefItem
	if unmarshalErr := attributevalue.UnmarshalListOfMaps(result.Items, &items); unmarshalErr != nil {
		return nil, apperrors.ErrInternalError("failed to unmarshal image-taskdef items", unmarshalErr)
	}

	// Helper function to convert item to ImageInfo
	convertItem := func(item *imageTaskDefItem) (*api.ImageInfo, error) {
		// Convert from strings to ints
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
			Cpu:                   cpuInt,
			Memory:                memoryInt,
			RuntimePlatform:       item.RuntimePlatform,
			ImageRegistry:         item.ImageRegistry,
			ImageName:             item.ImageName,
			ImageTag:              item.ImageTag,
		}, nil
	}

	// Prefer default configuration if available
	for i := range items {
		if items[i].isDefault() {
			return convertItem(&items[i])
		}
	}

	// If no default found, return the first one
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

// SetImageAsOnlyDefault marks a specific image configuration as the only default.
// It first unmarks all other defaults, then sets this one as default.
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

	// Generate ImageID for the default configuration
	imageName, imageTag := parseImageReference(image)
	imageID := GenerateImageID(
		imageName,
		imageTag,
		DefaultCPU,
		DefaultMemory,
		DefaultRuntimePlatform,
		taskRoleName,
		taskExecutionRoleName,
	)

	// Then mark this one as default
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
