// Package aws provides AWS-specific implementations for runvoy.
// This file contains image management using DynamoDB for image-taskdef mappings.
package aws

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"runvoy/internal/api"
	"runvoy/internal/logger"
	awsConstants "runvoy/internal/providers/aws/constants"
	"runvoy/internal/providers/aws/database/dynamodb"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// buildRoleARN constructs a full IAM role ARN from a role name and account ID.
// Returns an empty string if roleName is nil or empty.
func buildRoleARN(roleName *string, accountID, _ string) string {
	if roleName == nil || *roleName == "" {
		return ""
	}
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, *roleName)
}

// buildRoleARNs constructs task and execution role ARNs from names or config defaults.
// The execution role ARN is always required and defaults to DefaultTaskExecRoleARN from config.
func (e *Runner) buildRoleARNs(
	taskRoleName *string,
	taskExecutionRoleName *string,
	region string,
) (taskRoleARN, taskExecRoleARN string) {
	taskRoleARN = ""
	taskExecRoleARN = e.cfg.DefaultTaskExecRoleARN

	if taskRoleName != nil && *taskRoleName != "" {
		taskRoleARN = buildRoleARN(taskRoleName, e.cfg.AccountID, region)
	} else if e.cfg.DefaultTaskRoleARN != "" {
		taskRoleARN = e.cfg.DefaultTaskRoleARN
	}

	if taskExecutionRoleName != nil && *taskExecutionRoleName != "" {
		taskExecRoleARN = buildRoleARN(taskExecutionRoleName, e.cfg.AccountID, region)
	}

	return taskRoleARN, taskExecRoleARN
}

// determineDefaultStatus determines if an image should be marked as default.
// If isDefault is nil, it automatically marks the image as default if no default image exists.
func (e *Runner) determineDefaultStatus(
	ctx context.Context,
	isDefault *bool,
) (bool, error) {
	if isDefault != nil {
		return *isDefault, nil
	}

	defaultImg, defaultErr := e.imageRepo.GetDefaultImage(ctx)
	if defaultErr != nil {
		return false, fmt.Errorf("failed to check for default image: %w", defaultErr)
	}
	return defaultImg == nil, nil
}

// handleExistingImage handles the case when an image already exists.
func (e *Runner) handleExistingImage(
	ctx context.Context,
	image string,
	isDefault *bool,
	taskRoleName, taskExecutionRoleName *string,
	existing *api.ImageInfo,
	reqLogger *slog.Logger,
) error {
	reqLogger.Debug("image-taskdef mapping already exists", "context", map[string]string{
		"image":                  image,
		"task_definition_family": existing.TaskDefinitionName,
	})

	shouldBeDefault := isDefault != nil && *isDefault
	if shouldBeDefault {
		if setErr := e.imageRepo.SetImageAsOnlyDefault(ctx, image, taskRoleName, taskExecutionRoleName); setErr != nil {
			return fmt.Errorf("failed to set image as default: %w", setErr)
		}
	}

	return nil
}

// registerNewImage handles registration of a new image.
// It generates a unique ImageID, uses it as the task definition family name (prefixed with "runvoy-"),
// registers the task definition with ECS, and stores the mapping in DynamoDB.
//
//nolint:funlen // Complex registration flow with multiple steps
func (e *Runner) registerNewImage(
	ctx context.Context,
	image string,
	isDefault *bool,
	taskRoleName, taskExecutionRoleName *string,
	taskRoleARN, taskExecRoleARN, region string,
	cpu, memory int,
	runtimePlatform string,
	reqLogger *slog.Logger,
) (taskDefARN, family string, err error) {
	imageRef := ParseImageReference(image)

	// Generate ImageID from configuration
	imageID := dynamodb.GenerateImageID(
		imageRef.Name,
		imageRef.Tag,
		cpu,
		memory,
		runtimePlatform,
		taskRoleName,
		taskExecutionRoleName,
	)

	// Use ImageID (prefixed with "runvoy-") as task definition family name
	family = fmt.Sprintf("runvoy-%s", imageID)

	taskDefARN, err = e.registerTaskDefinitionWithRoles(
		ctx,
		family,
		image,
		taskRoleARN,
		taskExecRoleARN,
		region,
		cpu,
		memory,
		runtimePlatform,
		reqLogger,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to register ECS task definition: %w", err)
	}

	shouldBeDefault, err := e.determineDefaultStatus(ctx, isDefault)
	if err != nil {
		return "", "", err
	}

	if shouldBeDefault {
		if unmarkErr := e.imageRepo.UnmarkAllDefaults(ctx); unmarkErr != nil {
			return "", "", fmt.Errorf("failed to unmark existing defaults: %w", unmarkErr)
		}
	}

	if putErr := e.imageRepo.PutImageTaskDef(
		ctx,
		imageID,
		image,
		imageRef.Registry,
		imageRef.Name,
		imageRef.Tag,
		taskRoleName,
		taskExecutionRoleName,
		cpu,
		memory,
		runtimePlatform,
		family,
		shouldBeDefault,
	); putErr != nil {
		return "", "", fmt.Errorf("failed to store image-taskdef mapping: %w", putErr)
	}

	return taskDefARN, family, nil
}

