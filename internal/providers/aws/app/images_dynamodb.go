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

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/google/uuid"
)

// extractAccountIDFromRoleARN extracts the AWS account ID from a role ARN.
// Example: arn:aws:iam::123456789012:role/MyRole -> 123456789012
func extractAccountIDFromRoleARN(roleARN string) (string, error) {
	parts := strings.Split(roleARN, ":")
	if len(parts) < 5 {
		return "", fmt.Errorf("invalid role ARN format: %s", roleARN)
	}
	return parts[4], nil
}

// buildRoleARN constructs a full IAM role ARN from a role name and account ID.
// If roleName is empty/nil, returns empty string.
func buildRoleARN(roleName *string, accountID, region string) string {
	if roleName == nil || *roleName == "" {
		return ""
	}
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, *roleName)
}

// RegisterImage registers a Docker image with optional custom IAM roles.
// Creates a new task definition with a unique family name and stores the mapping in DynamoDB.
func (e *Runner) RegisterImage(
	ctx context.Context,
	image string,
	isDefault *bool,
	taskRoleName *string,
	taskExecutionRoleName *string,
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

	// Extract account ID from the default task execution role ARN in config
	accountID, err := extractAccountIDFromRoleARN(e.cfg.DefaultTaskExecRoleARN)
	if err != nil {
		return fmt.Errorf("failed to extract account ID: %w", err)
	}

	// Build role ARNs from names, or use defaults from config
	taskRoleARN := ""
	taskExecRoleARN := e.cfg.DefaultTaskExecRoleARN // Always required

	if taskRoleName != nil && *taskRoleName != "" {
		taskRoleARN = buildRoleARN(taskRoleName, accountID, region)
	} else if e.cfg.DefaultTaskRoleARN != "" {
		taskRoleARN = e.cfg.DefaultTaskRoleARN
	}

	if taskExecutionRoleName != nil && *taskExecutionRoleName != "" {
		taskExecRoleARN = buildRoleARN(taskExecutionRoleName, accountID, region)
	}

	// Check if this exact combination already exists
	existing, existingARN, err := e.imageRepo.GetImageTaskDef(ctx, image, taskRoleName, taskExecutionRoleName)
	if err != nil {
		return fmt.Errorf("failed to check existing image-taskdef mapping: %w", err)
	}

	if existing != nil {
		reqLogger.Debug("image-taskdef mapping already exists", "context", map[string]string{
			"image":                 image,
			"task_definition_arn":   existingARN,
			"task_definition_family": existing.TaskDefinitionName,
		})

		// If isDefault is requested, update the default status
		shouldBeDefault := isDefault != nil && *isDefault
		if shouldBeDefault {
			if err := e.imageRepo.SetImageAsOnlyDefault(ctx, image, taskRoleName, taskExecutionRoleName); err != nil {
				return fmt.Errorf("failed to set image as default: %w", err)
			}
		}

		return nil
	}

	// Generate a unique task definition family name using UUID
	family := fmt.Sprintf("runvoy-taskdef-%s", uuid.New().String())

	// Register the task definition with ECS
	taskDefARN, err := e.registerTaskDefinitionWithRoles(
		ctx,
		family,
		image,
		taskRoleARN,
		taskExecRoleARN,
		region,
		reqLogger,
	)
	if err != nil {
		return fmt.Errorf("failed to register ECS task definition: %w", err)
	}

	// Determine if this should be the default
	shouldBeDefault := false
	if isDefault != nil {
		shouldBeDefault = *isDefault
	} else {
		// Auto-default if no default exists
		defaultImg, err := e.imageRepo.GetDefaultImage(ctx)
		if err != nil {
			return fmt.Errorf("failed to check for default image: %w", err)
		}
		shouldBeDefault = (defaultImg == nil)
	}

	// Handle default status
	if shouldBeDefault {
		if err := e.imageRepo.UnmarkAllDefaults(ctx); err != nil {
			return fmt.Errorf("failed to unmark existing defaults: %w", err)
		}
	}

	// Parse the image reference into components
	imageRef := ParseImageReference(image)

	// Store the mapping in DynamoDB
	if err := e.imageRepo.PutImageTaskDef(
		ctx,
		image,
		imageRef.Registry,
		imageRef.Name,
		imageRef.Tag,
		taskRoleName,
		taskExecutionRoleName,
		taskDefARN,
		family,
		shouldBeDefault,
	); err != nil {
		return fmt.Errorf("failed to store image-taskdef mapping: %w", err)
	}

	reqLogger.Info("image registered successfully", "context", map[string]string{
		"image":                  image,
		"task_definition_arn":    taskDefARN,
		"task_definition_family": family,
		"is_default":             fmt.Sprintf("%t", shouldBeDefault),
	})

	return nil
}

// registerTaskDefinitionWithRoles registers a task definition with the specified roles.
func (e *Runner) registerTaskDefinitionWithRoles(
	ctx context.Context,
	family string,
	image string,
	taskRoleARN string,
	taskExecRoleARN string,
	region string,
	reqLogger *slog.Logger,
) (string, error) {
	// Build the task definition input (reuse existing buildTaskDefinitionInput logic)
	registerInput := buildTaskDefinitionInput(family, image, taskExecRoleARN, taskRoleARN, region, e.cfg)

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

	reqLogger.Info("task definition registered", "context", map[string]string{
		"family":               family,
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
func (e *Runner) RemoveImage(ctx context.Context, image string) error {
	if e.imageRepo == nil {
		return fmt.Errorf("image repository not configured")
	}
	if e.ecsClient == nil {
		return fmt.Errorf("ECS client not configured")
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	// Get all task definition ARNs for this image
	taskDefsToDeregister, err := e.imageRepo.GetTaskDefARNsForImage(ctx, image)
	if err != nil {
		return fmt.Errorf("failed to get task definitions for image: %w", err)
	}

	// Deregister all task definitions from ECS
	for _, taskDefARN := range taskDefsToDeregister {
		logArgs := []any{
			"operation", "ECS.DeregisterTaskDefinition",
			"task_definition", taskDefARN,
		}
		logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

		_, deregErr := e.ecsClient.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: awsStd.String(taskDefARN),
		})
		if deregErr != nil {
			reqLogger.Warn("failed to deregister task definition", "error", deregErr, "arn", taskDefARN)
			// Continue anyway to clean up DynamoDB
		}
	}

	// Delete all mappings from DynamoDB
	if err := e.imageRepo.DeleteImage(ctx, image); err != nil {
		return fmt.Errorf("failed to delete image from repository: %w", err)
	}

	reqLogger.Info("image removed successfully", "context", map[string]string{
		"image":                image,
		"task_definitions_removed": fmt.Sprintf("%d", len(taskDefsToDeregister)),
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

// GetTaskDefinitionARNForImage returns the task definition ARN for a specific image from DynamoDB.
// Uses default roles (from config) to look up the task definition.
func (e *Runner) GetTaskDefinitionARNForImage(ctx context.Context, image string) (string, error) {
	if e.imageRepo == nil {
		return "", fmt.Errorf("image repository not configured")
	}

	// Query with nil role names to get the default role variant
	imageInfo, taskDefARN, err := e.imageRepo.GetImageTaskDef(ctx, image, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get task definition for image: %w", err)
	}

	if imageInfo == nil {
		return "", fmt.Errorf("no task definition found for image: %s", image)
	}

	return taskDefARN, nil
}
