// Package orchestrator provides AWS-specific implementations for runvoy orchestrator.
// It handles ECS task execution and AWS service integration.
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/logger"
	awsClient "runvoy/internal/providers/aws/client"
	awsConstants "runvoy/internal/providers/aws/constants"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// Config holds AWS-specific execution configuration.
type Config struct {
	ECSCluster             string
	TaskDefinition         string
	Subnet1                string
	Subnet2                string
	SecurityGroup          string
	LogGroup               string
	DefaultTaskRoleARN     string
	DefaultTaskExecRoleARN string
	Region                 string
	AccountID              string
	SDKConfig              *awsStd.Config
}

// ImageTaskDefRepository defines the interface for image-taskdef mapping operations.
type ImageTaskDefRepository interface {
	PutImageTaskDef(
		ctx context.Context,
		imageID string,
		image string,
		imageRegistry string,
		imageName string,
		imageTag string,
		taskRoleName, taskExecutionRoleName *string,
		cpu, memory int,
		runtimePlatform string,
		taskDefFamily string,
		isDefault bool,
	) error
	GetImageTaskDef(
		ctx context.Context,
		image string,
		taskRoleName, taskExecutionRoleName *string,
		cpu, memory *int,
		runtimePlatform *string,
	) (*api.ImageInfo, error)
	GetImageTaskDefByID(ctx context.Context, imageID string) (*api.ImageInfo, error)
	GetAnyImageTaskDef(ctx context.Context, image string) (*api.ImageInfo, error)
	ListImages(ctx context.Context) ([]api.ImageInfo, error)
	GetDefaultImage(ctx context.Context) (*api.ImageInfo, error)
	UnmarkAllDefaults(ctx context.Context) error
	DeleteImage(ctx context.Context, image string) error
	SetImageAsOnlyDefault(ctx context.Context, image string, taskRoleName, taskExecutionRoleName *string) error
}

// Runner implements app.Runner for AWS ECS Fargate.
type Runner struct {
	ecsClient awsClient.ECSClient
	cwlClient CloudWatchLogsClient
	iamClient awsClient.IAMClient
	imageRepo ImageTaskDefRepository
	cfg       *Config
	logger    *slog.Logger
}

// NewRunner creates a new AWS ECS runner with the provided configuration.
func NewRunner(
	ecsClient awsClient.ECSClient,
	cwlClient CloudWatchLogsClient,
	iamClient awsClient.IAMClient,
	imageRepo ImageTaskDefRepository,
	cfg *Config,
	log *slog.Logger,
) *Runner {
	return &Runner{
		ecsClient: ecsClient,
		cwlClient: cwlClient,
		iamClient: iamClient,
		imageRepo: imageRepo,
		cfg:       cfg,
		logger:    log,
	}
}

// FetchLogsByExecutionID returns CloudWatch log events for the given execution ID.
func (e *Runner) FetchLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error) {
	if executionID == "" {
		return nil, appErrors.ErrBadRequest("executionID is required", nil)
	}

	stream := awsConstants.BuildLogStreamName(executionID)
	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	if verifyErr := verifyLogStreamExists(
		ctx, e.cwlClient, e.cfg.LogGroup, stream, executionID, reqLogger,
	); verifyErr != nil {
		return nil, verifyErr
	}

	reqLogger.Debug("calling external service", "context", map[string]string{
		"operation":    "CloudWatchLogs.GetLogEvents",
		"log_group":    e.cfg.LogGroup,
		"log_stream":   stream,
		"execution_id": executionID,
		"paginated":    "true",
	})

	events, err := getAllLogEvents(ctx, e.cwlClient, e.cfg.LogGroup, stream)
	if err != nil {
		return nil, err
	}
	reqLogger.Debug("log events fetched successfully", "context", map[string]string{
		"events_count": fmt.Sprintf("%d", len(events)),
	})

	return events, nil
}

