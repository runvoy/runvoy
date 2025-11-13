// Package aws provides AWS-specific implementations for runvoy.
// It handles ECS task execution and AWS service integration.
package aws

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/logger"
	awsConstants "runvoy/internal/providers/aws/constants"

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
	TaskRoleARN     string
	TaskExecRoleARN string
	Region          string
	SDKConfig       *awsStd.Config
}

// Runner implements app.Runner for AWS ECS Fargate.
type Runner struct {
	ecsClient *ecs.Client
	cfg       *Config
	logger    *slog.Logger
}

// NewRunner creates a new AWS ECS runner with the provided configuration.
func NewRunner(ecsClient *ecs.Client, cfg *Config, log *slog.Logger) *Runner {
	return &Runner{ecsClient: ecsClient, cfg: cfg, logger: log}
}

// FetchLogsByExecutionID returns CloudWatch log events for the given execution ID.
func (e *Runner) FetchLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error) {
	return FetchLogsByExecutionID(ctx, e.cfg, executionID)
}

type sidecarScriptData struct {
	ProjectName   string
	DefaultGitRef string
	HasGitRepo    bool
}

// buildSidecarContainerCommand constructs the shell command for the sidecar container.
// It handles .env file creation from user environment variables and git repository cloning.
func buildSidecarContainerCommand(hasGitRepo bool) []string {
	script := renderScript("sidecar.sh.tmpl", sidecarScriptData{
		ProjectName:   constants.ProjectName,
		DefaultGitRef: constants.DefaultGitRef,
		HasGitRepo:    hasGitRepo,
	})

	return []string{"/bin/sh", "-c", script}
}

func buildSidecarEnvironment(userEnv map[string]string) []ecsTypes.KeyValuePair {
	env := make([]ecsTypes.KeyValuePair, 0, len(userEnv)*2+1)
	seen := make(map[string]struct{}, len(userEnv)*2+1)

	add := func(name, value string) {
		if _, exists := seen[name]; exists {
			return
		}
		env = append(env, ecsTypes.KeyValuePair{
			Name:  awsStd.String(name),
			Value: awsStd.String(value),
		})
		seen[name] = struct{}{}
	}

	add("RUNVOY_SHARED_VOLUME_PATH", awsConstants.SharedVolumePath)

	for key, value := range userEnv {
		add("RUNVOY_USER_"+key, value)
	}

	for key, value := range userEnv {
		add(key, value)
	}

	return env
}

type gitRepoInfo struct {
	RepoURL  *string
	RepoRef  *string
	RepoPath *string
}

type mainScriptRepoData struct {
	URL     string
	Ref     string
	Path    string
	WorkDir string
}

type mainScriptData struct {
	ProjectName string
	RequestID   string
	Image       string
	Command     string
	Repo        *mainScriptRepoData
}

// buildMainContainerCommand constructs the shell command for the main runner container.
// It adds logging statements and optionally changes to the git repo working directory.
func buildMainContainerCommand(req *api.ExecutionRequest, requestID, image string, repo *gitRepoInfo) []string {
	var repoData *mainScriptRepoData
	if repo != nil {
		workDir := awsConstants.SharedVolumePath + "/repo"
		if trimmed := strings.TrimPrefix(awsStd.ToString(repo.RepoPath), "/"); trimmed != "" && trimmed != "." {
			workDir = workDir + "/" + trimmed
		}

		repoData = &mainScriptRepoData{
			URL:     awsStd.ToString(repo.RepoURL),
			Ref:     awsStd.ToString(repo.RepoRef),
			Path:    awsStd.ToString(repo.RepoPath),
			WorkDir: workDir,
		}
	}

	script := renderScript("main.sh.tmpl", mainScriptData{
		ProjectName: constants.ProjectName,
		RequestID:   requestID,
		Image:       image,
		Command:     req.Command,
		Repo:        repoData,
	})

	return []string{"/bin/sh", "-c", script}
}

