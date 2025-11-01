package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"runvoy/internal/constants"
	"runvoy/internal/database"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/events"
)

// handleECSTaskCompletion processes ECS Task State Change events
func (p *Processor) handleECSTaskCompletion(ctx context.Context, event events.CloudWatchEvent) error {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	var taskEvent ECSTaskStateChangeEvent
	if err := json.Unmarshal(event.Detail, &taskEvent); err != nil {
		reqLogger.Error("failed to parse ECS task event", "error", err)
		return fmt.Errorf("failed to parse ECS task event: %w", err)
	}

	executionID := extractExecutionIDFromTaskArn(taskEvent.TaskArn)

	reqLogger.Info("processing ECS task completion", "execution", map[string]string{
		"executionID":   executionID,
		"startedAt":     taskEvent.StartedAt,
		"stopCode":      taskEvent.StopCode,
		"stoppedAt":     taskEvent.StoppedAt,
		"stoppedReason": taskEvent.StoppedReason,
		"taskArn":       taskEvent.TaskArn,
	})

	execution, err := p.executionRepo.GetExecution(ctx, executionID)
	if err != nil {
		reqLogger.Error("failed to get execution", "error", err)
		return fmt.Errorf("failed to get execution: %w", err)
	}

	if execution == nil {
		reqLogger.Error("execution not found for task (orphaned task?)",
			"clusterArn", taskEvent.ClusterArn,
		)

		// Don't fail for orphaned tasks - they might have been started manually?
		// TODO: figure out what to do with orphaned tasks or if we should fail the Lambda
		return nil
	}

	status, exitCode := determineStatusAndExitCode(taskEvent)
	startedAt, err := ParseTime(taskEvent.StartedAt)
	if err != nil {
		reqLogger.Error("failed to parse startedAt timestamp", "error", err, "startedAt", taskEvent.StartedAt)
		return fmt.Errorf("failed to parse startedAt: %w", err)
	}

	stoppedAt, err := ParseTime(taskEvent.StoppedAt)
	if err != nil {
		reqLogger.Error("failed to parse stoppedAt timestamp", "error", err, "stoppedAt", taskEvent.StoppedAt)
		return fmt.Errorf("failed to parse stoppedAt: %w", err)
	}

	durationSeconds := int(stoppedAt.Sub(startedAt).Seconds())
	cost, err := CalculateFargateCost(taskEvent.CPU, taskEvent.Memory, durationSeconds)
	if err != nil {
		reqLogger.Warn("failed to calculate cost", "error", err)
		cost = 0.0 // Continue with zero cost rather than failing? TODO: figure out what to do with failed cost calculations
	}

	execution.Status = status
	execution.ExitCode = exitCode
	execution.CompletedAt = &stoppedAt
	execution.DurationSeconds = durationSeconds
	execution.CostUSD = cost

	if err := p.executionRepo.UpdateExecution(ctx, execution); err != nil {
		reqLogger.Error("failed to update execution", "error", err)
		return fmt.Errorf("failed to update execution: %w", err)
	}

	reqLogger.Info("execution updated successfully", "execution", execution)

	return nil
}

// extractExecutionIDFromTaskArn extracts the execution ID from a task ARN
// Task ARN format: arn:aws:ecs:region:account:task/cluster-name/EXECUTION_ID
func extractExecutionIDFromTaskArn(taskArn string) string {
	parts := strings.Split(taskArn, "/")
	return parts[len(parts)-1]
}

// determineStatusAndExitCode determines the final status and exit code from the task event
func determineStatusAndExitCode(event ECSTaskStateChangeEvent) (status string, exitCode int) {
	// Default values
	status = string(constants.ExecutionFailed)
	exitCode = 1

	// Check stop code first
	switch event.StopCode {
	case "UserInitiated":
		status = string(constants.ExecutionStopped)
		exitCode = 130 // Standard exit code for SIGINT/manual termination
		return status, exitCode
	case "TaskFailedToStart":
		status = string(constants.ExecutionFailed)
		exitCode = 1
		return status, exitCode
	}

	// Find the main runner container by name and get its exit code
	for _, container := range event.Containers {
		if container.Name == constants.RunnerContainerName {
			if container.ExitCode != nil {
				exitCode = *container.ExitCode
				if exitCode == 0 {
					status = string(constants.ExecutionSucceeded)
				} else {
					status = string(constants.ExecutionFailed)
				}
				return status, exitCode
			}
			// Runner container found but no exit code available
			break
		}
	}

	// If we reach here, we don't have a clear exit code
	// Use stop code to determine status
	if event.StopCode == "EssentialContainerExited" {
		// Container exited but we don't have the exit code
		// Assume failure since we should have the exit code for success
		status = string(constants.ExecutionFailed)
		exitCode = 1
	}

	return status, exitCode
}

// ECSCompletionHandler is a factory function that returns a handler for ECS completion events
func ECSCompletionHandler(
	executionRepo database.ExecutionRepository,
	log *slog.Logger) func(context.Context, events.CloudWatchEvent) error {
	return func(ctx context.Context, event events.CloudWatchEvent) error {
		p := &Processor{
			executionRepo: executionRepo,
			logger:        log,
		}
		return p.handleECSTaskCompletion(ctx, event)
	}
}