type sidecarScriptData struct {
	ProjectName    string
	DefaultGitRef  string
	HasGitRepo     bool
	SecretVarNames []string
	AllVarNames    []string
}

// getSecretVariableNames returns a list of variable names that should be treated as secrets.
// These variables will be processed without exposing their values in logs.
func getSecretVariableNames(userEnv map[string]string) []string {
	secretNames := []string{}
	secretPatterns := []string{
		"GITHUB_SECRET",
		"GITHUB_TOKEN",
		"SECRET",
		"TOKEN",
		"PASSWORD",
		"API_KEY",
		"API_SECRET",
		"PRIVATE_KEY",
		"ACCESS_KEY",
		"SECRET_KEY",
	}

	for key := range userEnv {
		upperKey := strings.ToUpper(key)
		for _, pattern := range secretPatterns {
			if strings.Contains(upperKey, pattern) {
				secretNames = append(secretNames, key)
				break
			}
		}
	}

	return secretNames
}

// sanitizeURLForLogging removes authentication tokens from URLs for safe logging.
// Replaces patterns like "https://token@host" with "https://***@host".
func sanitizeURLForLogging(url string) string {
	// Match pattern: https://anything@host/path
	re := regexp.MustCompile(`(https?://)([^/@]+@)([^/]+)`)
	return re.ReplaceAllString(url, "${1}***@${3}")
}

// injectGitHubTokenIfNeeded modifies a GitHub repository URL to include authentication
// if GITHUB_TOKEN is available in the user environment variables.
// Returns the original URL if it's not a GitHub URL or if no token is available.
func injectGitHubTokenIfNeeded(gitRepo string, userEnv map[string]string) string {
	if !strings.HasPrefix(gitRepo, "https://github.com/") {
		return gitRepo
	}

	token, hasToken := userEnv["GITHUB_TOKEN"]
	if !hasToken || token == "" {
		return gitRepo
	}

	return strings.Replace(gitRepo, "https://", "https://"+token+"@", 1)
}

