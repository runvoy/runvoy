// Package events provides event processing functionality for AWS CloudWatch events,
// particularly for handling ECS task state changes and execution completion notifications.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"runvoy/internal/constants"
	"runvoy/internal/database"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// parseTaskTimes parses and validates the task timestamps, calculating duration.
func parseTaskTimes(
	taskEvent *ECSTaskStateChangeEvent, executionStartedAt time.Time, reqLogger *slog.Logger,
) (startedAt, stoppedAt time.Time, durationSeconds int, err error) {
	if taskEvent.StartedAt != "" {
		startedAt, err = ParseTime(taskEvent.StartedAt)
		if err != nil {
			reqLogger.Error("failed to parse startedAt timestamp", "error", err, "started_at", taskEvent.StartedAt)
			return time.Time{}, time.Time{}, 0, fmt.Errorf("failed to parse startedAt: %w", err)
		}
	} else {
		reqLogger.Warn("startedAt missing from task event, using execution's StartedAt",
			"execution_started_at", executionStartedAt.Format(time.RFC3339),
		)
		startedAt = executionStartedAt
	}

	stoppedAt, err = ParseTime(taskEvent.StoppedAt)
	if err != nil {
		reqLogger.Error("failed to parse stoppedAt timestamp", "error", err, "stopped_at", taskEvent.StoppedAt)
		return time.Time{}, time.Time{}, 0, fmt.Errorf("failed to parse stoppedAt: %w", err)
	}

	durationSeconds = int(stoppedAt.Sub(startedAt).Seconds())
	if durationSeconds < 0 {
		reqLogger.Warn("calculated negative duration, setting to 0",
			"started_at", startedAt.Format(time.RFC3339),
			"stopped_at", stoppedAt.Format(time.RFC3339),
		)
		durationSeconds = 0
	}

	return startedAt, stoppedAt, durationSeconds, nil
}

// handleECSTaskCompletion processes ECS Task State Change events
func (p *Processor) handleECSTaskCompletion(ctx context.Context, event *events.CloudWatchEvent) error {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	var taskEvent ECSTaskStateChangeEvent
	if err := json.Unmarshal(event.Detail, &taskEvent); err != nil {
		reqLogger.Error("failed to parse ECS task event", "error", err)
		return fmt.Errorf("failed to parse ECS task event: %w", err)
	}

	executionID := extractExecutionIDFromTaskArn(taskEvent.TaskArn)

	reqLogger.Info("pattern matched, processing ECS task completion",
		"execution", map[string]string{
			"execution_id":   executionID,
			"started_at":     taskEvent.StartedAt,
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
		// Don't fail for orphaned tasks - they might have been started manually?
		// TODO: figure out what to do with orphaned tasks or if we should fail the Lambda
		return nil
	}

	status, exitCode := determineStatusAndExitCode(&taskEvent)
	_, stoppedAt, durationSeconds, err := parseTaskTimes(&taskEvent, execution.StartedAt, reqLogger)
	if err != nil {
		return err
	}

	execution.Status = status
	execution.ExitCode = exitCode
	execution.CompletedAt = &stoppedAt
	execution.DurationSeconds = durationSeconds

	if err = p.executionRepo.UpdateExecution(ctx, execution); err != nil {
		reqLogger.Error("failed to update execution", "error", err)
		return fmt.Errorf("failed to update execution: %w", err)
	}

	reqLogger.Info("execution updated successfully", "execution", execution)

	// Clean up WebSocket connections for terminal executions
	p.notifyDisconnectAndCleanup(ctx, execution.Status, executionID, reqLogger)

	return nil
}

// notifyDisconnectAndCleanup invokes the connection manager Lambda to notify connected clients
// of the execution completion, then removes WebSocket connections.
// This is best-effort and won't fail the handler if cleanup fails.
func (p *Processor) notifyDisconnectAndCleanup(
	ctx context.Context,
	status string,
	executionID string,
	reqLogger *slog.Logger,
) {
	if !isTerminalStatus(status) {
		return
	}

	// First, invoke websocket_manager Lambda to notify connected clients
	invokeErr := p.invokeWebSocketManager(ctx, executionID, reqLogger)
	if invokeErr != nil {
		reqLogger.Warn("failed to invoke websocket manager for disconnect notification",
			"error", invokeErr,
			"execution_id", executionID,
		)
	} else {
		reqLogger.Debug("invoked websocket manager for disconnect notification",
			"context", map[string]string{
				"execution_id": executionID,
			},
		)
	}

	// Then delete all connection records
	deletedCount, err := p.connectionRepo.DeleteConnectionsByExecutionID(ctx, executionID)
	if err != nil {
		// Log warning but don't fail - connection cleanup is best-effort
		reqLogger.Warn("failed to delete WebSocket connections", "context",
			map[string]string{
				"error":        err.Error(),
				"execution_id": executionID,
			},
		)
		return
	}

	if deletedCount > 0 {
		reqLogger.Debug("deleted WebSocket connections for terminal execution", "context",
			map[string]string{
				"execution_id":  executionID,
				"deleted_count": fmt.Sprintf("%d", deletedCount),
			},
		)
	}
}

// isTerminalStatus checks if an execution status is a terminal state
func isTerminalStatus(status string) bool {
	return status == string(constants.ExecutionSucceeded) ||
		status == string(constants.ExecutionFailed) ||
		status == string(constants.ExecutionStopped)
}

// invokeWebSocketManager invokes the websocket_manager Lambda to send disconnect notifications.
func (p *Processor) invokeWebSocketManager(
	ctx context.Context,
	executionID string,
	_ *slog.Logger,
) error {
	// Skip if lambdaClient is not initialized (e.g., in tests)
	if p.lambdaClient == nil {
		return nil
	}

	payload := map[string]string{
		"type":         "disconnect",
		"execution_id": executionID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal disconnect payload: %w", err)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)
	reqLogger.Debug("invoking websocket manager", "context",
		map[string]any{
			"function_name": p.websocketManager,
			"payload":       payload,
		},
	)

	invocation := &lambda.InvokeInput{
		FunctionName:   aws.String(p.websocketManager),
		InvocationType: types.InvocationTypeEvent, // Async invocation
		Payload:        payloadBytes,
	}

	_, err = p.lambdaClient.Invoke(ctx, invocation)
	if err != nil {
		return fmt.Errorf("failed to invoke websocket manager: %w", err)
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
	connectionRepo database.ConnectionRepository,
	log *slog.Logger) func(context.Context, events.CloudWatchEvent) error {
	return func(ctx context.Context, event events.CloudWatchEvent) error {
		p := &Processor{
			executionRepo:  executionRepo,
			connectionRepo: connectionRepo,
			logger:         log,
		}
		return p.handleECSTaskCompletion(ctx, &event)
	}
}