// RegisterImage registers a Docker image with optional custom IAM roles, CPU, Memory, and RuntimePlatform.
// Creates a new task definition with a unique family name and stores the mapping in DynamoDB.
//
//nolint:funlen // Complex registration flow with multiple steps
func (e *Runner) RegisterImage(
	ctx context.Context,
	image string,
	isDefault *bool,
	taskRoleName *string,
	taskExecutionRoleName *string,
	cpu *int,
	memory *int,
	runtimePlatform *string,
) error {
	if e.ecsClient == nil {
		return fmt.Errorf("ECS client not configured")
	}
	if e.imageRepo == nil {
		return fmt.Errorf("image repository not configured")
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	region := e.cfg.Region
	if region == "" {
		return fmt.Errorf("AWS region not configured")
	}

	if e.cfg.AccountID == "" {
		return fmt.Errorf("AWS account ID not configured")
	}

	taskRoleARN, taskExecRoleARN := e.buildRoleARNs(taskRoleName, taskExecutionRoleName, region)

	// Apply defaults for missing values
	cpuVal := dynamodb.DefaultCPU
	if cpu != nil {
		cpuVal = *cpu
	}
	memoryVal := dynamodb.DefaultMemory
	if memory != nil {
		memoryVal = *memory
	}
	runtimePlatformVal := dynamodb.DefaultRuntimePlatform
	if runtimePlatform != nil && *runtimePlatform != "" {
		runtimePlatformVal = *runtimePlatform
	}

	existing, err := e.imageRepo.GetImageTaskDef(
		ctx, image, taskRoleName, taskExecutionRoleName, cpu, memory, runtimePlatform,
	)
	if err != nil {
		return fmt.Errorf("failed to check existing image-taskdef mapping: %w", err)
	}

	if existing != nil {
		return e.handleExistingImage(
			ctx, image, isDefault, taskRoleName, taskExecutionRoleName,
			existing, reqLogger,
		)
	}

	taskDefARN, family, err := e.registerNewImage(
		ctx, image, isDefault, taskRoleName, taskExecutionRoleName,
		taskRoleARN, taskExecRoleARN, region,
		cpuVal, memoryVal, runtimePlatformVal,
		reqLogger,
	)
	if err != nil {
		return err
	}

	reqLogger.Info("image registered successfully", "context", map[string]string{
		"image":                  image,
		"task_definition_arn":    taskDefARN,
		"task_definition_family": family,
	})

	return nil
}

// registerTaskDefinitionWithRoles registers a task definition with the specified roles,
// CPU, Memory, and RuntimePlatform.
//
//nolint:funlen // Complex AWS API orchestration with registration and tagging
func (e *Runner) registerTaskDefinitionWithRoles(
	ctx context.Context,
	family string,
	image string,
	taskRoleARN string,
	taskExecRoleARN string,
	region string,
	cpu, memory int,
	runtimePlatform string,
	reqLogger *slog.Logger,
) (string, error) {
	// Convert to strings for ECS API
	cpuStr := fmt.Sprintf("%d", cpu)
	memoryStr := fmt.Sprintf("%d", memory)
	registerInput := buildTaskDefinitionInput(
		family, image, taskExecRoleARN, taskRoleARN, region, cpuStr, memoryStr, runtimePlatform, e.cfg,
	)

	logArgs := []any{
		"operation", "ECS.RegisterTaskDefinition",
		"family", family,
		"image", image,
		"task_role_arn", taskRoleARN,
		"task_exec_role_arn", taskExecRoleARN,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	output, err := e.ecsClient.RegisterTaskDefinition(ctx, registerInput)
	if err != nil {
		return "", fmt.Errorf("ECS RegisterTaskDefinition failed: %w", err)
	}

	if output.TaskDefinition == nil || output.TaskDefinition.TaskDefinitionArn == nil {
		return "", fmt.Errorf("ECS returned nil task definition")
	}

	taskDefARN := *output.TaskDefinition.TaskDefinitionArn

	tags := buildTaskDefinitionTags(image, nil)
	if len(tags) > 0 {
		tagLogArgs := []any{
			"operation", "ECS.TagResource",
			"task_definition_arn", taskDefARN,
			"family", family,
		}
		tagLogArgs = append(tagLogArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("calling external service", "context", logger.SliceToMap(tagLogArgs))

		_, tagErr := e.ecsClient.TagResource(ctx, &ecs.TagResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
			Tags:        tags,
		})
		if tagErr != nil {
			reqLogger.Warn(
				"failed to tag task definition (task definition registered successfully)",
				"arn", taskDefARN,
				"error", tagErr,
			)
			// Continue even if tagging fails - task definition is still registered
		} else {
			reqLogger.Debug("task definition tagged successfully", "arn", taskDefARN)
		}
	}

	reqLogger.Info("task definition registered", "context", map[string]string{
		"family":              family,
		"task_definition_arn": taskDefARN,
	})

	return taskDefARN, nil
}

// ListImages lists all registered Docker images from DynamoDB.
func (e *Runner) ListImages(ctx context.Context) ([]api.ImageInfo, error) {
	if e.imageRepo == nil {
		return nil, fmt.Errorf("image repository not configured")
	}

	images, err := e.imageRepo.ListImages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list images from repository: %w", err)
	}

	return images, nil
}

// RemoveImage removes a Docker image and all its task definition variants from DynamoDB.
// It also deregisters all associated task definitions from ECS.
// If deregistration fails for any task definition, it continues to clean up the remaining ones
// and still removes the mappings from DynamoDB.
//
//nolint:gocyclo,funlen // Complex deletion flow with pagination, deregistration, and deletion
func (e *Runner) RemoveImage(ctx context.Context, image string) error {
	if e.imageRepo == nil {
		return fmt.Errorf("image repository not configured")
	}
	if e.ecsClient == nil {
		return fmt.Errorf("ECS client not configured")
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	if e.cfg.AccountID == "" {
		return fmt.Errorf("AWS account ID not configured")
	}

	allImages, err := e.imageRepo.ListImages(ctx)
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	// Collect unique task definition families for this image
	families := make(map[string]bool)
	for i := range allImages {
		if allImages[i].Image == image && allImages[i].TaskDefinitionName != "" {
			families[allImages[i].TaskDefinitionName] = true
		}
	}

	// Deregister all task definition revisions for each family
	totalDeregistered := 0
	for family := range families {
		nextToken := ""
		logArgs := []any{
			"operation", "ECS.ListTaskDefinitions",
			"family", family,
			"image", image,
			"status", string(ecsTypes.TaskDefinitionStatusActive),
			"paginated", "true",
		}
		logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

		for {
			listOutput, listErr := e.ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
				FamilyPrefix: awsStd.String(family),
				Status:       ecsTypes.TaskDefinitionStatusActive,
				MaxResults:   awsStd.Int32(awsConstants.ECSTaskDefinitionMaxResults),
				NextToken:    awsStd.String(nextToken),
			})
			if listErr != nil {
				reqLogger.Warn("failed to list task definitions for family", "error", listErr, "family", family)
				break
			}

			// Collect ARNs for batch deletion
			var taskDefARNsToDelete []string
			for _, taskDefARN := range listOutput.TaskDefinitionArns {
				deregLogArgs := []any{
					"operation", "ECS.DeregisterTaskDefinition",
					"task_definition", taskDefARN,
					"family", family,
				}
				deregLogArgs = append(deregLogArgs, logger.GetDeadlineInfo(ctx)...)
				reqLogger.Debug("calling external service", "context", logger.SliceToMap(deregLogArgs))

				_, deregErr := e.ecsClient.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
					TaskDefinition: awsStd.String(taskDefARN),
				})
				if deregErr != nil {
					reqLogger.Warn("failed to deregister task definition", "error", deregErr, "arn", taskDefARN, "family", family)
				} else {
					taskDefARNsToDelete = append(taskDefARNsToDelete, taskDefARN)
					totalDeregistered++
					reqLogger.Info("deregistered task definition revision", "context", map[string]string{
						"family": family,
						"image":  image,
						"arn":    taskDefARN,
					})
				}
			}

			// Delete the deregistered task definitions
			if len(taskDefARNsToDelete) > 0 {
				deleteLogArgs := []any{
					"operation", "ECS.DeleteTaskDefinitions",
					"task_definitions_count", len(taskDefARNsToDelete),
					"family", family,
				}
				deleteLogArgs = append(deleteLogArgs, logger.GetDeadlineInfo(ctx)...)
				reqLogger.Debug("calling external service", "context", logger.SliceToMap(deleteLogArgs))

				deleteOutput, deleteErr := e.ecsClient.DeleteTaskDefinitions(ctx, &ecs.DeleteTaskDefinitionsInput{
					TaskDefinitions: taskDefARNsToDelete,
				})
				if deleteErr != nil {
					reqLogger.Warn(
						"failed to delete task definitions",
						"error", deleteErr,
						"family", family,
						"count", len(taskDefARNsToDelete),
					)
				} else {
					// Log successful deletions - the output contains deleted task definition ARNs
					if deleteOutput != nil {
						// The DeleteTaskDefinitions API returns deleted ARNs in the response
						// Log each successfully deleted task definition
						for _, deletedARN := range taskDefARNsToDelete {
							reqLogger.Info("deleted task definition", "context", map[string]string{
								"family": family,
								"image":  image,
								"arn":    deletedARN,
							})
						}
					}
					// Log any failures if present
					if deleteOutput != nil && deleteOutput.Failures != nil && len(deleteOutput.Failures) > 0 {
						for _, failure := range deleteOutput.Failures {
							reqLogger.Warn("task definition deletion failed", "context", map[string]string{
								"family": family,
								"arn":    awsStd.ToString(failure.Arn),
								"reason": awsStd.ToString(failure.Reason),
								"detail": awsStd.ToString(failure.Detail),
							})
						}
					}
				}
			}

			if listOutput.NextToken == nil {
				break
			}
			nextToken = *listOutput.NextToken
		}
	}

	if deleteErr := e.imageRepo.DeleteImage(ctx, image); deleteErr != nil {
		return fmt.Errorf("failed to delete image from repository: %w", deleteErr)
	}

	reqLogger.Info("image removed successfully", "context", map[string]any{
		"image":                    image,
		"task_definitions_removed": totalDeregistered,
	})

	return nil
}

