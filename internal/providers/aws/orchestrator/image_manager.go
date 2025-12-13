package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/runvoy/runvoy/internal/api"
	apperrors "github.com/runvoy/runvoy/internal/errors"
	"github.com/runvoy/runvoy/internal/logger"
	awsClient "github.com/runvoy/runvoy/internal/providers/aws/client"
	awsConstants "github.com/runvoy/runvoy/internal/providers/aws/constants"
	"github.com/runvoy/runvoy/internal/providers/aws/database/dynamodb"
	"github.com/runvoy/runvoy/internal/providers/aws/ecsdefs"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// ImageRegistryImpl implements the ImageRegistry interface for AWS ECS and DynamoDB.
// It handles Docker image registration, listing, retrieval, and removal.
type ImageRegistryImpl struct {
	ecsClient awsClient.ECSClient
	iamClient awsClient.IAMClient
	imageRepo ImageTaskDefRepository
	cfg       *Config
	logger    *slog.Logger
}

// NewImageRegistry creates a new AWS image manager.
func NewImageRegistry(
	ecsClient awsClient.ECSClient,
	iamClient awsClient.IAMClient,
	imageRepo ImageTaskDefRepository,
	cfg *Config,
	log *slog.Logger,
) *ImageRegistryImpl {
	return &ImageRegistryImpl{
		ecsClient: ecsClient,
		iamClient: iamClient,
		imageRepo: imageRepo,
		cfg:       cfg,
		logger:    log,
	}
}

