package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func handleExec(ctx context.Context, cfg *Config, req Request) (Response, error) {
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
		shellCommand = buildShellCommand(cfg, req.Repo, branch, req.Command)
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

	// Determine which task definition to use
	taskDefArn := cfg.TaskDef
	if req.Image != "" {
		// Register a new task definition with the custom image
		var err error
		taskDefArn, err = getOrCreateTaskDefinition(ctx, cfg, req.Image)
		if err != nil {
			return Response{}, fmt.Errorf("failed to get/create task definition: %v", err)
		}
	}

	// Run Fargate task
	runTaskInput := &ecs.RunTaskInput{
		Cluster:        &cfg.ECSCluster,
		TaskDefinition: &taskDefArn,
		LaunchType:     ecsTypes.LaunchTypeFargate,
		NetworkConfiguration: &ecsTypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecsTypes.AwsVpcConfiguration{
				Subnets:        []string{cfg.Subnet1, cfg.Subnet2},
				SecurityGroups: []string{cfg.SecurityGroup},
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

	runTaskResp, err := cfg.ECSClient.RunTask(ctx, runTaskInput)
	if err != nil {
		return Response{}, fmt.Errorf("failed to run task: %v", err)
	}

	if len(runTaskResp.Tasks) == 0 {
		return Response{}, fmt.Errorf("no task created")
	}

	task := runTaskResp.Tasks[0]
	taskArn := *task.TaskArn

	// Generate log stream name (ECS format: task/{container-name}/{task-id})
	// Extract task ID from ARN by finding the last slash
	lastSlash := -1
	for i := len(taskArn) - 1; i >= 0; i-- {
		if taskArn[i] == '/' {
			lastSlash = i
			break
		}
	}
	taskID := taskArn[lastSlash+1:]
	logStream := fmt.Sprintf("task/executor/%s", taskID)

	return Response{
		ExecutionID: execID,
		TaskArn:     taskArn,
		Status:      "starting",
		LogStream:   logStream,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// getOrCreateTaskDefinition gets or creates a task definition for the specified image
// It uses a hash of the image name to create a unique family name
// If a task definition with this family already exists, it reuses it
func getOrCreateTaskDefinition(ctx context.Context, cfg *Config, image string) (string, error) {
	// Create a stable family name based on the image
	// Use a hash to keep the name short and valid (alphanumeric + hyphens only)
	hash := sha256.Sum256([]byte(image))
	imageHash := hex.EncodeToString(hash[:])[:16] // Use first 16 chars of hash
	family := fmt.Sprintf("mycli-task-%s", imageHash)

	// Check if this task definition family already exists
	describeResp, err := cfg.ECSClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(family),
	})
	if err == nil && describeResp.TaskDefinition != nil {
		// Task definition exists, return its ARN
		return *describeResp.TaskDefinition.TaskDefinitionArn, nil
	}

	// Task definition doesn't exist, we need to create it
	// First, describe the base task definition to get its configuration
	baseTaskDef, err := cfg.ECSClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(cfg.TaskDef),
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe base task definition: %v", err)
	}

	baseDef := baseTaskDef.TaskDefinition

	// Create a new container definition with the custom image
	containerDef := baseDef.ContainerDefinitions[0]
	containerDef.Image = aws.String(image)

	// Register the new task definition
	registerResp, err := cfg.ECSClient.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family:                  aws.String(family),
		NetworkMode:             baseDef.NetworkMode,
		RequiresCompatibilities: baseDef.RequiresCompatibilities,
		Cpu:                     baseDef.Cpu,
		Memory:                  baseDef.Memory,
		ExecutionRoleArn:        baseDef.ExecutionRoleArn,
		TaskRoleArn:             baseDef.TaskRoleArn,
		ContainerDefinitions:    []ecsTypes.ContainerDefinition{containerDef},
	})
	if err != nil {
		return "", fmt.Errorf("failed to register task definition: %v", err)
	}

	return *registerResp.TaskDefinition.TaskDefinitionArn, nil
}

