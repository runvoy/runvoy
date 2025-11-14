// Package aws provides AWS-specific implementations for runvoy.
// This file contains image management using DynamoDB for image-taskdef mappings.
package aws

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/api"
	"runvoy/internal/auth"
	"runvoy/internal/logger"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// buildRoleARN constructs a full IAM role ARN from a role name and account ID.
// Returns an empty string if roleName is nil or empty.
func buildRoleARN(roleName *string, accountID, _ string) string {
	if roleName == nil || *roleName == "" {
		return ""
	}
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, *roleName)
}

// buildTaskDefinitionARN constructs a task definition ARN from family name.
// Since each family is only registered once, the revision is always 1.
// This is used for DeregisterTaskDefinition which requires a full ARN with revision.
// For RunTask, we use just the family name so ECS picks the latest active revision.
func (e *Runner) buildTaskDefinitionARN(family, region string) string {
	return fmt.Sprintf("arn:aws:ecs:%s:%s:task-definition/%s:1", region, e.cfg.AccountID, family)
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
// It generates a unique task definition family name using UUID, registers the task definition
// with ECS, and stores the mapping in DynamoDB.
func (e *Runner) registerNewImage(
	ctx context.Context,
	image string,
	isDefault *bool,
	taskRoleName, taskExecutionRoleName *string,
	taskRoleARN, taskExecRoleARN, region string,
	reqLogger *slog.Logger,
) (taskDefARN, family string, err error) {
	family = fmt.Sprintf("runvoy-taskdef-%s", auth.GenerateUUID())

	taskDefARN, err = e.registerTaskDefinitionWithRoles(
		ctx,
		family,
		image,
		taskRoleARN,
		taskExecRoleARN,
		region,
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

	imageRef := ParseImageReference(image)

	if putErr := e.imageRepo.PutImageTaskDef(
		ctx,
		image,
		imageRef.Registry,
		imageRef.Name,
		imageRef.Tag,
		taskRoleName,
		taskExecutionRoleName,
		family,
		shouldBeDefault,
	); putErr != nil {
		return "", "", fmt.Errorf("failed to store image-taskdef mapping: %w", putErr)
	}

	return taskDefARN, family, nil
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

	if e.cfg.AccountID == "" {
		return fmt.Errorf("AWS account ID not configured")
	}

	taskRoleARN, taskExecRoleARN := e.buildRoleARNs(taskRoleName, taskExecutionRoleName, region)

	existing, err := e.imageRepo.GetImageTaskDef(ctx, image, taskRoleName, taskExecutionRoleName)
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
		taskRoleARN, taskExecRoleARN, region, reqLogger,
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
	region := e.cfg.Region

	allImages, err := e.imageRepo.ListImages(ctx)
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	var taskDefsToDeregister []string
	for i := range allImages {
		if allImages[i].Image == image && allImages[i].TaskDefinitionName != "" {
			taskDefARN := e.buildTaskDefinitionARN(allImages[i].TaskDefinitionName, region)
			taskDefsToDeregister = append(taskDefsToDeregister, taskDefARN)
		}
	}

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
		}
	}

	if deleteErr := e.imageRepo.DeleteImage(ctx, image); deleteErr != nil {
		return fmt.Errorf("failed to delete image from repository: %w", deleteErr)
	}

	reqLogger.Info("image removed successfully", "context", map[string]string{
		"image":                    image,
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

// GetTaskDefinitionARNForImage returns the task definition family name for a specific image from DynamoDB.
// Uses default roles (from config) to look up the task definition by querying with nil role names.
// Returns just the family name - ECS will automatically use the latest ACTIVE revision when running tasks.
func (e *Runner) GetTaskDefinitionARNForImage(ctx context.Context, image string) (string, error) {
	if e.imageRepo == nil {
		return "", fmt.Errorf("image repository not configured")
	}

	imageInfo, err := e.imageRepo.GetImageTaskDef(ctx, image, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get task definition for image: %w", err)
	}

	if imageInfo == nil {
		return "", fmt.Errorf("no task definition found for image: %s", image)
	}

	return imageInfo.TaskDefinitionName, nil
}
