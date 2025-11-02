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

	// Note: Image override is not supported yet
	if req.Image != "" && req.Image != e.cfg.DefaultImage {
		reqLogger.Debug("custom image requested but not supported via overrides, aborting",
			"requested", req.Image)
		return "", "", nil, appErrors.ErrBadRequest("custom image requested but not supported via overrides yet", nil)
	}

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
		envVars = append(envVars, ecsTypes.KeyValuePair{
			Name:  awsStd.String("RUNVOY_USER_" + key),
			Value: awsStd.String(value),
		})
	}

	sidecarEnv := []ecsTypes.KeyValuePair{
		{Name: awsStd.String("SHARED_VOLUME_PATH"), Value: awsStd.String(constants.SharedVolumePath)},
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
		TaskDefinition: awsStd.String(e.cfg.TaskDefinition),
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
		"taskDefinition", e.cfg.TaskDefinition,
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
