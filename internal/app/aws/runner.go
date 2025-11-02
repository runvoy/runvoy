// Package aws provides AWS-specific implementations for runvoy.
// It handles ECS task execution and AWS service integration.
package aws

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// Config holds AWS-specific execution configuration.
type Config struct {
	ECSCluster      string
	TaskDefinition  string
	Subnet1         string
	Subnet2         string
	SecurityGroup   string
	LogGroup        string
	DefaultImage    string
	TaskRoleARN     string
	TaskExecRoleARN string
	Region          string
}

// Runner implements app.Runner for AWS ECS Fargate.
type Runner struct {
	ecsClient *ecs.Client
	cfg       *Config
	logger    *slog.Logger
}

// NewRunner creates a new AWS ECS runner with the provided configuration.
func NewRunner(ecsClient *ecs.Client, cfg *Config, logger *slog.Logger) *Runner {
	return &Runner{ecsClient: ecsClient, cfg: cfg, logger: logger}
}

// FetchLogsByExecutionID returns CloudWatch log events for the given execution ID.
func (e *Runner) FetchLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error) {
	return FetchLogsByExecutionID(ctx, e.cfg, executionID)
}

// buildSidecarContainerCommand constructs the shell command for the sidecar container.
// It handles .env file creation from user environment variables and git repository cloning.
func buildSidecarContainerCommand(hasGitRepo bool) []string {
	commands := []string{"set -e"}

	// Create .env file from user environment variables (if any)
	commands = append(commands,
		"if env | grep -q '^RUNVOY_USER_'; then",
		fmt.Sprintf("  echo '### %s sidecar: Creating .env file from user environment variables'", constants.ProjectName),
		"  ENV_FILE_PATH=\"${RUNVOY_SHARED_VOLUME_PATH}/.env\"",
		"  env | grep '^RUNVOY_USER_' | while IFS='=' read -r key value; do",
		"    actual_key=\"${key#RUNVOY_USER_}\"",
		"    echo \"${actual_key}=${value}\" >> \"${ENV_FILE_PATH}\"",
		"  done",
		"  if [ -f \"${ENV_FILE_PATH}\" ]; then",
		fmt.Sprintf("    echo '### %s sidecar: .env file created with' $(wc -l < \"${ENV_FILE_PATH}\") 'variables at' \"${ENV_FILE_PATH}\"", constants.ProjectName),
		"  else",
		fmt.Sprintf("    echo '### %s sidecar: No .env file created'", constants.ProjectName),
		"  fi",
		"else",
		fmt.Sprintf("  echo '### %s sidecar: No RUNVOY_USER_* variables found, skipping .env creation'", constants.ProjectName),
		"fi",
	)

	// Git repository cloning logic (if specified)
	if hasGitRepo {
		commands = append(commands,
			"apk add --no-cache git",
			"GIT_REF=${GIT_REF:-main}",
			"CLONE_PATH=${RUNVOY_SHARED_VOLUME_PATH}/repo",
			fmt.Sprintf("echo '### %s sidecar: Cloning ${GIT_REPO} (ref: ${GIT_REF})'", constants.ProjectName),
			"git clone --depth 1 --branch \"${GIT_REF}\" \"${GIT_REPO}\" \"${CLONE_PATH}\"",
			fmt.Sprintf("echo '### %s sidecar: Clone completed successfully'", constants.ProjectName),
			"if [ -f \"${RUNVOY_SHARED_VOLUME_PATH}/.env\" ]; then",
			"  cp \"${RUNVOY_SHARED_VOLUME_PATH}/.env\" \"${CLONE_PATH}/.env\"",
			fmt.Sprintf("  echo '### %s sidecar: .env file copied to repo directory'", constants.ProjectName),
			"fi",
			"ls -la \"${CLONE_PATH}\"",
		)
	} else {
		// If no GIT_REPO is specified, just exit successfully
		commands = append(commands,
			fmt.Sprintf("echo '### %s sidecar: No git repository specified, exiting'", constants.ProjectName),
		)
	}

	commands = append(commands,
		fmt.Sprintf("echo '### %s sidecar: Sidecar completed successfully'", constants.ProjectName),
	)

	return []string{"/bin/sh", "-c", strings.Join(commands, "\n")}
}

