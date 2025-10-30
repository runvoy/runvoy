// Package aws provides AWS-specific implementations for runvoy.
// It handles ECS task execution and AWS service integration.
package aws

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/lambdacontext"
	awsstd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
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

// StartTask triggers an ECS Fargate task and returns identifiers.
func (e *Runner) StartTask(ctx context.Context, userEmail string, req api.ExecutionRequest) (string, string, error) {
	if e.ecsClient == nil {
		return "", "", apperrors.ErrInternalError("ECS cli endpoint not configured", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	// Note: Image override is not supported via task overrides; use task definition image.
	if req.Image != "" && req.Image != e.cfg.DefaultImage {
		reqLogger.Debug("custom image requested but not supported via overrides, using task definition image",
			"requested", req.Image, "using", e.cfg.DefaultImage)
	}

	envVars := []ecstypes.KeyValuePair{
		{Name: awsstd.String("RUNVOY_COMMAND"), Value: awsstd.String(req.Command)},
	}
	for key, value := range req.Env {
		envVars = append(envVars, ecstypes.KeyValuePair{Name: awsstd.String(key), Value: awsstd.String(value)})
	}

	requestID := ""
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		requestID = lc.AwsRequestID
	}

	containerCommand := []string{"/bin/sh", "-c"}
	actualCommands := []string{
		fmt.Sprintf("printf '### %s runner execution started by requestID %s\\n'", constants.ProjectName, requestID),
		fmt.Sprintf("printf '### %s command => %s\\n'", constants.ProjectName, req.Command),
		req.Command,
	}
	containerCommand = append(containerCommand, strings.Join(actualCommands, " && "))

	runTaskInput := &ecs.RunTaskInput{
		Cluster:        awsstd.String(e.cfg.ECSCluster),
		TaskDefinition: awsstd.String(e.cfg.TaskDefinition),
		LaunchType:     ecstypes.LaunchTypeFargate,
		Overrides: &ecstypes.TaskOverride{ContainerOverrides: []ecstypes.ContainerOverride{{
			Name:        awsstd.String(constants.RunnerContainerName),
			Command:     containerCommand,
			Environment: envVars,
		}}},
		NetworkConfiguration: &ecstypes.NetworkConfiguration{AwsvpcConfiguration: &ecstypes.AwsVpcConfiguration{
			Subnets:        []string{e.cfg.Subnet1, e.cfg.Subnet2},
			SecurityGroups: []string{e.cfg.SecurityGroup},
			AssignPublicIp: ecstypes.AssignPublicIpEnabled,
		}},
		Tags: []ecstypes.Tag{{Key: awsstd.String("UserEmail"), Value: awsstd.String(userEmail)}},
	}

	runTaskOutput, err := e.ecsClient.RunTask(ctx, runTaskInput)
	if err != nil {
		return "", "", apperrors.ErrInternalError("failed to start ECS task", err)
	}
	if len(runTaskOutput.Tasks) == 0 {
		return "", "", apperrors.ErrInternalError("no tasks were started", nil)
	}

	task := runTaskOutput.Tasks[0]
	taskARN := awsstd.ToString(task.TaskArn)
	executionIDParts := strings.Split(taskARN, "/")
	executionID := executionIDParts[len(executionIDParts)-1]

	reqLogger.Debug("task started", "taskARN", taskARN, "executionID", executionID)

	// Add ExecutionID tag to the task for easier tracking (best-effort)
	_, tagErr := e.ecsClient.TagResource(ctx, &ecs.TagResourceInput{
		ResourceArn: awsstd.String(taskARN),
		Tags:        []ecstypes.Tag{{Key: awsstd.String("ExecutionID"), Value: awsstd.String(executionID)}},
	})
	if tagErr != nil {
		reqLogger.Warn("failed to add ExecutionID tag to task", "error", tagErr, "taskARN", taskARN, "executionID", executionID)
	}

	return executionID, taskARN, nil
}

// KillTask terminates an ECS task identified by executionID.
// It checks the task status before termination and only stops tasks that are RUNNING or ACTIVATING.
// Returns an error if the task is already terminated or not found.
func (e *Runner) KillTask(ctx context.Context, executionID string) error {
	if e.ecsClient == nil {
		return apperrors.ErrInternalError("ECS client not configured", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	// First, describe the task to check its current status
	// We can use ListTasks to find the task ARN, or construct it from the execution ID
	// For ECS, we can use DescribeTasks with just the task ID (execution ID) if we know the cluster
	// However, AWS ECS requires the full task ARN. Let's use ListTasks to find it first.
	listOutput, err := e.ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       awsstd.String(e.cfg.ECSCluster),
		DesiredStatus: ecstypes.DesiredStatusRunning,
	})
	if err != nil {
		reqLogger.Debug("failed to list tasks", "error", err, "executionID", executionID)
		return apperrors.ErrInternalError("failed to list tasks", err)
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
		listStoppedOutput, err := e.ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:       awsstd.String(e.cfg.ECSCluster),
			DesiredStatus: ecstypes.DesiredStatusStopped,
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
		return apperrors.ErrNotFound("task not found", nil)
	}

	// Describe the task to check its current status
	describeOutput, err := e.ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: awsstd.String(e.cfg.ECSCluster),
		Tasks:   []string{taskARN},
	})
	if err != nil {
		reqLogger.Debug("failed to describe task", "error", err, "executionID", executionID, "taskARN", taskARN)
		return apperrors.ErrInternalError("failed to describe task", err)
	}

	if len(describeOutput.Tasks) == 0 {
		reqLogger.Warn("task not found", "executionID", executionID, "taskARN", taskARN)
		return apperrors.ErrNotFound("task not found", nil)
	}

	task := describeOutput.Tasks[0]
	currentStatus := awsstd.ToString(task.LastStatus)

	reqLogger.Debug("task status check", "executionID", executionID, "status", currentStatus)

	// Check if task is already terminated
	terminatedStatuses := []string{"STOPPED", "STOPPING", "DEACTIVATING"}
	for _, status := range terminatedStatuses {
		if currentStatus == status {
			return apperrors.ErrBadRequest("task is already terminated or terminating", fmt.Errorf("task status: %s", currentStatus))
		}
	}

	// Only stop tasks that are RUNNING or ACTIVATING
	runnableStatuses := []string{"RUNNING", "ACTIVATING"}
	isRunnable := false
	for _, status := range runnableStatuses {
		if currentStatus == status {
			isRunnable = true
			break
		}
	}

	if !isRunnable {
		return apperrors.ErrBadRequest("task cannot be terminated in current state", fmt.Errorf("task status: %s", currentStatus))
	}

	// Stop the task
	stopOutput, err := e.ecsClient.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: awsstd.String(e.cfg.ECSCluster),
		Task:    awsstd.String(taskARN),
		Reason:  awsstd.String("Terminated by user via kill endpoint"),
	})
	if err != nil {
		reqLogger.Error("failed to stop task", "error", err, "executionID", executionID, "taskARN", taskARN)
		return apperrors.ErrInternalError("failed to stop task", err)
	}

	reqLogger.Info("task termination initiated", "executionID", executionID, "taskARN", awsstd.ToString(stopOutput.Task.TaskArn), "previousStatus", currentStatus)

	return nil
}
