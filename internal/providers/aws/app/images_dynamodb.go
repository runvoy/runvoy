// Package aws provides AWS-specific implementations for runvoy.
// This file contains image management using DynamoDB for image-taskdef mappings.
package aws

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"
	awsConstants "runvoy/internal/providers/aws/constants"
	"runvoy/internal/providers/aws/database/dynamodb"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
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
	region string,
	cpu, memory int,
	runtimePlatform string,
	reqLogger *slog.Logger,
) (taskDefARN, family string, err error) {
	imageRef := ParseImageReference(image)

	imageID := dynamodb.GenerateImageID(
		imageRef.Name,
		imageRef.Tag,
		cpu,
		memory,
		runtimePlatform,
		taskRoleName,
		taskExecutionRoleName,
	)

	family = sanitizeImageIDForTaskDef(imageID)

	taskRoleARN, taskExecRoleARN := e.buildRoleARNs(taskRoleName, taskExecutionRoleName, region)

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

// validateIAMRoles validates that the specified IAM roles exist in AWS.
// Returns an error if any role does not exist.
func (e *Runner) validateIAMRoles(
	ctx context.Context,
	taskRoleName *string,
	taskExecutionRoleName *string,
	region string,
	reqLogger *slog.Logger,
) error {
	if e.iamClient == nil {
		return fmt.Errorf("IAM client not configured")
	}

	rolesToValidate := []struct {
		name *string
		arn  string
		kind string
	}{}

	if taskRoleName != nil && *taskRoleName != "" {
		taskRoleARN := buildRoleARN(taskRoleName, e.cfg.AccountID, region)
		rolesToValidate = append(rolesToValidate, struct {
			name *string
			arn  string
			kind string
		}{taskRoleName, taskRoleARN, "task"})
	}

	if taskExecutionRoleName != nil && *taskExecutionRoleName != "" {
		taskExecRoleARN := buildRoleARN(taskExecutionRoleName, e.cfg.AccountID, region)
		rolesToValidate = append(rolesToValidate, struct {
			name *string
			arn  string
			kind string
		}{taskExecutionRoleName, taskExecRoleARN, "task execution"})
	}

	for _, role := range rolesToValidate {
		roleName := *role.name
		logArgs := []any{
			"operation", "IAM.GetRole",
			"role_name", roleName,
			"role_kind", role.kind,
		}
		logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("validating IAM role", "context", logger.SliceToMap(logArgs))

		_, err := e.iamClient.GetRole(ctx, &iam.GetRoleInput{
			RoleName: awsStd.String(roleName),
		})
		if err != nil {
			var noSuchEntity *iamTypes.NoSuchEntityException
			if errors.As(err, &noSuchEntity) {
				return apperrors.ErrBadRequest(
					fmt.Sprintf("%s IAM role does not exist: %s", role.kind, roleName),
					err,
				)
			}
			return fmt.Errorf("failed to validate %s IAM role %s: %w", role.kind, roleName, err)
		}
	}

	return nil
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

	// Validate IAM roles exist before proceeding
	if err := e.validateIAMRoles(ctx, taskRoleName, taskExecutionRoleName, region, reqLogger); err != nil {
		return err
	}

	// Apply defaults for missing values
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
		region,
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
	cpuStr := fmt.Sprintf("%d", cpu)
	memoryStr := fmt.Sprintf("%d", memory)
	registerInput := buildTaskDefinitionInput(
		ctx, family, image, taskExecRoleARN, taskRoleARN, region, cpuStr, memoryStr, runtimePlatform, e.cfg,
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

	var matchingImages []api.ImageInfo

	if looksLikeImageID(image) {
		imageInfo, getErr := e.imageRepo.GetImageTaskDefByID(ctx, image)
		if getErr != nil {
			return fmt.Errorf("failed to get image by ImageID: %w", getErr)
		}
		if imageInfo != nil {
			matchingImages = []api.ImageInfo{*imageInfo}
		}
	} else {
		allImages, listErr := e.imageRepo.ListImages(ctx)
		if listErr != nil {
			return fmt.Errorf("failed to list images: %w", listErr)
		}

		for i := range allImages {
			if allImages[i].Image == image && allImages[i].TaskDefinitionName != "" {
				matchingImages = append(matchingImages, allImages[i])
			}
		}
	}

	if len(matchingImages) == 0 {
		return apperrors.ErrNotFound("image not found", fmt.Errorf("image %q not found", image))
	}

	families := make(map[string]bool)
	for i := range matchingImages {
		if matchingImages[i].TaskDefinitionName != "" {
			families[matchingImages[i].TaskDefinitionName] = true
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

// sanitizeImageIDForTaskDef sanitizes an ImageID for use as an ECS task definition family name.
// ECS task definition family names must match [a-zA-Z0-9_-]+ (no dots or other special chars).
// Replaces invalid characters (dots, etc.) with hyphens.
func sanitizeImageIDForTaskDef(imageID string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := re.ReplaceAllString(imageID, "-")
	re2 := regexp.MustCompile(`-+`)
	sanitized = re2.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")
	return fmt.Sprintf("runvoy-%s", sanitized)
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

// GetImage retrieves a single Docker image by ID or name.
// Accepts either an ImageID (e.g., "alpine:latest-a1b2c3d4") or an image name (e.g., "alpine:latest").
// If ImageID is provided, queries directly by ID. Otherwise, uses GetAnyImageTaskDef to find any configuration.
func (e *Runner) GetImage(ctx context.Context, image string) (*api.ImageInfo, error) {
	if e.imageRepo == nil {
		return nil, fmt.Errorf("image repository not configured")
	}

	var imageInfo *api.ImageInfo
	var err error

	if looksLikeImageID(image) {
		imageInfo, err = e.imageRepo.GetImageTaskDefByID(ctx, image)
		if err != nil {
			return nil, fmt.Errorf("failed to get image by ImageID: %w", err)
		}
	} else {
		imageInfo, err = e.imageRepo.GetAnyImageTaskDef(ctx, image)
		if err != nil {
			return nil, fmt.Errorf("failed to get image: %w", err)
		}
	}

	if imageInfo == nil {
		return nil, fmt.Errorf("image not found: %s", image)
	}

	return imageInfo, nil
}

// GetTaskDefinitionARNForImage returns the task definition family name for a specific image or ImageID.
// Accepts either an ImageID (e.g., "alpine:latest-a1b2c3d4") or an image name (e.g., "alpine:latest").
// If ImageID is provided, queries directly by ID. Otherwise, uses GetAnyImageTaskDef to find any configuration.
// Returns just the family name - ECS will automatically use the latest ACTIVE revision when running tasks.
func (e *Runner) GetTaskDefinitionARNForImage(ctx context.Context, image string) (string, error) {
	if e.imageRepo == nil {
		return "", fmt.Errorf("image repository not configured")
	}

	var imageInfo *api.ImageInfo
	var err error

	if looksLikeImageID(image) {
		imageInfo, err = e.imageRepo.GetImageTaskDefByID(ctx, image)
		if err != nil {
			return "", fmt.Errorf("failed to get task definition by ImageID: %w", err)
		}
	} else {
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