// GetDefaultImageFromDB returns the default Docker image from DynamoDB.
func (e *Runner) GetDefaultImageFromDB(ctx context.Context) (string, error) {
	if e.imageRepo == nil {
		return "", fmt.Errorf("image repository not configured")
	}

	imageInfo, err := e.imageRepo.GetDefaultImage(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get default image: %w", err)
	}

	if imageInfo == nil {
		return "", nil
	}

	return imageInfo.Image, nil
}

// looksLikeImageID checks if a string looks like an ImageID format.
// ImageID format: {name}-{tag}-{8-char-hash}
func looksLikeImageID(s string) bool {
	// ImageID should have at least 2 hyphens (name-tag-hash)
	// and the last part should be 8 characters (hash)
	const minParts = 3
	const hashLength = 8
	parts := strings.Split(s, "-")
	if len(parts) < minParts {
		return false
	}
	// Check if last part looks like a hash (8 hex characters)
	lastPart := parts[len(parts)-1]
	if len(lastPart) == hashLength {
		// Check if it's hexadecimal
		for _, c := range lastPart {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
				return false
			}
		}
		return true
	}
	return false
}

// GetTaskDefinitionARNForImage returns the task definition family name for a specific image or ImageID.
// Accepts either an ImageID (e.g., "alpine-latest-a1b2c3d4") or an image name (e.g., "alpine:latest").
// If ImageID is provided, queries directly by ID. Otherwise, uses GetAnyImageTaskDef to find any configuration.
// Returns just the family name - ECS will automatically use the latest ACTIVE revision when running tasks.
func (e *Runner) GetTaskDefinitionARNForImage(ctx context.Context, image string) (string, error) {
	if e.imageRepo == nil {
		return "", fmt.Errorf("image repository not configured")
	}

	var imageInfo *api.ImageInfo
	var err error

	// Check if input looks like an ImageID
	if looksLikeImageID(image) {
		// Query by ImageID directly
		imageInfo, err = e.imageRepo.GetImageTaskDefByID(ctx, image)
		if err != nil {
			return "", fmt.Errorf("failed to get task definition by ImageID: %w", err)
		}
	} else {
		// Use GetAnyImageTaskDef to find any configuration for this image
		// This is more flexible than requiring exact CPU/Memory/RuntimePlatform match
		imageInfo, err = e.imageRepo.GetAnyImageTaskDef(ctx, image)
		if err != nil {
			return "", fmt.Errorf("failed to get task definition for image: %w", err)
		}
	}

	if imageInfo == nil {
		return "", fmt.Errorf("no task definition found for image: %s", image)
	}

	return imageInfo.TaskDefinitionName, nil
}
