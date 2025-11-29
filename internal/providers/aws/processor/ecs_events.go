package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/constants"
	"github.com/runvoy/runvoy/internal/logger"
	awsConstants "github.com/runvoy/runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-lambda-go/events"
)

// handleECSTaskEvent processes ECS Task State Change events
func (p *Processor) handleECSTaskEvent(
	ctx context.Context,
	event *events.CloudWatchEvent,
	reqLogger *slog.Logger,
) error {
	var taskEvent ECSTaskStateChangeEvent
	if err := json.Unmarshal(event.Detail, &taskEvent); err != nil {
		reqLogger.Error("failed to parse ECS task event", "error", err)
		return fmt.Errorf("failed to parse ECS task event: %w", err)
	}

	executionID := extractExecutionIDFromTaskArn(taskEvent.TaskArn)

	reqLogger.Info("processing ECS task state change",
		"execution", map[string]string{
			"execution_id":   executionID,
			"started_at":     taskEvent.StartedAt,
			"last_status":    taskEvent.LastStatus,
			"stop_code":      taskEvent.StopCode,
			"stopped_at":     taskEvent.StoppedAt,
			"stopped_reason": taskEvent.StoppedReason,
			"task_arn":       taskEvent.TaskArn,
		})

	execution, err := p.executionRepo.GetExecution(ctx, executionID)
	if err != nil {
		reqLogger.Error("failed to get execution", "error", err)
		return fmt.Errorf("failed to get execution: %w", err)
	}

	if execution == nil {
		reqLogger.Error("execution not found for task (orphaned task?)",
			"cluster_arn", taskEvent.ClusterArn,
		)
		// Orphaned tasks are tasks that exist in ECS but have no corresponding execution record.
		// This can happen if tasks were started manually or if the execution record was deleted.
		// We don't fail the Lambda for orphaned tasks to avoid breaking the event processing pipeline.
		return nil
	}

	status := awsConstants.EcsStatus(taskEvent.LastStatus)

	switch status { //nolint:exhaustive // we are only interested in a subset of the possible ECS task statuses
	case awsConstants.EcsStatusRunning:
		return p.updateExecutionToRunning(ctx, executionID, execution, reqLogger)
	case awsConstants.EcsStatusStopped:
		return p.finalizeExecutionFromTaskEvent(ctx, executionID, execution, &taskEvent, reqLogger)
	default:
		reqLogger.Debug("ignoring unhandled ECS task status update",
			"context", map[string]string{
				"execution_id": executionID,
				"last_status":  taskEvent.LastStatus,
			},
		)
		return nil
	}
}

func (p *Processor) updateExecutionToRunning(
	ctx context.Context,
	executionID string,
	execution *api.Execution,
	reqLogger *slog.Logger,
) error {
	currentStatus := constants.ExecutionStatus(execution.Status)
	targetStatus := constants.ExecutionRunning

	if currentStatus == targetStatus {
		reqLogger.Debug("execution already marked as "+string(targetStatus),
			"context", map[string]string{
				"execution_id": executionID,
			},
		)
		return nil
	}

	if !constants.CanTransition(currentStatus, targetStatus) {
		reqLogger.Debug("skipping invalid status transition to "+string(targetStatus),
			"context", map[string]string{
				"execution_id":   executionID,
				"current_status": execution.Status,
				"target_status":  string(targetStatus),
			},
		)
		return nil
	}

	execution.Status = string(targetStatus)
	execution.CompletedAt = nil

	// Extract request ID from context and set ModifiedByRequestID
	requestID := logger.ExtractRequestIDFromContext(ctx)
	if requestID != "" {
		execution.ModifiedByRequestID = requestID
	}

	if err := p.executionRepo.UpdateExecution(ctx, execution); err != nil {
		reqLogger.Error("failed to update execution status to "+string(targetStatus),
			"error", err,
			"execution_id", executionID,
		)
		return fmt.Errorf("failed to update execution to running: %w", err)
	}

	reqLogger.Debug("execution marked as "+string(targetStatus),
		"context", map[string]string{
			"execution_id": executionID,
		},
	)

	return nil
}

func (p *Processor) finalizeExecutionFromTaskEvent(
	ctx context.Context,
	executionID string,
	execution *api.Execution,
	taskEvent *ECSTaskStateChangeEvent,
	reqLogger *slog.Logger,
) error {
	status, exitCode := determineStatusAndExitCode(taskEvent)
	_, stoppedAt, durationSeconds, err := parseTaskTimes(taskEvent, execution.StartedAt, reqLogger)
	if err != nil {
		return err
	}

	currentStatus := constants.ExecutionStatus(execution.Status)
	targetStatus := constants.ExecutionStatus(status)

	if !constants.CanTransition(currentStatus, targetStatus) {
		reqLogger.Warn("skipping invalid status transition",
			"context", map[string]string{
				"execution_id":   executionID,
				"current_status": execution.Status,
				"target_status":  status,
			},
		)
		return nil
	}

	execution.Status = status
	execution.ExitCode = exitCode
	execution.CompletedAt = &stoppedAt
	execution.DurationSeconds = durationSeconds

	// Extract request ID from context and set ModifiedByRequestID
	requestID := logger.ExtractRequestIDFromContext(ctx)
	if requestID != "" {
		execution.ModifiedByRequestID = requestID
	}

	if err = p.executionRepo.UpdateExecution(ctx, execution); err != nil {
		reqLogger.Error("failed to update execution", "error", err)
		return fmt.Errorf("failed to update execution: %w", err)
	}

	reqLogger.Info("execution updated successfully", "execution", execution)

	if err = p.logEventRepo.DeleteLogEvents(ctx, executionID); err != nil {
		reqLogger.Error("failed to delete buffered log events", "error", err, "execution_id", executionID)
		return fmt.Errorf("failed to delete buffered logs: %w", err)
	}

	// Notify WebSocket clients about the execution completion
	if err = p.webSocketManager.NotifyExecutionCompletion(ctx, &executionID); err != nil {
		reqLogger.Error("failed to notify websocket clients of disconnect", "error", err)
		return err
	}

	return nil
}

// extractExecutionIDFromTaskArn extracts the execution ID from a task ARN
// Task ARN format: arn:aws:ecs:region:account:task/cluster-name/EXECUTION_ID
func extractExecutionIDFromTaskArn(taskArn string) string {
	parts := strings.Split(taskArn, "/")
	return parts[len(parts)-1]
}

// determineStatusAndExitCode determines the final status and exit code from the task event
func determineStatusAndExitCode(event *ECSTaskStateChangeEvent) (status string, exitCode int) {
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
		if container.Name == awsConstants.RunnerContainerName {
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
