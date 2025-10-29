package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"runvoy/internal/database"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/events"
)

// handleECSTaskCompletion processes ECS Task State Change events
func (p *Processor) handleECSTaskCompletion(ctx context.Context, event events.CloudWatchEvent) error {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	// Parse the event detail
	var taskEvent ECSTaskStateChangeEvent
	if err := json.Unmarshal(event.Detail, &taskEvent); err != nil {
		reqLogger.Error("failed to parse ECS task event", "error", err)
		return fmt.Errorf("failed to parse ECS task event: %w", err)
	}

	// Extract execution ID from task ARN (last segment)
	executionID := extractExecutionIDFromTaskArn(taskEvent.TaskArn)
	reqLogger = reqLogger.With("executionID", executionID, "taskArn", taskEvent.TaskArn)

	reqLogger.Debug("processing ECS task completion",
		"lastStatus", taskEvent.LastStatus,
		"stopCode", taskEvent.StopCode,
		"stoppedReason", taskEvent.StoppedReason,
	)

	// Get the existing execution record
	execution, err := p.executionRepo.GetExecution(ctx, executionID)
	if err != nil {
		reqLogger.Error("failed to get execution", "error", err)
		return fmt.Errorf("failed to get execution: %w", err)
	}

	if execution == nil {
		reqLogger.Warn("execution not found for task (orphaned task?)",
			"clusterArn", taskEvent.ClusterArn,
		)
		// Don't fail for orphaned tasks - they might have been started manually
		return nil
	}

	// Determine final status and exit code
	status, exitCode := determineStatusAndExitCode(taskEvent)

	// Parse timestamps
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

	// Calculate duration
	durationSeconds := int(stoppedAt.Sub(startedAt).Seconds())

	// Calculate cost
	cost, err := CalculateFargateCost(taskEvent.CPU, taskEvent.Memory, durationSeconds)
	if err != nil {
		reqLogger.Warn("failed to calculate cost", "error", err)
		cost = 0.0 // Continue with zero cost rather than failing
	}

	// Update execution record
	execution.Status = status
	execution.ExitCode = exitCode
	execution.CompletedAt = &stoppedAt
	execution.DurationSeconds = durationSeconds
	execution.CostUSD = cost

	if err := p.executionRepo.UpdateExecution(ctx, execution); err != nil {
		reqLogger.Error("failed to update execution", "error", err)
		return fmt.Errorf("failed to update execution: %w", err)
	}

	reqLogger.Info("execution updated successfully",
		"status", status,
		"exitCode", exitCode,
		"durationSeconds", durationSeconds,
		"costUSD", fmt.Sprintf("%.6f", cost),
	)

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
	status = "FAILED"
	exitCode = 1

	// Check stop code first
	switch event.StopCode {
	case "UserInitiated":
		status = "STOPPED"
		exitCode = 130 // Standard exit code for SIGINT/manual termination
		return
	case "TaskFailedToStart":
		status = "FAILED"
		exitCode = 1
		return
	}

	// Get exit code from the first container (executor container)
	if len(event.Containers) > 0 && event.Containers[0].ExitCode != nil {
		exitCode = *event.Containers[0].ExitCode
		if exitCode == 0 {
			status = "SUCCEEDED"
		} else {
			status = "FAILED"
		}
		return
	}

	// If we reach here, we don't have a clear exit code
	// Use stop code to determine status
	if event.StopCode == "EssentialContainerExited" {
		// Container exited but we don't have the exit code
		// Assume failure since we should have the exit code for success
		status = "FAILED"
		exitCode = 1
	}

	return
}

// ECSCompletionHandler is a factory function that returns a handler for ECS completion events
func ECSCompletionHandler(executionRepo database.ExecutionRepository, log *slog.Logger) func(context.Context, events.CloudWatchEvent) error {
	return func(ctx context.Context, event events.CloudWatchEvent) error {
		p := &Processor{
			executionRepo: executionRepo,
			logger:        log,
		}
		return p.handleECSTaskCompletion(ctx, event)
	}
}