// buildMainContainerCommand constructs the shell command for the main runner container.
// It adds logging statements and optionally changes to the git repo working directory.
func buildMainContainerCommand(req api.ExecutionRequest, requestID string, hasGitRepo bool) []string {
	commands := []string{
		fmt.Sprintf("printf '### %s runner execution started by requestID %s\\n'",
			constants.ProjectName, requestID),
	}

	if hasGitRepo {
		workDir := constants.SharedVolumePath + "/repo"
		if req.GitPath != "" && req.GitPath != "." {
			workDir = workDir + "/" + strings.TrimPrefix(req.GitPath, "/")
		}
		commands = append(commands,
			fmt.Sprintf("cd %s", workDir),
			fmt.Sprintf("printf '### %s working directory: %s\\n'", constants.ProjectName, workDir),
		)
	}

	commands = append(commands,
		fmt.Sprintf("printf '### %s command => %s\\n'", constants.ProjectName, req.Command),
		req.Command,
	)

	return []string{"/bin/sh", "-c", strings.Join(commands, " && ")}
}

// StartTask triggers an ECS Fargate task and returns identifiers.
func (e *Runner) StartTask(ctx context.Context, userEmail string, req api.ExecutionRequest) (string, *time.Time, error) {
	if e.ecsClient == nil {
		return "", nil, appErrors.ErrInternalError("ECS cli endpoint not configured", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	imageToUse := e.cfg.DefaultImage
	if req.Image != "" {
		imageToUse = req.Image
	}

	taskDefARN, err := GetTaskDefinitionForImage(ctx, e.ecsClient, imageToUse, reqLogger)
	if err != nil {
		return "", nil, appErrors.ErrBadRequest("image not registered", err)
	}

	reqLogger.Debug("using task definition for image", "image", imageToUse, "taskDef", taskDefARN)

	hasGitRepo := req.GitRepo != ""
	requestID := logger.GetRequestID(ctx)
	envVars := []ecsTypes.KeyValuePair{
		{Name: awsStd.String("RUNVOY_COMMAND"), Value: awsStd.String(req.Command)},
	}
	for key, value := range req.Env {
		envVars = append(envVars, ecsTypes.KeyValuePair{
			Name:  awsStd.String(key),
			Value: awsStd.String(value),
		})
	}

	sidecarEnv := []ecsTypes.KeyValuePair{
		{Name: awsStd.String("RUNVOY_SHARED_VOLUME_PATH"),
			Value: awsStd.String(constants.SharedVolumePath)},
	}

	for key, value := range req.Env {
		sidecarEnv = append(sidecarEnv, ecsTypes.KeyValuePair{
			Name:  awsStd.String(key),
			Value: awsStd.String(value),
		})
	}

	if hasGitRepo {
		gitRef := req.GitRef
		if gitRef == "" {
			gitRef = "main"
		}
		sidecarEnv = append(sidecarEnv,
			ecsTypes.KeyValuePair{Name: awsStd.String("GIT_REPO"), Value: awsStd.String(req.GitRepo)},
			ecsTypes.KeyValuePair{Name: awsStd.String("GIT_REF"), Value: awsStd.String(gitRef)},
		)
		reqLogger.Debug("configured sidecar for git cloning",
			"gitRepo", req.GitRepo,
			"gitRef", gitRef)
	} else {
		sidecarEnv = append(sidecarEnv,
			ecsTypes.KeyValuePair{Name: awsStd.String("GIT_REPO"), Value: awsStd.String("")},
		)
		reqLogger.Debug("sidecar configured without git (will exit 0)")
	}

	containerOverrides := []ecsTypes.ContainerOverride{
		{
			Name:        awsStd.String(constants.SidecarContainerName),
			Command:     buildSidecarContainerCommand(hasGitRepo),
			Environment: sidecarEnv,
		},
		{
			Name:        awsStd.String(constants.RunnerContainerName),
			Command:     buildMainContainerCommand(req, requestID, hasGitRepo),
			Environment: envVars,
		},
	}

	// Build task tags
	tags := []ecsTypes.Tag{
		{Key: awsStd.String("UserEmail"), Value: awsStd.String(userEmail)},
	}
	if hasGitRepo {
		tags = append(tags, ecsTypes.Tag{
			Key:   awsStd.String("HasGitRepo"),
			Value: awsStd.String("true"),
		})
	}

	runTaskInput := &ecs.RunTaskInput{
		Cluster:        awsStd.String(e.cfg.ECSCluster),
		TaskDefinition: awsStd.String(taskDefARN),
		LaunchType:     ecsTypes.LaunchTypeFargate,
		Overrides: &ecsTypes.TaskOverride{
			ContainerOverrides: containerOverrides,
		},
		NetworkConfiguration: &ecsTypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecsTypes.AwsVpcConfiguration{
				Subnets:        []string{e.cfg.Subnet1, e.cfg.Subnet2},
				SecurityGroups: []string{e.cfg.SecurityGroup},
				AssignPublicIp: ecsTypes.AssignPublicIpEnabled,
			},
		},
		Tags: tags,
	}

	logArgs := []any{
		"operation", "ECS.RunTask",
		"cluster", e.cfg.ECSCluster,
		"taskDefinition", taskDefARN,
		"image", imageToUse,
		"containerCount", len(containerOverrides),
		"userEmail", userEmail,
		"hasGitRepo", hasGitRepo,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	runTaskOutput, err := e.ecsClient.RunTask(ctx, runTaskInput)
	if err != nil {
		return "", nil, appErrors.ErrInternalError("failed to start ECS task", err)
	}
	if len(runTaskOutput.Tasks) == 0 {
		return "", nil, appErrors.ErrInternalError("no tasks were started", nil)
	}

	task := runTaskOutput.Tasks[0]
	taskARN := awsStd.ToString(task.TaskArn)
	executionIDParts := strings.Split(taskARN, "/")
	executionID := executionIDParts[len(executionIDParts)-1]
	createdAt := task.CreatedAt

	reqLogger.Debug("task started", "task", map[string]string{
		"taskARN":     taskARN,
		"executionID": executionID,
	})

	// Add ExecutionID tag to the task for easier tracking (best-effort)
	tagLogArgs := []any{
		"operation", "ECS.TagResource",
		"taskARN", taskARN,
		"executionID", executionID,
		"createdAt", createdAt,
	}
	tagLogArgs = append(tagLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(tagLogArgs))

	_, tagErr := e.ecsClient.TagResource(ctx, &ecs.TagResourceInput{
		ResourceArn: awsStd.String(taskARN),
		Tags:        []ecsTypes.Tag{{Key: awsStd.String("ExecutionID"), Value: awsStd.String(executionID)}},
	})
	if tagErr != nil {
		reqLogger.Warn(
			"failed to add ExecutionID tag to task",
			"error", tagErr,
			"taskARN", taskARN,
			"executionID", executionID)
	}

	return executionID, createdAt, nil
}

// RegisterImage registers a Docker image and creates the corresponding task definition.
// isDefault: if true, explicitly set as default; if nil or false, becomes default only if no default exists (first image behavior)
func (e *Runner) RegisterImage(ctx context.Context, image string, isDefault *bool) error {
	if e.ecsClient == nil {
		return fmt.Errorf("ECS client not configured")
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	region := e.cfg.Region
	if region == "" {
		return fmt.Errorf("AWS region not configured")
	}

	// Determine if this image should be marked as default:
	// 1. If isDefault is explicitly true, set as default
	// 2. If image matches cfg.DefaultImage, set as default
	// 3. If isDefault is nil (not provided) and no default exists, make this the first default
	shouldBeDefault := false
	if isDefault != nil && *isDefault {
		// Explicitly requested to be default
		shouldBeDefault = true
	} else if e.cfg.DefaultImage != "" && image == e.cfg.DefaultImage {
		// Matches config default image
		shouldBeDefault = true
	} else if isDefault == nil {
		// isDefault not provided - check if any default exists; if not, make this one the default (first image behavior)
		hasDefault, err := hasExistingDefaultImage(ctx, e.ecsClient, reqLogger)
		if err != nil {
			reqLogger.Warn("failed to check for existing default image, proceeding", "error", err)
			// Continue without setting as default if we can't check
		} else if !hasDefault {
			// No default exists, make this one the default (first image becomes default)
			shouldBeDefault = true
		}
	}
	// If isDefault is explicitly false, don't set as default (even if no default exists)

	_, err := RegisterTaskDefinitionForImage(ctx, e.ecsClient, e.cfg, image, shouldBeDefault, region, reqLogger)
	if err != nil {
		return fmt.Errorf("failed to register task definition: %w", err)
	}

	return nil
}

// ListImages lists all registered Docker images.
func (e *Runner) ListImages(ctx context.Context) ([]api.ImageInfo, error) {
	if e.ecsClient == nil {
		return nil, fmt.Errorf("ECS client not configured")
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	reqLogger.Debug("calling external service", "context", map[string]string{
		"operation": "ECS.ListTaskDefinitions",
		"status":    "active",
		"paginated": "true",
	})

	taskDefArns, err := listTaskDefinitionsByPrefix(ctx, e.ecsClient, constants.TaskDefinitionFamilyPrefix+"-")
	if err != nil {
		return nil, err
	}

	result := make([]api.ImageInfo, 0)
	seenImages := make(map[string]bool)

	for _, taskDefARN := range taskDefArns {
		descOutput, err := e.ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: awsStd.String(taskDefARN),
		})
		if err != nil {
			reqLogger.Warn("failed to describe task definition", "arn", taskDefARN, "error", err)
			continue
		}

		if descOutput.TaskDefinition == nil {
			continue
		}

		// Extract image from container definition (runner container)
		image := ""
		familyName := ""
		if descOutput.TaskDefinition.Family != nil {
			familyName = *descOutput.TaskDefinition.Family
		}
		for _, container := range descOutput.TaskDefinition.ContainerDefinitions {
			if container.Name != nil {
				if *container.Name == constants.RunnerContainerName && container.Image != nil {
					image = *container.Image
					reqLogger.Debug("found runner container image", "container", map[string]string{
						"family":         familyName,
						"container_name": *container.Name,
						"image":          *container.Image,
					})
					break
				}
			}
		}

		if image == "" {
			reqLogger.Debug("no runner container found in task definition", "container", map[string]string{
				"family":          familyName,
				"container_count": fmt.Sprintf("%d", len(descOutput.TaskDefinition.ContainerDefinitions)),
			})
		}

		isDefault := false
		tagsOutput, err := e.ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
		})
		if err == nil && tagsOutput != nil {
			for _, tag := range tagsOutput.Tags {
				if tag.Key != nil && tag.Value != nil {
					switch *tag.Key {
					case "runvoy.default":
						if *tag.Value == "true" {
							isDefault = true
						}
					}
				}
			}
		}

		if !isDefault && image != "" && image == e.cfg.DefaultImage {
			isDefault = true
		}

		if image != "" && !seenImages[image] {
			seenImages[image] = true
			family := awsStd.ToString(descOutput.TaskDefinition.Family)
			taskDefARN := awsStd.ToString(descOutput.TaskDefinition.TaskDefinitionArn)
			result = append(result, api.ImageInfo{
				Image:              image,
				TaskDefinitionARN:  taskDefARN,
				TaskDefinitionName: family,
				IsDefault:          isDefault,
			})
		}
	}

	return result, nil
}