// StartTask triggers an ECS Fargate task and returns identifiers.
func (e *Runner) StartTask( //nolint: funlen
	ctx context.Context, userEmail string, req *api.ExecutionRequest) (string, *time.Time, error) {
	if e.ecsClient == nil {
		return "", nil, appErrors.ErrInternalError("ECS cli endpoint not configured", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)
	imageToUse := req.Image

	if imageToUse == "" {
		defaultImage, err := GetDefaultImage(ctx, e.ecsClient, reqLogger)
		if err != nil {
			return "", nil, appErrors.ErrInternalError("failed to query default image", err)
		}
		if defaultImage == "" {
			return "", nil, appErrors.ErrBadRequest("no image specified and no default image configured", nil)
		}
		imageToUse = defaultImage
		reqLogger.Debug("using default image", "image", imageToUse)
	}

	taskDefARN, err := GetTaskDefinitionForImage(ctx, e.ecsClient, imageToUse, reqLogger)
	if err != nil {
		return "", nil, appErrors.ErrBadRequest("image not registered", err)
	}

	reqLogger.Debug("using task definition for image", "context", map[string]string{
		"image": imageToUse,
		"arn":   taskDefARN,
	})

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

	sidecarEnv := buildSidecarEnvironment(req.Env)

	if hasGitRepo {
		gitRef := req.GitRef
		if gitRef == "" {
			gitRef = constants.DefaultGitRef
		}
		sidecarEnv = append(sidecarEnv,
			ecsTypes.KeyValuePair{Name: awsStd.String("GIT_REPO"), Value: awsStd.String(req.GitRepo)},
			ecsTypes.KeyValuePair{Name: awsStd.String("GIT_REF"), Value: awsStd.String(gitRef)},
		)
		reqLogger.Debug("configured sidecar for git cloning",
			"git_repo", req.GitRepo,
			"git_ref", gitRef)
	} else {
		sidecarEnv = append(sidecarEnv,
			ecsTypes.KeyValuePair{Name: awsStd.String("GIT_REPO"), Value: awsStd.String("")},
		)
		reqLogger.Debug("sidecar configured without git (will exit 0)")
	}

	var repo *gitRepoInfo
	if hasGitRepo {
		repo = &gitRepoInfo{
			RepoURL:  awsStd.String(req.GitRepo),
			RepoRef:  awsStd.String(req.GitRef),
			RepoPath: awsStd.String(req.GitPath),
		}
	}
	containerOverrides := []ecsTypes.ContainerOverride{
		{
			Name:        awsStd.String(awsConstants.SidecarContainerName),
			Command:     buildSidecarContainerCommand(hasGitRepo),
			Environment: sidecarEnv,
		},
		{
			Name:        awsStd.String(awsConstants.RunnerContainerName),
			Command:     buildMainContainerCommand(req, requestID, imageToUse, repo),
			Environment: envVars,
		},
	}

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
		"task_definition", taskDefARN,
		"image", imageToUse,
		"container_count", len(containerOverrides),
		"user_email", userEmail,
		"has_git_repo", hasGitRepo,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	var runTaskOutput *ecs.RunTaskOutput
	runTaskOutput, err = e.ecsClient.RunTask(ctx, runTaskInput)
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

	reqLogger.Info("task started", "context", map[string]string{
		"task_arn":     taskARN,
		"execution_id": executionID,
		"created_at":   createdAt.Format(time.RFC3339),
	})

	tagLogArgs := []any{
		"operation", "ECS.TagResource",
		"task_arn", taskARN,
		"execution_id", executionID,
		"created_at", createdAt,
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
			"task_arn", taskARN,
			"execution_id", executionID)
	}

	return executionID, createdAt, nil
}

// RegisterImage registers a Docker image and creates the corresponding task definition.
// isDefault: if true, explicitly set as default.
// If nil or false, becomes default only if no default exists (first image behavior).
func (e *Runner) RegisterImage(ctx context.Context, image string, isDefault *bool) error {
	if e.ecsClient == nil {
		return fmt.Errorf("ECS client not configured")
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	region := e.cfg.Region
	if region == "" {
		return fmt.Errorf("AWS region not configured")
	}

	err := RegisterTaskDefinitionForImage(ctx, e.ecsClient, e.cfg, image, isDefault, region, reqLogger)
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

	result, err := collectImageInfos(ctx, e.ecsClient, taskDefArns, reqLogger)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// describeTaskDef describes a task definition and returns it.
func describeTaskDef(
	ctx context.Context, ecsClient *ecs.Client, taskDefARN string, reqLogger *slog.Logger,
) (*ecsTypes.TaskDefinition, error) {
	descOutput, err := ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: awsStd.String(taskDefARN),
	})
	if err != nil {
		reqLogger.Error("failed to describe task definition", "context", map[string]string{
			"operation": "ECS.DescribeTaskDefinition",
			"arn":       taskDefARN,
			"error":     err.Error(),
		})
		return nil, appErrors.ErrInternalError("failed to describe task definition", err)
	}
	if descOutput.TaskDefinition == nil {
		return nil, appErrors.ErrInternalError("task definition not found", nil)
	}
	return descOutput.TaskDefinition, nil
}