// buildSidecarContainerCommand constructs the shell command for the sidecar container.
// It handles .env file creation from user environment variables and git repository cloning.
func buildSidecarContainerCommand(hasGitRepo bool, userEnv map[string]string) []string {
	secretVarNames := getSecretVariableNames(userEnv)
	allVarNames := make([]string, 0, len(userEnv))
	for key := range userEnv {
		allVarNames = append(allVarNames, key)
	}
	script := renderScript("sidecar.sh.tmpl", sidecarScriptData{
		ProjectName:    constants.ProjectName,
		DefaultGitRef:  constants.DefaultGitRef,
		HasGitRepo:     hasGitRepo,
		SecretVarNames: secretVarNames,
		AllVarNames:    allVarNames,
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

		repoURL := awsStd.ToString(repo.RepoURL)
		// Sanitize URL for logging to avoid exposing tokens
		sanitizedURL := sanitizeURLForLogging(repoURL)
		repoData = &mainScriptRepoData{
			URL:     sanitizedURL,
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
func (e *Runner) StartTask(
	ctx context.Context, userEmail string, req *api.ExecutionRequest) (string, *time.Time, error) {
	if e.ecsClient == nil {
		return "", nil, appErrors.ErrInternalError("ECS cli endpoint not configured", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	imageToUse, taskDefARN, err := e.resolveImage(ctx, req, reqLogger)
	if err != nil {
		return "", nil, err
	}

	gitConfig := e.configureGitRepo(ctx, req, reqLogger)

	containerOverrides, mainEnvVars := e.buildContainerOverrides(ctx, req, gitConfig, reqLogger)

	runTaskInput := e.buildRunTaskInput(userEmail, taskDefARN, containerOverrides, gitConfig.HasRepo)

	executionID, createdAt, taskARN, err := e.executeTask(ctx, runTaskInput, imageToUse, reqLogger)
	if err != nil {
		return "", nil, err
	}

	e.logTaskStarted(reqLogger, userEmail, taskARN, executionID, createdAt, req, imageToUse, mainEnvVars)

	return executionID, createdAt, nil
}

// resolveImage determines which image to use and gets its task definition ARN.
func (e *Runner) resolveImage(
	ctx context.Context, req *api.ExecutionRequest, reqLogger *slog.Logger,
) (imageToUse, taskDefARN string, err error) {
	imageToUse = req.Image

	if imageToUse == "" {
		defaultImage, getErr := e.GetDefaultImageFromDB(ctx)
		if getErr != nil {
			return "", "", appErrors.ErrInternalError("failed to query default image", getErr)
		}
		if defaultImage == "" {
			return "", "", appErrors.ErrBadRequest("no image specified and no default image configured", nil)
		}
		imageToUse = defaultImage
		reqLogger.Debug("using default image", "image", imageToUse)
	}

	taskDefARN, err = e.GetTaskDefinitionARNForImage(ctx, imageToUse)
	if err != nil {
		return "", "", appErrors.ErrBadRequest("image not registered", err)
	}

	reqLogger.Debug("task definition resolved", "context", map[string]string{
		"image": imageToUse,
		"arn":   taskDefARN,
	})

	return
}

// gitRepoConfig holds the configuration for git repository setup
type gitRepoConfig struct {
	HasRepo              bool
	AuthenticatedRepoURL string
	Ref                  string
	Info                 *gitRepoInfo
}

// configureGitRepo sets up git repository configuration if provided in the request.
func (e *Runner) configureGitRepo(
	_ context.Context, req *api.ExecutionRequest, reqLogger *slog.Logger,
) *gitRepoConfig {
	config := &gitRepoConfig{HasRepo: req.GitRepo != ""}

	if !config.HasRepo {
		return config
	}

	gitRef := req.GitRef
	if gitRef == "" {
		gitRef = constants.DefaultGitRef
	}
	config.Ref = gitRef

	config.AuthenticatedRepoURL = injectGitHubTokenIfNeeded(req.GitRepo, req.Env)

	config.Info = &gitRepoInfo{
		RepoURL:  awsStd.String(config.AuthenticatedRepoURL),
		RepoRef:  awsStd.String(gitRef),
		RepoPath: awsStd.String(req.GitPath),
	}

	reqLogger.Debug("git repository configured",
		"git_repo", req.GitRepo,
		"git_ref", gitRef)
	if config.AuthenticatedRepoURL != req.GitRepo {
		reqLogger.Debug("using GitHub token for repository authentication")
	}

	return config
}

// buildContainerOverrides constructs the container overrides for sidecar and main runner containers.
func (e *Runner) buildContainerOverrides(
	ctx context.Context, req *api.ExecutionRequest, gitConfig *gitRepoConfig, _ *slog.Logger,
) ([]ecsTypes.ContainerOverride, []ecsTypes.KeyValuePair) {
	requestID := logger.GetRequestID(ctx)

	mainEnvVars := []ecsTypes.KeyValuePair{
		{Name: awsStd.String("RUNVOY_COMMAND"), Value: awsStd.String(req.Command)},
	}
	for key, value := range req.Env {
		mainEnvVars = append(mainEnvVars, ecsTypes.KeyValuePair{
			Name:  awsStd.String(key),
			Value: awsStd.String(value),
		})
	}

	sidecarEnv := buildSidecarEnvironment(req.Env)
	if gitConfig.HasRepo {
		sidecarEnv = append(sidecarEnv,
			ecsTypes.KeyValuePair{Name: awsStd.String("GIT_REPO"), Value: awsStd.String(gitConfig.AuthenticatedRepoURL)},
			ecsTypes.KeyValuePair{Name: awsStd.String("GIT_REF"), Value: awsStd.String(gitConfig.Ref)},
		)
	} else {
		sidecarEnv = append(sidecarEnv,
			ecsTypes.KeyValuePair{Name: awsStd.String("GIT_REPO"), Value: awsStd.String("")},
		)
	}

	return []ecsTypes.ContainerOverride{
		{
			Name:        awsStd.String(awsConstants.SidecarContainerName),
			Command:     buildSidecarContainerCommand(gitConfig.HasRepo, req.Env),
			Environment: sidecarEnv,
		},
		{
			Name:        awsStd.String(awsConstants.RunnerContainerName),
			Command:     buildMainContainerCommand(req, requestID, req.Image, gitConfig.Info),
			Environment: mainEnvVars,
		},
	}, mainEnvVars
}

// buildRunTaskInput constructs the ECS RunTask input with all necessary configuration.
func (e *Runner) buildRunTaskInput(
	userEmail, taskDefARN string,
	containerOverrides []ecsTypes.ContainerOverride,
	hasGitRepo bool,
) *ecs.RunTaskInput {
	tags := []ecsTypes.Tag{
		{Key: awsStd.String("UserEmail"), Value: awsStd.String(userEmail)},
	}
	if hasGitRepo {
		tags = append(tags, ecsTypes.Tag{
			Key:   awsStd.String("HasGitRepo"),
			Value: awsStd.String("true"),
		})
	}

	return &ecs.RunTaskInput{
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
}

// executeTask calls the ECS RunTask API and extracts execution identifiers from the response.
func (e *Runner) executeTask(
	ctx context.Context,
	runTaskInput *ecs.RunTaskInput,
	imageToUse string,
	reqLogger *slog.Logger,
) (executionID string, createdAt *time.Time, taskARN string, err error) {
	logArgs := []any{
		"operation", "ECS.RunTask",
		"cluster", e.cfg.ECSCluster,
		"task_definition", runTaskInput.TaskDefinition,
		"image", imageToUse,
		"container_count", len(runTaskInput.Overrides.ContainerOverrides),
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	runTaskOutput, err := e.ecsClient.RunTask(ctx, runTaskInput)
	if err != nil {
		return "", nil, "", appErrors.ErrInternalError("failed to start ECS task", err)
	}
	if len(runTaskOutput.Tasks) == 0 {
		return "", nil, "", appErrors.ErrInternalError("no tasks were started", nil)
	}

	task := runTaskOutput.Tasks[0]
	taskARN = awsStd.ToString(task.TaskArn)
	executionIDParts := strings.Split(taskARN, "/")
	executionID = executionIDParts[len(executionIDParts)-1]
	createdAt = task.CreatedAt

	return executionID, createdAt, taskARN, nil
}

// logTaskStarted logs the successful task start with request details.
func (e *Runner) logTaskStarted(
	reqLogger *slog.Logger,
	userEmail, taskARN, executionID string,
	createdAt *time.Time,
	req *api.ExecutionRequest,
	imageToUse string,
	_ []ecsTypes.KeyValuePair,
) {
	requestFields := make(map[string]string)
	if req.Command != "" {
		requestFields["command"] = req.Command
	}
	if imageToUse != "" {
		requestFields["image"] = imageToUse
	}
	if req.GitRepo != "" {
		requestFields["git_repo"] = req.GitRepo
	}
	if req.GitRef != "" {
		requestFields["git_ref"] = req.GitRef
	}
	if req.GitPath != "" {
		requestFields["git_path"] = req.GitPath
	}
	if len(req.Env) > 0 {
		requestFields["env_keys"] = strings.Join(slices.Collect(maps.Keys(req.Env)), ", ")
	}
	if len(req.Secrets) > 0 {
		requestFields["secrets"] = strings.Join(req.Secrets, ", ")
	}

	logContext := map[string]any{
		"user_email":   userEmail,
		"task_arn":     taskARN,
		"execution_id": executionID,
		"created_at":   createdAt.Format(time.RFC3339),
	}
	if len(requestFields) > 0 {
		logContext["request"] = requestFields
	}

	reqLogger.Info("task started", "context", logContext)
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
		// Error is already wrapped by findTaskARNByExecutionID, pass through
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