// RemoveImage removes a Docker image and deregisters its task definitions.
func (e *Runner) RemoveImage(ctx context.Context, image string) error {
	if e.ecsClient == nil {
		return fmt.Errorf("ECS client not configured")
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	if err := DeregisterTaskDefinitionsForImage(ctx, e.ecsClient, image, reqLogger); err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}

	return nil
}

// KillTask terminates an ECS task identified by executionID.
// It checks the task status before termination and only stops tasks that are RUNNING or ACTIVATING.
// Returns an error if the task is already terminated or not found.
func (e *Runner) KillTask(ctx context.Context, executionID string) error {
	if e.ecsClient == nil {
		return appErrors.ErrInternalError("ECS client not configured", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	// First, describe the task to check its current status
	// We can use ListTasks to find the task ARN, or construct it from the execution ID
	// For ECS, we can use DescribeTasks with just the task ID (execution ID) if we know the cluster
	// However, AWS ECS requires the full task ARN. Let's use ListTasks to find it first.
	listLogArgs := []any{
		"operation", "ECS.ListTasks",
		"cluster", e.cfg.ECSCluster,
		"desiredStatus", "RUNNING",
		"executionID", executionID,
	}
	listLogArgs = append(listLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(listLogArgs))

	listOutput, err := e.ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       awsStd.String(e.cfg.ECSCluster),
		DesiredStatus: ecsTypes.DesiredStatusRunning,
	})
	if err != nil {
		reqLogger.Debug("failed to list tasks", "error", err, "executionID", executionID)
		return appErrors.ErrInternalError("failed to list tasks", err)
	}

	// Find the task ARN that matches our execution ID
	var taskARN string
	for _, arn := range listOutput.TaskArns {
		parts := strings.Split(arn, "/")
		if len(parts) > 0 && parts[len(parts)-1] == executionID {
			taskARN = arn
			break
		}
	}

	// If not found in running tasks, check stopped tasks
	if taskARN == "" {
		listStoppedLogArgs := []any{
			"operation", "ECS.ListTasks",
			"cluster", e.cfg.ECSCluster,
			"desiredStatus", "STOPPED",
			"executionID", executionID,
		}
		listStoppedLogArgs = append(listStoppedLogArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("calling external service", "context", logger.SliceToMap(listStoppedLogArgs))

		listStoppedOutput, err := e.ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:       awsStd.String(e.cfg.ECSCluster),
			DesiredStatus: ecsTypes.DesiredStatusStopped,
		})
		if err == nil {
			for _, arn := range listStoppedOutput.TaskArns {
				parts := strings.Split(arn, "/")
				if len(parts) > 0 && parts[len(parts)-1] == executionID {
					taskARN = arn
					break
				}
			}
		}
	}

	if taskARN == "" {
		reqLogger.Warn("task not found", "executionID", executionID)
		return appErrors.ErrNotFound("task not found", nil)
	}

	// Describe the task to check its current status
	describeLogArgs := []any{
		"operation", "ECS.DescribeTasks",
		"cluster", e.cfg.ECSCluster,
		"taskARN", taskARN,
		"executionID", executionID,
	}
	describeLogArgs = append(describeLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(describeLogArgs))

	describeOutput, err := e.ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: awsStd.String(e.cfg.ECSCluster),
		Tasks:   []string{taskARN},
	})
	if err != nil {
		reqLogger.Debug("failed to describe task", "error", err, "executionID", executionID, "taskARN", taskARN)
		return appErrors.ErrInternalError("failed to describe task", err)
	}

	if len(describeOutput.Tasks) == 0 {
		reqLogger.Warn("task not found", "executionID", executionID, "taskARN", taskARN)
		return appErrors.ErrNotFound("task not found", nil)
	}

	task := describeOutput.Tasks[0]
	currentStatus := awsStd.ToString(task.LastStatus)

	reqLogger.Debug("task status check", "executionID", executionID, "status", currentStatus)

	terminatedStatuses := []string{
		string(constants.EcsStatusStopped),
		string(constants.EcsStatusStopping),
		string(constants.EcsStatusDeactivating),
	}
	for _, status := range terminatedStatuses {
		if currentStatus == status {
			return appErrors.ErrBadRequest(
				"task is already terminated or terminating",
				fmt.Errorf("task status: %s", currentStatus))
		}
	}

	taskRunnableStatuses := []string{
		string(constants.EcsStatusRunning),
		string(constants.EcsStatusActivating),
	}
	if !slices.Contains(taskRunnableStatuses, string(constants.EcsStatus(currentStatus))) {
		return appErrors.ErrBadRequest(
			"task cannot be terminated in current state",
			fmt.Errorf(
				"task status: %s, expected: %s",
				currentStatus,
				strings.Join(taskRunnableStatuses, ", ")))
	}

	stopLogArgs := []any{
		"operation", "ECS.StopTask",
		"cluster", e.cfg.ECSCluster,
		"taskARN", taskARN,
		"executionID", executionID,
		"currentStatus", currentStatus,
	}
	stopLogArgs = append(stopLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(stopLogArgs))

	stopOutput, err := e.ecsClient.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: awsStd.String(e.cfg.ECSCluster),
		Task:    awsStd.String(taskARN),
		Reason:  awsStd.String("Terminated by user via kill endpoint"),
	})
	if err != nil {
		reqLogger.Error("failed to stop task", "error", err, "executionID", executionID, "taskARN", taskARN)
		return appErrors.ErrInternalError("failed to stop task", err)
	}

	reqLogger.Info(
		"task termination initiated",
		"executionID", executionID,
		"taskARN", awsStd.ToString(stopOutput.Task.TaskArn),
		"previousStatus", currentStatus)

	return nil
}
