package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func handleExec(ctx context.Context, req Request) (Response, error) {
	// Validate required fields
	if req.Command == "" {
		return Response{}, fmt.Errorf("command required")
	}
	// Repo is only required when git cloning is enabled
	if !req.SkipGit && req.Repo == "" {
		return Response{}, fmt.Errorf("repo required (unless --skip-git is used)")
	}

	// Generate execution ID if not provided
	execID := req.ExecutionID
	if execID == "" {
		execID = generateExecutionID()
	}

	// Set defaults
	branch := req.Branch
	if branch == "" {
		branch = "main"
	}

	// Note: Custom images per execution are not fully supported at runtime via overrides
	_ = req.Image

	timeoutSeconds := req.TimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = 1800 // 30 minutes default (currently unused in ECS call)
	}
	_ = timeoutSeconds

	// Construct the shell command
	var shellCommand string
	if req.SkipGit {
		// Skip git cloning and run command directly
		shellCommand = buildDirectCommand(req.Command)
	} else {
		// Standard flow: setup git credentials, clone repo, execute command
		shellCommand = buildShellCommand(req.Repo, branch, req.Command, githubToken, gitlabToken, sshPrivateKey)
	}

	// Build environment variables for the container
	// Include user-provided environment variables
	envVars := []ecsTypes.KeyValuePair{
		{Name: aws.String("EXECUTION_ID"), Value: aws.String(execID)},
	}

	// Add user-provided environment variables
	for key, value := range req.Env {
		envVars = append(envVars, ecsTypes.KeyValuePair{
			Name:  aws.String(key),
			Value: aws.String(value),
		})
	}

	// Run Fargate task
	runTaskInput := &ecs.RunTaskInput{
		Cluster:        &ecsCluster,
		TaskDefinition: &taskDef,
		LaunchType:     ecsTypes.LaunchTypeFargate,
		NetworkConfiguration: &ecsTypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecsTypes.AwsVpcConfiguration{
				Subnets:        []string{subnet1, subnet2},
				SecurityGroups: []string{securityGroup},
				AssignPublicIp: ecsTypes.AssignPublicIpEnabled,
			},
		},
		Tags: []ecsTypes.Tag{
			{Key: aws.String("ExecutionID"), Value: aws.String(execID)},
		},
	}

	// Add Repo tag if provided (not required when skip-git is enabled)
	if req.Repo != "" {
		runTaskInput.Tags = append(runTaskInput.Tags, ecsTypes.Tag{
			Key:   aws.String("Repo"),
			Value: aws.String(req.Repo),
		})
	}

	// Override container with our command and environment
	containerOverride := ecsTypes.ContainerOverride{
		Name:        aws.String("executor"),
		Environment: envVars,
		// Override the command to run our shell script
		Command: []string{"/bin/sh", "-c", shellCommand},
	}

	runTaskInput.Overrides = &ecsTypes.TaskOverride{
		ContainerOverrides: []ecsTypes.ContainerOverride{containerOverride},
	}

	runTaskResp, err := ecsClient.RunTask(ctx, runTaskInput)
	if err != nil {
		return Response{}, fmt.Errorf("failed to run task: %v", err)
	}

	if len(runTaskResp.Tasks) == 0 {
		return Response{}, fmt.Errorf("no task created")
	}

	task := runTaskResp.Tasks[0]
	taskArn := *task.TaskArn

	// Generate log stream name (ECS uses task ID)
	taskID := taskArn[len(taskArn)-36:] // Last 36 chars (UUID)
	logStream := fmt.Sprintf("task/%s", taskID)

	return Response{
		ExecutionID: execID,
		TaskArn:     taskArn,
		Status:      "starting",
		LogStream:   logStream,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func handleStatus(ctx context.Context, req Request) (Response, error) {
	if req.TaskArn == "" {
		return Response{}, fmt.Errorf("task_arn required")
	}

	describeResp, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &ecsCluster,
		Tasks:   []string{req.TaskArn},
	})
	if err != nil {
		return Response{}, fmt.Errorf("failed to describe task: %v", err)
	}

	if len(describeResp.Tasks) == 0 {
		return Response{}, fmt.Errorf("task not found")
	}

	task := describeResp.Tasks[0]
	createdAt := ""
	if task.CreatedAt != nil {
		createdAt = task.CreatedAt.Format(time.RFC3339)
	}

	return Response{
		Status:        aws.ToString(task.LastStatus),
		DesiredStatus: aws.ToString(task.DesiredStatus),
		CreatedAt:     createdAt,
	}, nil
}

func handleLogs(ctx context.Context, req Request) (Response, error) {
	if req.ExecutionID == "" {
		return Response{}, fmt.Errorf("execution_id required")
	}

	// Try to find log streams for this execution
	// ECS creates log streams with format: task/<task-id>
	// We stored execution ID as a tag, but for simplicity, we'll search by prefix

	streamPrefix := "task/"

	filterResp, err := logsClient.FilterLogEvents(ctx, &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:        &logGroup,
		LogStreamNamePrefix: &streamPrefix,
		Limit:               aws.Int32(1000),
	})
	if err != nil {
		return Response{}, fmt.Errorf("failed to get logs: %v", err)
	}

	var logs string
	for _, event := range filterResp.Events {
		if event.Message != nil {
			logs += *event.Message + "\n"
		}
	}

	if logs == "" {
		logs = "No logs available yet. The task may still be starting."
	}

	return Response{Logs: logs}, nil
}