// extractImageFromTaskDef extracts the runner container image from a task definition.
func extractImageFromTaskDef(taskDef *ecsTypes.TaskDefinition, reqLogger *slog.Logger) string {
	familyName := awsStd.ToString(taskDef.Family)
	for i := range taskDef.ContainerDefinitions {
		container := &taskDef.ContainerDefinitions[i]
		if container.Name != nil && *container.Name == awsConstants.RunnerContainerName && container.Image != nil {
			return *container.Image
		}
	}
	reqLogger.Debug("no runner container found in task definition", "container", map[string]string{
		"family":          familyName,
		"container_count": fmt.Sprintf("%d", len(taskDef.ContainerDefinitions)),
	})
	return ""
}

// checkIsDefaultTaskDef checks if a task definition is marked as default.
func checkIsDefaultTaskDef(ctx context.Context, ecsClient *ecs.Client, taskDefARN string) bool {
	tagsOutput, err := ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
		ResourceArn: awsStd.String(taskDefARN),
	})
	if err != nil || tagsOutput == nil {
		return false
	}
	for _, tag := range tagsOutput.Tags {
		if tag.Key != nil && tag.Value != nil &&
			*tag.Key == constants.TaskDefinitionIsDefaultTagKey &&
			*tag.Value == constants.TaskDefinitionIsDefaultTagValue {
			return true
		}
	}
	return false
}

// collectImageInfos iterates described ECS task definitions and extracts unique runner image infos.
func collectImageInfos(
	ctx context.Context, ecsClient *ecs.Client, taskDefArns []string, reqLogger *slog.Logger,
) ([]api.ImageInfo, error) { //nolint:cyclop
	result := make([]api.ImageInfo, 0)
	seenImages := make(map[string]bool)

	for _, taskDefARN := range taskDefArns {
		taskDef, err := describeTaskDef(ctx, ecsClient, taskDefARN, reqLogger)
		if err != nil {
			return nil, err
		}

		image := extractImageFromTaskDef(taskDef, reqLogger)
		if image == "" || seenImages[image] {
			continue
		}

		seenImages[image] = true
		isDefault := checkIsDefaultTaskDef(ctx, ecsClient, taskDefARN)

		result = append(result, api.ImageInfo{
			Image:              image,
			TaskDefinitionARN:  awsStd.ToString(taskDef.TaskDefinitionArn),
			TaskDefinitionName: awsStd.ToString(taskDef.Family),
			IsDefault:          awsStd.Bool(isDefault),
		})
	}

	for _, imageInfo := range result {
		reqLogger.Debug("found runner container image", "context", map[string]string{
			"family":         imageInfo.TaskDefinitionName,
			"container_name": awsConstants.RunnerContainerName,
			"image":          imageInfo.Image,
			"is_default":     strconv.FormatBool(awsStd.ToBool(imageInfo.IsDefault)),
		})
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

// findTaskARNByExecutionID finds the task ARN for a given execution ID by checking both running and stopped tasks.
func (e *Runner) findTaskARNByExecutionID(
	ctx context.Context, executionID string, reqLogger *slog.Logger,
) (string, error) {
	listLogArgs := []any{
		"operation", "ECS.ListTasks",
		"cluster", e.cfg.ECSCluster,
		"desired_status", "RUNNING",
		"execution_id", executionID,
	}
	listLogArgs = append(listLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(listLogArgs))

	listOutput, err := e.ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       awsStd.String(e.cfg.ECSCluster),
		DesiredStatus: ecsTypes.DesiredStatusRunning,
	})
	if err != nil {
		reqLogger.Debug("failed to list tasks", "error", err, "execution_id", executionID)
		return "", appErrors.ErrInternalError("failed to list tasks", err)
	}

	taskARN := extractTaskARNFromList(listOutput.TaskArns, executionID)
	if taskARN != "" {
		return taskARN, nil
	}

	// If not found in running tasks, check stopped tasks
	listStoppedLogArgs := []any{
		"operation", "ECS.ListTasks",
		"cluster", e.cfg.ECSCluster,
		"desired_status", "STOPPED",
		"execution_id", executionID,
	}
	listStoppedLogArgs = append(listStoppedLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(listStoppedLogArgs))

	listStoppedOutput, err := e.ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       awsStd.String(e.cfg.ECSCluster),
		DesiredStatus: ecsTypes.DesiredStatusStopped,
	})
	if err == nil {
		taskARN = extractTaskARNFromList(listStoppedOutput.TaskArns, executionID)
	}

	if taskARN == "" {
		reqLogger.Error("task not found", "execution_id", executionID)
		return "", appErrors.ErrNotFound("task not found", nil)
	}

	return taskARN, nil
}