// RegisterImage registers a Docker image with optional custom IAM roles, CPU, Memory, and RuntimePlatform.
// Creates a new task definition with a unique family name and stores the mapping in DynamoDB.
//
//nolint:funlen // Complex registration flow with multiple steps
func (m *ImageRegistryImpl) RegisterImage(
	ctx context.Context,
	image string,
	isDefault *bool,
	taskRoleName *string,
	taskExecutionRoleName *string,
	cpu *int,
	memory *int,
	runtimePlatform *string,
	createdBy string,
) error {
	if m.ecsClient == nil {
		return errors.New("ECS client not configured")
	}
	if m.imageRepo == nil {
		return errors.New("image repository not configured")
	}

	reqLogger := logger.DeriveRequestLogger(ctx, m.logger)

	region := m.cfg.Region
	if region == "" {
		return errors.New("AWS region not configured")
	}

	if m.cfg.AccountID == "" {
		return errors.New("AWS account ID not configured")
	}

	// Validate IAM roles exist before proceeding
	if err := m.validateIAMRoles(ctx, taskRoleName, taskExecutionRoleName, region, reqLogger); err != nil {
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

	existing, err := m.imageRepo.GetImageTaskDef(
		ctx, image, taskRoleName, taskExecutionRoleName, cpu, memory, runtimePlatform,
	)
	if err != nil {
		return fmt.Errorf("failed to check existing image-taskdef mapping: %w", err)
	}

	if existing != nil {
		return m.handleExistingImage(
			ctx, image, isDefault, taskRoleName, taskExecutionRoleName,
			existing, reqLogger,
		)
	}

	taskDefARN, family, err := m.registerNewImage(
		ctx, image, isDefault, taskRoleName, taskExecutionRoleName,
		region,
		cpuVal, memoryVal, runtimePlatformVal,
		createdBy,
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

// ListImages lists all registered Docker images from DynamoDB.
func (m *ImageRegistryImpl) ListImages(ctx context.Context) ([]api.ImageInfo, error) {
	if m.imageRepo == nil {
		return nil, errors.New("image repository not configured")
	}

	images, err := m.imageRepo.ListImages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list images from repository: %w", err)
	}

	return images, nil
}

// GetImage retrieves a single Docker image by ID or name.
// Accepts either an ImageID (e.g., "alpine:latest-a1b2c3d4") or an image name (e.g., "alpine:latest").
// If ImageID is provided, queries directly by ID. Otherwise, uses GetAnyImageTaskDef to find any configuration.
// If image is empty, returns the default image if one is configured.
func (m *ImageRegistryImpl) GetImage(ctx context.Context, image string) (*api.ImageInfo, error) {
	if m.imageRepo == nil {
		return nil, errors.New("image repository not configured")
	}

	// If no image specified, try to get the default image
	if image == "" {
		imageInfo, err := m.imageRepo.GetDefaultImage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get default image: %w", err)
		}
		if imageInfo == nil {
			return nil, apperrors.ErrBadRequest("no image specified and no default image configured", nil)
		}
		return imageInfo, nil
	}

	var imageInfo *api.ImageInfo
	var err error

	if looksLikeImageID(image) {
		imageInfo, err = m.imageRepo.GetImageTaskDefByID(ctx, image)
		if err != nil {
			return nil, fmt.Errorf("failed to get image by ImageID: %w", err)
		}
	} else {
		imageInfo, err = m.imageRepo.GetAnyImageTaskDef(ctx, image)
		if err != nil {
			return nil, fmt.Errorf("failed to get image: %w", err)
		}
	}

	if imageInfo == nil {
		return nil, apperrors.ErrNotFound(
			"image not found: "+image,
			nil,
		)
	}

	return imageInfo, nil
}

// RemoveImage removes a Docker image and all its task definition variants from DynamoDB.
// It also deregisters all associated task definitions from ECS.
// If deregistration fails for any task definition, it continues to clean up the remaining ones
// and still removes the mappings from DynamoDB.
//
// NOTE: To avoid accidental deletion of multiple image configurations, this function requires
// the full ImageID (e.g., "alpine:latest-a1b2c3d4") instead of just the image name/tag.
// Use ListImages to find the specific ImageID you want to remove.
//
//nolint:gocyclo,funlen // Complex deletion flow with pagination, deregistration, and deletion
func (m *ImageRegistryImpl) RemoveImage(ctx context.Context, image string) error {
	if m.imageRepo == nil {
		return errors.New("image repository not configured")
	}
	if m.ecsClient == nil {
		return errors.New("ECS client not configured")
	}

	reqLogger := logger.DeriveRequestLogger(ctx, m.logger)

	if m.cfg.AccountID == "" {
		return errors.New("AWS account ID not configured")
	}

	var matchingImages []api.ImageInfo

	if looksLikeImageID(image) {
		imageInfo, getErr := m.imageRepo.GetImageTaskDefByID(ctx, image)
		if getErr != nil {
			return fmt.Errorf("failed to get image by ImageID: %w", getErr)
		}
		if imageInfo != nil {
			matchingImages = []api.ImageInfo{*imageInfo}
		}
	} else {
		// For unregistering, require the exact ImageID to avoid any ambiguity
		// This ensures there's no mismatch between what the user intended and what gets deleted
		return apperrors.ErrBadRequest(
			fmt.Sprintf(
				"image unregister requires exact ImageID (e.g., \"alpine:latest-a1b2c3d4\"). "+
					"Use 'images list' to find the exact ImageID for %q",
				image,
			),
			nil,
		)
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
		logArgs := []any{
			"operation", "ECS.ListTaskDefinitions",
			"family", family,
			"image", image,
			"status", string(ecsTypes.TaskDefinitionStatusActive),
			"paginated", "true",
		}
		logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

		for page := range listTaskDefinitionPages(ctx, m.ecsClient, ecs.ListTaskDefinitionsInput{
			FamilyPrefix: awsStd.String(family),
			Status:       ecsTypes.TaskDefinitionStatusActive,
			MaxResults:   awsStd.Int32(awsConstants.ECSTaskDefinitionMaxResults),
		}) {
			if page.Err != nil {
				reqLogger.Error("failed to list task definitions for family", "context",
					map[string]string{
						"error":  page.Err.Error(),
						"family": family,
					})
				break
			}

			// Collect ARNs for batch deletion
			var taskDefARNsToDelete []string
			for _, taskDefARN := range page.Output.TaskDefinitionArns {
				deregLogArgs := []any{
					"operation", "ECS.DeregisterTaskDefinition",
					"task_definition", taskDefARN,
					"family", family,
				}
				deregLogArgs = append(deregLogArgs, logger.GetDeadlineInfo(ctx)...)
				reqLogger.Debug("calling external service", "context", logger.SliceToMap(deregLogArgs))

				_, deregErr := m.ecsClient.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
					TaskDefinition: awsStd.String(taskDefARN),
				})
				if deregErr != nil {
					reqLogger.Warn("failed to deregister task definition", "error", deregErr, "arn", taskDefARN, "family", family)
				} else {
					taskDefARNsToDelete = append(taskDefARNsToDelete, taskDefARN)
					totalDeregistered++
					reqLogger.Debug("deregistered task definition revision", "context", map[string]string{
						"family": family,
						"image":  image,
						"arn":    taskDefARN,
					})
				}
			}

			// Delete the deregistered task definitions
			if len(taskDefARNsToDelete) > 0 {
				m.deleteTaskDefinitions(ctx, reqLogger, family, image, taskDefARNsToDelete)
			}
		}
	}

	if deleteErr := m.imageRepo.DeleteImage(ctx, image); deleteErr != nil {
		return fmt.Errorf("failed to delete image from repository: %w", deleteErr)
	}

	reqLogger.Info("image removed successfully", "context", map[string]any{
		"image":                    image,
		"task_definitions_removed": totalDeregistered,
	})

	return nil
}

