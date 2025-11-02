// Package aws provides AWS-specific implementations for runvoy.
// It handles ECS task execution and AWS service integration.
package aws

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/logger"
	"runvoy/internal/database"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// Config holds AWS-specific execution configuration.
type Config struct {
	ECSCluster            string
	TaskDefinition        string
	TaskDefinitionWithGit string // Task definition with git-cloner sidecar
	Subnet1               string
	Subnet2               string
	SecurityGroup         string
	LogGroup              string
	DefaultImage          string
	TaskRoleARN           string
	TaskExecRoleARN       string
}

// Runner implements app.Runner for AWS ECS Fargate.
type Runner struct {
	ecsClient   *ecs.Client
	cfg         *Config
	logger      *slog.Logger
	taskDefRepo database.TaskDefinitionRepository
}

// NewRunner creates a new AWS ECS runner with the provided configuration.
func NewRunner(ecsClient *ecs.Client, cfg *Config, logger *slog.Logger, taskDefRepo database.TaskDefinitionRepository) *Runner {
	return &Runner{ecsClient: ecsClient, cfg: cfg, logger: logger, taskDefRepo: taskDefRepo}
}

// FetchLogsByExecutionID returns CloudWatch log events for the given execution ID.
func (e *Runner) FetchLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error) {
	return FetchLogsByExecutionID(ctx, e.cfg, executionID)
}

// computeTaskKey computes a deterministic hash key for a task definition based on image and hasGit flag.
func computeTaskKey(image string, hasGit bool) string {
	key := fmt.Sprintf("%s:%v", image, hasGit)
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])[:16] // Use first 16 chars for shorter keys
}

// buildMainContainerCommand constructs the shell command for the main runner container.
// It creates .env file from user environment variables, adds logging statements,
// and optionally changes to the git repo working directory.
func buildMainContainerCommand(req api.ExecutionRequest, requestID string, hasGitRepo bool) []string {
	commands := []string{
		fmt.Sprintf("printf '### %s runner execution started by requestID %s\\n'",
			constants.ProjectName, requestID),
	}

	// Create .env file from user environment variables (if any)
	// This works for both git and non-git flows
	if len(req.Env) > 0 {
		commands = append(commands, `printf '### runvoy: Creating .env file from user environment variables\n'`)

		// Extract RUNVOY_USER_* variables and write to .env file
		createEnvFileScript := `
env | grep '^RUNVOY_USER_' | while IFS='=' read -r key value; do
  actual_key="${key#RUNVOY_USER_}"
  echo "${actual_key}=${value}" >> .env
done
if [ -f .env ]; then
  printf '### runvoy: .env file created with %d variables\n' "$(wc -l < .env)"
else
  printf '### runvoy: No .env file created\n'
fi
`
		commands = append(commands, strings.TrimSpace(createEnvFileScript))
	}

	// If git repo is specified, change to the cloned directory
	if hasGitRepo {
		workDir := constants.SharedVolumePath + "/repo"
		if req.GitPath != "" && req.GitPath != "." {
			workDir = workDir + "/" + strings.TrimPrefix(req.GitPath, "/")
		}
		commands = append(commands,
			fmt.Sprintf("cd %s", workDir),
			fmt.Sprintf("printf '### %s working directory: %s\\n'", constants.ProjectName, workDir),
		)

		// If .env was created in the previous step, it's in the initial directory
		// For git repos, we also want .env in the repo directory
		if len(req.Env) > 0 {
			commands = append(commands, `if [ -f ../.env ]; then cp ../.env .env; fi`)
		}
	}

	commands = append(commands,
		fmt.Sprintf("printf '### %s command => %s\\n'", constants.ProjectName, req.Command),
		req.Command,
	)

	return []string{"/bin/sh", "-c", strings.Join(commands, " && ")}
}