// extractTaskARNFromList finds the task ARN that matches the execution ID from a list of task ARNs.
func extractTaskARNFromList(taskArns []string, executionID string) string {
	for _, arn := range taskArns {
		parts := strings.Split(arn, "/")
		if len(parts) > 0 && parts[len(parts)-1] == executionID {
			return arn
		}
	}
	return ""
}

// validateTaskStatusForKill validates that a task is in a state that can be terminated.
func validateTaskStatusForKill(currentStatus string) error {
	terminatedStatuses := []string{
		string(awsConstants.EcsStatusStopped),
		string(awsConstants.EcsStatusStopping),
		string(awsConstants.EcsStatusDeactivating),
	}
	if slices.Contains(terminatedStatuses, currentStatus) {
		return appErrors.ErrBadRequest(
			"task is already terminated or terminating",
			fmt.Errorf("task status: %s", currentStatus))
	}

	taskRunnableStatuses := []string{
		string(awsConstants.EcsStatusRunning),
		string(awsConstants.EcsStatusActivating),
	}
	if !slices.Contains(taskRunnableStatuses, string(awsConstants.EcsStatus(currentStatus))) {
		return appErrors.ErrBadRequest(
			"task cannot be terminated in current state",
			fmt.Errorf(
				"task status: %s, expected: %s",
				currentStatus,
				strings.Join(taskRunnableStatuses, ", ")))
	}

	return nil
}

// KillTask terminates an ECS task identified by executionID.
// It checks the task status before termination and only stops tasks that are RUNNING or ACTIVATING.
// Returns an error if the task is already terminated or not found.
//
//nolint:funlen // Complex AWS API orchestration
func (e *Runner) KillTask(ctx context.Context, executionID string) error {
	if e.ecsClient == nil {
		return appErrors.ErrInternalError("ECS client not configured", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	taskARN, err := e.findTaskARNByExecutionID(ctx, executionID, reqLogger)
	if err != nil {
		return err
	}

	describeLogArgs := []any{
		"operation", "ECS.DescribeTasks",
		"cluster", e.cfg.ECSCluster,
		"task_arn", taskARN,
		"execution_id", executionID,
	}
	describeLogArgs = append(describeLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(describeLogArgs))

	describeOutput, err := e.ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: awsStd.String(e.cfg.ECSCluster),
		Tasks:   []string{taskARN},
	})
	if err != nil {
		reqLogger.Error("failed to describe task", "context", map[string]string{
			"error":        err.Error(),
			"execution_id": executionID,
			"task_arn":     taskARN,
		})
		return appErrors.ErrInternalError("failed to describe task", err)
	}

	if len(describeOutput.Tasks) == 0 {
		reqLogger.Error("task not found", "context", map[string]string{
			"execution_id": executionID,
			"task_arn":     taskARN,
		})
		return appErrors.ErrNotFound("task not found", nil)
	}

	task := describeOutput.Tasks[0]
	currentStatus := awsStd.ToString(task.LastStatus)
	reqLogger.Debug("task status check", "execution_id", executionID, "status", currentStatus)

	if validateErr := validateTaskStatusForKill(currentStatus); validateErr != nil {
		return validateErr
	}

	stopLogArgs := []any{
		"operation", "ECS.StopTask",
		"cluster", e.cfg.ECSCluster,
		"task_arn", taskARN,
		"execution_id", executionID,
		"current_status", currentStatus,
	}
	stopLogArgs = append(stopLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(stopLogArgs))

	stopOutput, err := e.ecsClient.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: awsStd.String(e.cfg.ECSCluster),
		Task:    awsStd.String(taskARN),
		Reason:  awsStd.String("Terminated by user via kill endpoint"),
	})
	if err != nil {
		reqLogger.Error("failed to stop task", "error", err, "execution_id", executionID, "task_arn", taskARN)
		return appErrors.ErrInternalError("failed to stop task", err)
	}

	reqLogger.Info(
		"task termination initiated",
		"execution_id", executionID,
		"task_arn", awsStd.ToString(stopOutput.Task.TaskArn),
		"previous_status", currentStatus)

	return nil
}