// deleteTaskDefinitions deletes task definitions and logs the results.
func (m *ImageRegistryImpl) deleteTaskDefinitions(
	ctx context.Context,
	reqLogger *slog.Logger,
	family, image string,
	taskDefARNsToDelete []string,
) {
	deleteLogArgs := []any{
		"operation", "ECS.DeleteTaskDefinitions",
		"task_definitions_count", len(taskDefARNsToDelete),
		"family", family,
	}
	deleteLogArgs = append(deleteLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(deleteLogArgs))

	deleteOutput, deleteErr := m.ecsClient.DeleteTaskDefinitions(ctx, &ecs.DeleteTaskDefinitionsInput{
		TaskDefinitions: taskDefARNsToDelete,
	})
	if deleteErr != nil {
		reqLogger.Warn(
			"failed to delete task definitions",
			"error", deleteErr,
			"family", family,
			"count", len(taskDefARNsToDelete),
		)
		return
	}

	// Log successful deletions - the output contains deleted task definition ARNs
	if deleteOutput != nil {
		// The DeleteTaskDefinitions API returns deleted ARNs in the response
		// Log each successfully deleted task definition
		for _, deletedARN := range taskDefARNsToDelete {
			reqLogger.Debug("deleted task definition", "context", map[string]string{
				"family": family,
				"image":  image,
				"arn":    deletedARN,
			})
		}
		// Log any failures if present
		if len(deleteOutput.Failures) > 0 {
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

// buildRoleARNs constructs task and execution role ARNs from names or config defaults.
// The execution role ARN is always required and defaults to DefaultTaskExecRoleARN from config.
func (m *ImageRegistryImpl) buildRoleARNs(
	taskRoleName *string,
	taskExecutionRoleName *string,
	region string,
) (taskRoleARN, taskExecRoleARN string) {
	taskRoleARN = ""
	taskExecRoleARN = m.cfg.DefaultTaskExecRoleARN

	if taskRoleName != nil && *taskRoleName != "" {
		taskRoleARN = buildRoleARN(taskRoleName, m.cfg.AccountID, region)
	} else if m.cfg.DefaultTaskRoleARN != "" {
		taskRoleARN = m.cfg.DefaultTaskRoleARN
	}

	if taskExecutionRoleName != nil && *taskExecutionRoleName != "" {
		taskExecRoleARN = buildRoleARN(taskExecutionRoleName, m.cfg.AccountID, region)
	}

	return taskRoleARN, taskExecRoleARN
}

// determineDefaultStatus determines if an image should be marked as default.
// If isDefault is nil, it automatically marks the image as default if no default image exists.
func (m *ImageRegistryImpl) determineDefaultStatus(
	ctx context.Context,
	isDefault *bool,
) (bool, error) {
	if isDefault != nil {
		return *isDefault, nil
	}

	defaultImg, defaultErr := m.imageRepo.GetDefaultImage(ctx)
	if defaultErr != nil {
		return false, fmt.Errorf("failed to check for default image: %w", defaultErr)
	}
	return defaultImg == nil, nil
}

// handleExistingImage handles the case when an image already exists.
func (m *ImageRegistryImpl) handleExistingImage(
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
		if setErr := m.imageRepo.SetImageAsOnlyDefault(ctx, image, taskRoleName, taskExecutionRoleName); setErr != nil {
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
func (m *ImageRegistryImpl) registerNewImage(
	ctx context.Context,
	image string,
	isDefault *bool,
	taskRoleName, taskExecutionRoleName *string,
	region string,
	cpu, memory int,
	runtimePlatform string,
	createdBy string,
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

	taskRoleARN, taskExecRoleARN := m.buildRoleARNs(taskRoleName, taskExecutionRoleName, region)

	shouldBeDefault, err := m.determineDefaultStatus(ctx, isDefault)
	if err != nil {
		return "", "", err
	}

	taskDefARN, err = m.registerTaskDefinitionWithRoles(
		ctx,
		family,
		image,
		taskRoleARN,
		taskExecRoleARN,
		region,
		cpu,
		memory,
		runtimePlatform,
		shouldBeDefault,
		reqLogger,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to register ECS task definition: %w", err)
	}

	if shouldBeDefault {
		if unmarkErr := m.imageRepo.UnmarkAllDefaults(ctx); unmarkErr != nil {
			return "", "", fmt.Errorf("failed to unmark existing defaults: %w", unmarkErr)
		}
	}

	if putErr := m.imageRepo.PutImageTaskDef(
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
		createdBy,
	); putErr != nil {
		return "", "", fmt.Errorf("failed to store image-taskdef mapping: %w", putErr)
	}

	return taskDefARN, family, nil
}

// validateIAMRoles validates that the specified IAM roles exist in AWS.
// Returns an error if any role does not exist.
func (m *ImageRegistryImpl) validateIAMRoles(
	ctx context.Context,
	taskRoleName *string,
	taskExecutionRoleName *string,
	region string,
	reqLogger *slog.Logger,
) error {
	if m.iamClient == nil {
		return errors.New("IAM client not configured")
	}

	rolesToValidate := []struct {
		name *string
		arn  string
		kind string
	}{}

	if taskRoleName != nil && *taskRoleName != "" {
		taskRoleARN := buildRoleARN(taskRoleName, m.cfg.AccountID, region)
		rolesToValidate = append(rolesToValidate, struct {
			name *string
			arn  string
			kind string
		}{taskRoleName, taskRoleARN, "task"})
	}

	if taskExecutionRoleName != nil && *taskExecutionRoleName != "" {
		taskExecRoleARN := buildRoleARN(taskExecutionRoleName, m.cfg.AccountID, region)
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

		_, err := m.iamClient.GetRole(ctx, &iam.GetRoleInput{
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

// registerTaskDefinitionWithRoles registers a task definition with the specified roles,
// CPU, Memory, and RuntimePlatform.
//
//nolint:funlen // Complex AWS API orchestration with registration and tagging
func (m *ImageRegistryImpl) registerTaskDefinitionWithRoles(
	ctx context.Context,
	family string,
	image string,
	taskRoleARN string,
	taskExecRoleARN string,
	region string,
	cpu, memory int,
	runtimePlatform string,
	isDefault bool,
	reqLogger *slog.Logger,
) (string, error) {
	registerInput := BuildTaskDefinitionInput(
		ctx,
		family,
		image,
		taskExecRoleARN,
		taskRoleARN,
		region,
		cpu,
		memory,
		runtimePlatform,
		m.cfg,
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

	output, err := m.ecsClient.RegisterTaskDefinition(ctx, registerInput)
	if err != nil {
		return "", fmt.Errorf("ECS RegisterTaskDefinition failed: %w", err)
	}

	if output.TaskDefinition == nil || output.TaskDefinition.TaskDefinitionArn == nil {
		return "", errors.New("ECS returned nil task definition")
	}

	taskDefARN := *output.TaskDefinition.TaskDefinitionArn

	var isDefaultPtr *bool
	if isDefault {
		isDefaultPtr = awsStd.Bool(true)
	}
	tags := ecsdefs.BuildTaskDefinitionTags(image, isDefaultPtr)
	if len(tags) > 0 {
		tagLogArgs := []any{
			"operation", "ECS.TagResource",
			"task_definition_arn", taskDefARN,
			"family", family,
		}
		tagLogArgs = append(tagLogArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("calling external service", "context", logger.SliceToMap(tagLogArgs))

		_, tagErr := m.ecsClient.TagResource(ctx, &ecs.TagResourceInput{
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

	reqLogger.Debug("task definition registered", "context", map[string]string{
		"family":              family,
		"task_definition_arn": taskDefARN,
	})

	return taskDefARN, nil
}
