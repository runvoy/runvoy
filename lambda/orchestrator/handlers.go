package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// CreateExecutionRequest is the request body for POST /executions
type CreateExecutionRequest struct {
	Command        string            `json:"command"`
	Lock           string            `json:"lock,omitempty"`
	Image          string            `json:"image,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	TimeoutSeconds int               `json:"timeout,omitempty"`
}

// CreateExecutionResponse is the response for POST /executions
type CreateExecutionResponse struct {
	ExecutionID string `json:"execution_id"`
	TaskArn     string `json:"task_arn"`
	LogURL      string `json:"log_url,omitempty"`
	Status      string `json:"status"`
}

// handleCreateExecution handles POST /executions
func handleCreateExecution(ctx context.Context, cfg *Config, user *User, request events.APIGatewayProxyRequest) (*CreateExecutionResponse, error) {
	// Parse request body
	var req CreateExecutionRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return nil, fmt.Errorf("invalid request: %v", err)
	}

	// Validate required fields
	if req.Command == "" {
		return nil, fmt.Errorf("command required")
	}

	// Set defaults
	if req.TimeoutSeconds == 0 {
		req.TimeoutSeconds = 1800 // 30 minutes default
	}

	// Generate execution ID
	execID := generateExecutionID()

	// Try to acquire lock if requested
	if req.Lock != "" {
		acquired, holder, err := tryAcquireLock(ctx, cfg, req.Lock, execID, user.UserEmail, req.TimeoutSeconds)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}
		if !acquired {
			return nil, fmt.Errorf("lock '%s' is held by %s (execution %s) since %s",
				req.Lock, holder.UserEmail, holder.ExecutionID, holder.AcquiredAt)
		}
	}

	// Determine which task definition to use
	taskDefArn := cfg.TaskDef
	if req.Image != "" {
		// Register a new task definition with the custom image
		var err error
		taskDefArn, err = getOrCreateTaskDefinition(ctx, cfg, req.Image)
		if err != nil {
			// Release lock if we acquired it
			if req.Lock != "" {
				_ = releaseLock(ctx, cfg, req.Lock)
			}
			return nil, fmt.Errorf("failed to get/create task definition: %v", err)
		}
	}

	// Build environment variables for the container
	envVars := []ecsTypes.KeyValuePair{
		{Name: aws.String("EXECUTION_ID"), Value: aws.String(execID)},
		{Name: aws.String("USER_EMAIL"), Value: aws.String(user.UserEmail)},
	}
	if req.Lock != "" {
		envVars = append(envVars, ecsTypes.KeyValuePair{
			Name:  aws.String("LOCK_NAME"),
			Value: aws.String(req.Lock),
		})
	}

	// Add user-provided environment variables
	for key, value := range req.Env {
		envVars = append(envVars, ecsTypes.KeyValuePair{
			Name:  aws.String(key),
			Value: aws.String(value),
		})
	}

	// Construct the shell command
	shellCommand := buildDirectCommand(req.Command)

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
			{Key: aws.String("UserEmail"), Value: aws.String(user.UserEmail)},
		},
		Overrides: &ecsTypes.TaskOverride{
			ContainerOverrides: []ecsTypes.ContainerOverride{
				{
					Name:        aws.String("executor"),
					Environment: envVars,
					Command:     []string{"/bin/sh", "-c", shellCommand},
				},
			},
		},
	}

	runTaskResp, err := cfg.ECSClient.RunTask(ctx, runTaskInput)
	if err != nil {
		// Release lock if we acquired it
		if req.Lock != "" {
			_ = releaseLock(ctx, cfg, req.Lock)
		}
		return nil, fmt.Errorf("failed to run task: %v", err)
	}

	if len(runTaskResp.Tasks) == 0 {
		// Release lock if we acquired it
		if req.Lock != "" {
			_ = releaseLock(ctx, cfg, req.Lock)
		}
		return nil, fmt.Errorf("no task created")
	}

	task := runTaskResp.Tasks[0]
	taskArn := *task.TaskArn

	// Extract task ID from ARN
	taskID := extractTaskID(taskArn)
	logStream := fmt.Sprintf("task/executor/%s", taskID)

	// Record execution in DynamoDB
	now := time.Now().UTC()
	execution := &Execution{
		ExecutionID:   execID,
		UserEmail:     user.UserEmail,
		Command:       req.Command,
		LockName:      req.Lock,
		TaskArn:       taskArn,
		StartedAt:     now.Format(time.RFC3339),
		Status:        "starting",
		LogStreamName: logStream,
	}

	if err := recordExecution(ctx, cfg, execution); err != nil {
		fmt.Printf("[WARN] Failed to record execution: %v\n", err)
		// Don't fail the request, task is already running
	}

	// Generate log viewer URL (TODO: implement JWT signing)
	logURL := ""
	if cfg.WebUIURL != "" {
		logURL = fmt.Sprintf("%s/%s", cfg.WebUIURL, execID)
	}

	return &CreateExecutionResponse{
		ExecutionID: execID,
		TaskArn:     taskArn,
		LogURL:      logURL,
		Status:      "starting",
	}, nil
}

// handleGetExecution handles GET /executions/{id}
func handleGetExecution(ctx context.Context, cfg *Config, user *User, executionID string) (*Execution, error) {
	exec, err := getExecution(ctx, cfg, executionID)
	if err != nil {
		return nil, err
	}

	// Verify user has access to this execution
	if exec.UserEmail != user.UserEmail {
		return nil, fmt.Errorf("unauthorized: execution belongs to another user")
	}

	// Update status from ECS if not completed
	if exec.Status != "completed" && exec.Status != "failed" {
		if exec.TaskArn != "" {
			describeResp, err := cfg.ECSClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
				Cluster: &cfg.ECSCluster,
				Tasks:   []string{exec.TaskArn},
			})
			if err == nil && len(describeResp.Tasks) > 0 {
				task := describeResp.Tasks[0]
				exec.Status = aws.ToString(task.LastStatus)
			}
		}
	}

	return exec, nil
}

// handleListExecutions handles GET /executions
func handleListExecutions(ctx context.Context, cfg *Config, user *User, request events.APIGatewayProxyRequest) ([]Execution, error) {
	// Parse query parameters
	limitStr := request.QueryStringParameters["limit"]
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	executions, err := listExecutions(ctx, cfg, user.UserEmail, limit)
	if err != nil {
		return nil, err
	}

	return executions, nil
}

// handleGetExecutionLogs handles GET /executions/{id}/logs
func handleGetExecutionLogs(ctx context.Context, cfg *Config, user *User, executionID string) (map[string]interface{}, error) {
	exec, err := getExecution(ctx, cfg, executionID)
	if err != nil {
		return nil, err
	}

	// Verify user has access to this execution
	if exec.UserEmail != user.UserEmail {
		return nil, fmt.Errorf("unauthorized: execution belongs to another user")
	}

	// Get logs from CloudWatch
	logs := ""
	if exec.LogStreamName != "" {
		// Check if log stream exists
		describeStreamsResp, err := cfg.LogsClient.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName:        &cfg.LogGroup,
			LogStreamNamePrefix: &exec.LogStreamName,
			Limit:               aws.Int32(1),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe log streams: %v", err)
		}

		if len(describeStreamsResp.LogStreams) > 0 {
			logStreamName := *describeStreamsResp.LogStreams[0].LogStreamName

			// Get logs from the specific log stream
			filterResp, err := cfg.LogsClient.FilterLogEvents(ctx, &cloudwatchlogs.FilterLogEventsInput{
				LogGroupName:   &cfg.LogGroup,
				LogStreamNames: []string{logStreamName},
				Limit:          aws.Int32(1000),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get logs: %v", err)
			}

			// Format logs with timestamps
			for _, event := range filterResp.Events {
				if event.Message != nil && event.Timestamp != nil {
					timestamp := time.UnixMilli(*event.Timestamp).UTC()
					logs += fmt.Sprintf("%s | %s\n", timestamp.Format("2006-01-02 15:04:05 UTC"), *event.Message)
				}
			}
		}
	}

	if logs == "" {
		logs = "No logs available yet. The task may still be starting."
	}

	return map[string]interface{}{
		"execution_id": executionID,
		"status":       exec.Status,
		"logs":         logs,
		"completed":    exec.Status == "completed" || exec.Status == "failed",
	}, nil
}

// handleListLocks handles GET /locks
func handleListLocks(ctx context.Context, cfg *Config, user *User) ([]Lock, error) {
	locks, err := listLocks(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return locks, nil
}
// getOrCreateTaskDefinition gets or creates a task definition for the specified image
// It uses a sanitized version of the image name to create a human-readable family name
// If a task definition with this family already exists and has the same image, it reuses it
// If a name collision occurs (same sanitized name, different image), it falls back to hash-based naming
func getOrCreateTaskDefinition(ctx context.Context, cfg *Config, image string) (string, error) {
	// Sanitize the image name for use in task definition family name
	// Replace invalid characters with hyphens and convert to lowercase
	sanitized := strings.NewReplacer(
		":", "-", // alpine:latest → alpine-latest
		"/", "-", // hashicorp/terraform → hashicorp-terraform
		".", "-", // ubuntu:22.04 → ubuntu-22-04
		"@", "-", // sha256 digest references
		"_", "-", // underscores to hyphens for consistency
	).Replace(image)

	sanitized = strings.ToLower(sanitized)

	// Truncate if too long (ECS family name limit is 255 chars, leave room for prefix)
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}

	// Trim trailing hyphens that might result from sanitization
	sanitized = strings.TrimRight(sanitized, "-")

	family := fmt.Sprintf("mycli-task-%s", sanitized)

	// Check if this task definition family already exists
	describeResp, err := cfg.ECSClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(family),
	})

	if err == nil && describeResp.TaskDefinition != nil {
		// Task definition exists - verify it has the correct image
		existingImage := aws.ToString(describeResp.TaskDefinition.ContainerDefinitions[0].Image)

		if existingImage == image {
			// Perfect match! Reuse the existing task definition
			fmt.Printf("[INFO] Reusing existing task definition %s for image %s\n", family, image)
			return *describeResp.TaskDefinition.TaskDefinitionArn, nil
		}

		// Name collision detected: same sanitized name but different image
		// Fall back to hash-based naming for uniqueness
		fmt.Printf("[WARN] Task definition %s exists but with different image (%s vs %s), using hash-based name\n",
			family, existingImage, image)

		hash := sha256.Sum256([]byte(image))
		shortHash := hex.EncodeToString(hash[:])[:8]
		family = fmt.Sprintf("mycli-task-%s-%s", sanitized, shortHash)

		// Check if the hash-based family exists (it should, but verify)
		describeHashResp, hashErr := cfg.ECSClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: aws.String(family),
		})
		if hashErr == nil && describeHashResp.TaskDefinition != nil {
			fmt.Printf("[INFO] Reusing existing hash-based task definition %s\n", family)
			return *describeHashResp.TaskDefinition.TaskDefinitionArn, nil
		}
	}

	// Task definition doesn't exist, we need to create it
	fmt.Printf("[INFO] Creating new task definition %s for image %s\n", family, image)

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

	fmt.Printf("[INFO] Successfully registered task definition %s\n", family)
	return *registerResp.TaskDefinition.TaskDefinitionArn, nil
}
