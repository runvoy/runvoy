// Package aws provides AWS-specific implementations for runvoy.
// It handles ECS task execution and AWS service integration.
package aws

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"runvoy/internal/api"
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

	// TODO: find a better way to get the request ID, or better, to ensure it's always available in the context
	requestID := ""
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		requestID = lc.AwsRequestID
	}
	containerCommand := []string{"/bin/sh", "-c", fmt.Sprintf("echo 'Execution for requestID %s starting'; %s", requestID, req.Command)}

	runTaskInput := &ecs.RunTaskInput{
		Cluster:        awsstd.String(e.cfg.ECSCluster),
		TaskDefinition: awsstd.String(e.cfg.TaskDefinition),
		LaunchType:     ecstypes.LaunchTypeFargate,
		Overrides: &ecstypes.TaskOverride{ContainerOverrides: []ecstypes.ContainerOverride{{
            Name:        awsstd.String("runner"),
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