// StartTask triggers an ECS Fargate task and returns identifiers.
func (e *Runner) StartTask(ctx context.Context, userEmail string, req api.ExecutionRequest) (string, string, *time.Time, error) {
	if e.ecsClient == nil {
		return "", "", nil, appErrors.ErrInternalError("ECS cli endpoint not configured", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	// Determine which image to use
	image := req.Image
	if image == "" {
		image = e.cfg.DefaultImage
	}

	// Determine if git repo is requested
	hasGitRepo := req.GitRepo != ""

	// Get or create task definition from registry
	taskDefinition, err := e.GetOrCreateTaskDefinition(ctx, image, hasGitRepo, userEmail)
	if err != nil {
		return "", "", nil, err
	}

	if hasGitRepo {
		reqLogger.Debug("using git-enabled task definition",
			"gitRepo", req.GitRepo,
			"gitRef", req.GitRef,
			"gitPath", req.GitPath,
			"image", image,
			"taskDefinition", taskDefinition)
	} else {
		reqLogger.Debug("using task definition",
			"image", image,
			"taskDefinition", taskDefinition)
	}

	// Extract request ID from context (set by middleware)
	requestID := logger.GetRequestID(ctx)

	// Build environment variables for main container
	// User env vars are passed with two formats:
	// 1. Direct: KEY=value (for direct access in shell)
	// 2. Prefixed: RUNVOY_USER_KEY=value (for .env file creation)
	envVars := []ecsTypes.KeyValuePair{
		{Name: awsStd.String("RUNVOY_COMMAND"), Value: awsStd.String(req.Command)},
	}
	for key, value := range req.Env {
		// Add direct env var (for container environment)
		envVars = append(envVars, ecsTypes.KeyValuePair{
			Name:  awsStd.String(key),
			Value: awsStd.String(value),
		})
		// Add prefixed env var (for .env file creation)
		envVars = append(envVars, ecsTypes.KeyValuePair{
			Name:  awsStd.String("RUNVOY_USER_" + key),
			Value: awsStd.String(value),
		})
	}

	// Build container overrides
	containerOverrides := []ecsTypes.ContainerOverride{
		{
			Name:        awsStd.String(constants.RunnerContainerName),
			Command:     buildMainContainerCommand(req, requestID, hasGitRepo),
			Environment: envVars,
		},
	}

	// If using git, add git-cloner sidecar container overrides
	if hasGitRepo {
		gitRef := req.GitRef
		if gitRef == "" {
			gitRef = "main"
		}

		containerOverrides = append(containerOverrides, ecsTypes.ContainerOverride{
			Name: awsStd.String(constants.GitClonerContainerName),
			Environment: []ecsTypes.KeyValuePair{
				{Name: awsStd.String("GIT_REPO"), Value: awsStd.String(req.GitRepo)},
				{Name: awsStd.String("GIT_REF"), Value: awsStd.String(gitRef)},
				{Name: awsStd.String("SHARED_VOLUME_PATH"), Value: awsStd.String(constants.SharedVolumePath)},
			},
		})

		reqLogger.Debug("configured git-cloner sidecar",
			"gitRepo", req.GitRepo,
			"gitRef", gitRef)
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
		TaskDefinition: awsStd.String(taskDefinition),
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

	// Log before calling ECS RunTask
	logArgs := []any{
		"operation", "ECS.RunTask",
		"cluster", e.cfg.ECSCluster,
		"taskDefinition", taskDefinition,
		"containerCount", len(containerOverrides),
		"userEmail", userEmail,
		"hasGitRepo", hasGitRepo,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	runTaskOutput, err := e.ecsClient.RunTask(ctx, runTaskInput)
	if err != nil {
		return "", "", nil, appErrors.ErrInternalError("failed to start ECS task", err)
	}
	if len(runTaskOutput.Tasks) == 0 {
		return "", "", nil, appErrors.ErrInternalError("no tasks were started", nil)
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

	return executionID, taskARN, createdAt, nil
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

// RegisterTaskDefinition creates a new ECS task definition for the specified image and returns task definition info.
func (e *Runner) RegisterTaskDefinition(ctx context.Context, image string, hasGit bool, userEmail string) (*api.TaskDefinition, error) {
	if e.ecsClient == nil {
		return nil, appErrors.ErrInternalError("ECS client not configured", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	// Determine which base task definition to use
	baseTaskDefARN := e.cfg.TaskDefinition
	if hasGit {
		if e.cfg.TaskDefinitionWithGit == "" {
			return nil, appErrors.ErrInternalError("git-enabled task definition not configured", nil)
		}
		baseTaskDefARN = e.cfg.TaskDefinitionWithGit
	}

	// Describe the base task definition to clone it
	describeLogArgs := []any{
		"operation", "ECS.DescribeTaskDefinition",
		"taskDefinition", baseTaskDefARN,
		"image", image,
		"hasGit", hasGit,
	}
	describeLogArgs = append(describeLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(describeLogArgs))

	baseTaskDef, err := e.ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: awsStd.String(baseTaskDefARN),
		Include: []ecsTypes.TaskDefinitionField{
			ecsTypes.TaskDefinitionFieldTags,
		},
	})
	if err != nil {
		return nil, appErrors.ErrInternalError("failed to describe base task definition", err)
	}

	// Tags are returned separately in DescribeTaskDefinitionOutput, not in TaskDefinition
	tags := baseTaskDef.Tags

	// Clone the container definitions and update the runner image
	containerDefs := make([]ecsTypes.ContainerDefinition, len(baseTaskDef.TaskDefinition.ContainerDefinitions))
	for i, container := range baseTaskDef.TaskDefinition.ContainerDefinitions {
		containerDefs[i] = container
		// Update the runner container's image
		if awsStd.ToString(container.Name) == constants.RunnerContainerName {
			containerDefs[i].Image = awsStd.String(image)
			reqLogger.Debug("updated runner container image", "oldImage", container.Image, "newImage", image)
		}
	}

	// Create a unique family name for this task definition
	taskKey := computeTaskKey(image, hasGit)
	familyName := fmt.Sprintf("runvoy-task-%s", taskKey)
	if hasGit {
		familyName = fmt.Sprintf("runvoy-task-%s-withgit", taskKey)
	}

	// Build the register input
	registerInput := &ecs.RegisterTaskDefinitionInput{
		Family:                  awsStd.String(familyName),
		NetworkMode:            baseTaskDef.TaskDefinition.NetworkMode,
		RequiresCompatibilities: baseTaskDef.TaskDefinition.RequiresCompatibilities,
		Cpu:                     baseTaskDef.TaskDefinition.Cpu,
		Memory:                  baseTaskDef.TaskDefinition.Memory,
		ExecutionRoleArn:       baseTaskDef.TaskDefinition.ExecutionRoleArn,
		TaskRoleArn:            baseTaskDef.TaskDefinition.TaskRoleArn,
		ContainerDefinitions:   containerDefs,
	}

	// Copy volumes if present (for git-enabled task definitions)
	if baseTaskDef.TaskDefinition.Volumes != nil {
		registerInput.Volumes = baseTaskDef.TaskDefinition.Volumes
	}

	// Copy ephemeral storage if present
	if baseTaskDef.TaskDefinition.EphemeralStorage != nil {
		registerInput.EphemeralStorage = baseTaskDef.TaskDefinition.EphemeralStorage
	}

	// Copy tags from base task definition (if present)
	if tags != nil {
		registerInput.Tags = append(registerInput.Tags, tags...)
	}
	// Add our own tags
	registerInput.Tags = append(registerInput.Tags, ecsTypes.Tag{
		Key:   awsStd.String("Image"),
		Value: awsStd.String(image),
	})
	registerInput.Tags = append(registerInput.Tags, ecsTypes.Tag{
		Key:   awsStd.String("RegisteredBy"),
		Value: awsStd.String(userEmail),
	})

	// Register the new task definition
	registerLogArgs := []any{
		"operation", "ECS.RegisterTaskDefinition",
		"family", familyName,
		"image", image,
		"hasGit", hasGit,
	}
	registerLogArgs = append(registerLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(registerLogArgs))

	registerOutput, err := e.ecsClient.RegisterTaskDefinition(ctx, registerInput)
	if err != nil {
		return nil, appErrors.ErrInternalError("failed to register task definition", err)
	}

	taskDefARN := awsStd.ToString(registerOutput.TaskDefinition.TaskDefinitionArn)
	reqLogger.Info("task definition registered successfully",
		"taskDefARN", taskDefARN,
		"image", image,
		"hasGit", hasGit,
		"family", familyName)

	return &api.TaskDefinition{
		TaskKey:           taskKey,
		Image:             image,
		HasGit:            hasGit,
		TaskDefinitionARN: taskDefARN,
		CreatedAt:         time.Now().UTC(),
		CreatedBy:         userEmail,
	}, nil
}

// GetOrCreateTaskDefinition retrieves a task definition from the registry, or creates it if it doesn't exist.
func (e *Runner) GetOrCreateTaskDefinition(ctx context.Context, image string, hasGit bool, userEmail string) (string, error) {
	if e.taskDefRepo == nil {
		// Fallback to default task definitions if registry is not available
		if hasGit {
			return e.cfg.TaskDefinitionWithGit, nil
		}
		return e.cfg.TaskDefinition, nil
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	taskKey := computeTaskKey(image, hasGit)

	// Try to get from registry
	taskDef, err := e.taskDefRepo.GetTaskDefinition(ctx, taskKey)
	if err != nil {
		return "", appErrors.ErrDatabaseError("failed to lookup task definition", err)
	}

	if taskDef != nil {
		// Update last used timestamp (best-effort, don't fail if it errors)
		_ = e.taskDefRepo.UpdateLastUsed(ctx, taskKey)
		reqLogger.Debug("found task definition in registry", "taskKey", taskKey, "taskDefARN", taskDef.TaskDefinitionARN)
		return taskDef.TaskDefinitionARN, nil
	}

	// Not found in registry - register it on-demand (hybrid approach)
	reqLogger.Info("task definition not found in registry, registering on-demand", "image", image, "hasGit", hasGit)

	newTaskDef, err := e.RegisterTaskDefinition(ctx, image, hasGit, userEmail)
	if err != nil {
		return "", err
	}

	// Store in registry (best-effort, don't fail if it errors)
	registryEntry := &api.TaskDefinition{
		TaskKey:           newTaskDef.TaskKey,
		Image:             image,
		HasGit:            hasGit,
		TaskDefinitionARN: newTaskDef.TaskDefinitionARN,
		CreatedAt:         newTaskDef.CreatedAt,
		CreatedBy:         userEmail,
	}
	if err := e.taskDefRepo.CreateTaskDefinition(ctx, registryEntry); err != nil {
		// Log but don't fail - task definition is already registered in ECS
		reqLogger.Warn("failed to store task definition in registry", "error", err, "taskDefARN", newTaskDef.TaskDefinitionARN)
	}

	return newTaskDef.TaskDefinitionARN, nil
}