func handleStatus(ctx context.Context, cfg *Config, req Request) (Response, error) {
	if req.TaskArn == "" {
		return Response{}, fmt.Errorf("task_arn required")
	}

	describeResp, err := cfg.ECSClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &cfg.ECSCluster,
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

func handleLogs(ctx context.Context, cfg *Config, req Request) (Response, error) {
	if req.TaskArn == "" {
		return Response{}, fmt.Errorf("task_arn required")
	}

	// Extract task ID from ARN by finding the last slash
	// ARN format: arn:aws:ecs:region:account:task/cluster-name/task-id
	lastSlash := -1
	for i := len(req.TaskArn) - 1; i >= 0; i-- {
		if req.TaskArn[i] == '/' {
			lastSlash = i
			break
		}
	}
	if lastSlash == -1 {
		return Response{}, fmt.Errorf("invalid task ARN format (no slash found): %s", req.TaskArn)
	}
	taskID := req.TaskArn[lastSlash+1:]
	if len(taskID) != 32 && len(taskID) != 36 {
		return Response{}, fmt.Errorf("invalid task ID length: %s (expected 32 or 36 chars)", taskID)
	}

	// Log stream format: task/{container-name}/{task-id}
	logStreamPrefix := fmt.Sprintf("task/executor/%s", taskID)

	// Debug logging
	fmt.Printf("[DEBUG] handleLogs: TaskARN=%s\n", req.TaskArn)
	fmt.Printf("[DEBUG] handleLogs: TaskID=%s\n", taskID)
	fmt.Printf("[DEBUG] handleLogs: LogGroup=%s\n", cfg.LogGroup)
	fmt.Printf("[DEBUG] handleLogs: LogStreamPrefix=%s\n", logStreamPrefix)

	// First, check if any log streams exist with this prefix
	// This handles cases where logs haven't been written yet
	describeStreamsResp, err := cfg.LogsClient.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        &cfg.LogGroup,
		LogStreamNamePrefix: &logStreamPrefix,
		Limit:               aws.Int32(10), // Get more streams for debugging
	})
	if err != nil {
		fmt.Printf("[DEBUG] DescribeLogStreams error: %v\n", err)
		return Response{}, fmt.Errorf("failed to describe log streams: %v", err)
	}

	fmt.Printf("[DEBUG] Found %d log streams\n", len(describeStreamsResp.LogStreams))
	for i, stream := range describeStreamsResp.LogStreams {
		fmt.Printf("[DEBUG] Stream[%d]: %s\n", i, aws.ToString(stream.LogStreamName))
	}

	// If no log streams found, task hasn't started logging yet
	if len(describeStreamsResp.LogStreams) == 0 {
		return Response{Logs: fmt.Sprintf("No logs available yet. Searched for log stream prefix: %s\nThe task may still be starting or provisioning.", logStreamPrefix)}, nil
	}

	// Use the actual log stream name (might have additional suffix)
	logStreamName := *describeStreamsResp.LogStreams[0].LogStreamName
	fmt.Printf("[DEBUG] Using log stream: %s\n", logStreamName)

	// Get logs from the specific log stream
	filterResp, err := cfg.LogsClient.FilterLogEvents(ctx, &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:   &cfg.LogGroup,
		LogStreamNames: []string{logStreamName},
		Limit:          aws.Int32(1000),
	})
	if err != nil {
		fmt.Printf("[DEBUG] FilterLogEvents error: %v\n", err)
		return Response{}, fmt.Errorf("failed to get logs: %v", err)
	}

	fmt.Printf("[DEBUG] Found %d log events\n", len(filterResp.Events))

	// Format logs with timestamps
	var logs string
	for _, event := range filterResp.Events {
		if event.Message != nil && event.Timestamp != nil {
			// Convert Unix milliseconds to time.Time
			timestamp := time.UnixMilli(*event.Timestamp).UTC()
			// Format as: 2025-10-26 14:32:10 UTC | message
			logs += fmt.Sprintf("%s | %s\n", timestamp.Format("2006-01-02 15:04:05 UTC"), *event.Message)
		}
	}

	if logs == "" {
		logs = "No logs available yet. The task may still be starting."
	}

	return Response{Logs: logs}, nil
}
