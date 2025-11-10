package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	"runvoy/internal/websocket"

	"github.com/aws/aws-lambda-go/events"
)

// Backend implements the events.Backend interface for AWS.
type Backend struct {
	executionRepo    database.ExecutionRepository
	webSocketManager websocket.Manager
	logger           *slog.Logger
}

// NewBackend creates a new AWS event backend.
func NewBackend(
	executionRepo database.ExecutionRepository,
	webSocketManager websocket.Manager,
	logger *slog.Logger,
) *Backend {
	return &Backend{
		executionRepo:    executionRepo,
		webSocketManager: webSocketManager,
		logger:           logger,
	}
}

// HandleCloudEvent processes CloudWatch events (ECS task state changes).
func (b *Backend) HandleCloudEvent(
	ctx context.Context,
	rawEvent *json.RawMessage,
	reqLogger *slog.Logger,
) (bool, error) {
	var cwEvent events.CloudWatchEvent
	if err := json.Unmarshal(*rawEvent, &cwEvent); err != nil || cwEvent.Source == "" || cwEvent.DetailType == "" {
		return false, nil
	}

	reqLogger.Debug("processing CloudWatch event",
		"context", map[string]string{
			"source":      cwEvent.Source,
			"detail_type": cwEvent.DetailType,
		},
	)

	switch cwEvent.DetailType {
	case "ECS Task State Change":
		return true, b.handleECSTaskCompletion(ctx, &cwEvent, reqLogger)
	default:
		reqLogger.Warn("ignoring unhandled CloudWatch event detail type",
			"context", map[string]string{
				"detail_type": cwEvent.DetailType,
				"source":      cwEvent.Source,
			},
		)
		return true, nil
	}
}

// HandleLogsEvent processes CloudWatch Logs events.
func (b *Backend) HandleLogsEvent(
	ctx context.Context,
	rawEvent *json.RawMessage,
	reqLogger *slog.Logger,
) (bool, error) {
	var cwLogsEvent events.CloudwatchLogsEvent
	if err := json.Unmarshal(*rawEvent, &cwLogsEvent); err != nil || cwLogsEvent.AWSLogs.Data == "" {
		return false, nil
	}

	data, err := cwLogsEvent.AWSLogs.Parse()
	if err != nil {
		reqLogger.Error("failed to parse CloudWatch Logs data",
			"error", err,
		)
		return true, err
	}

	executionID := constants.ExtractExecutionIDFromLogStream(data.LogStream)
	if executionID == "" {
		reqLogger.Warn("unable to extract execution ID from log stream",
			"context", map[string]string{
				"log_stream": data.LogStream,
			},
		)
		return true, nil
	}

	reqLogger.Debug("processing CloudWatch logs event",
		"context", map[string]any{
			"log_group":    data.LogGroup,
			"log_stream":   data.LogStream,
			"execution_id": executionID,
			"log_count":    len(data.LogEvents),
		},
	)

	// Convert CloudWatch log events to api.LogEvent format
	logEvents := make([]api.LogEvent, 0, len(data.LogEvents))
	for _, cwLogEvent := range data.LogEvents {
		logEvents = append(logEvents, api.LogEvent{
			Timestamp: cwLogEvent.Timestamp,
			Message:   cwLogEvent.Message,
		})
	}

	sendErr := b.webSocketManager.SendLogsToExecution(ctx, &executionID, logEvents)
	if sendErr != nil {
		reqLogger.Error("failed to send logs to WebSocket connections",
			"error", sendErr,
			"execution_id", executionID,
		)
		// Don't return error - logs were processed correctly, connection issue shouldn't fail processing
	}

	return true, nil
}

// HandleWebSocketEvent processes API Gateway WebSocket events.
func (b *Backend) HandleWebSocketEvent(
	ctx context.Context,
	rawEvent *json.RawMessage,
	reqLogger *slog.Logger,
) (events.APIGatewayProxyResponse, bool) {
	var wsReq events.APIGatewayWebsocketProxyRequest
	if err := json.Unmarshal(*rawEvent, &wsReq); err != nil || wsReq.RequestContext.RouteKey == "" {
		return events.APIGatewayProxyResponse{}, false
	}

	// This is a WebSocket request, handle it through the manager
	if _, err := b.webSocketManager.HandleRequest(ctx, rawEvent, reqLogger); err != nil {
		// Return error response to API Gateway
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Internal server error: %v", err),
		}, true
	}

	// Build the response based on the route
	resp := events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "OK",
	}

	return resp, true
}

// handleECSTaskCompletion processes ECS Task State Change events
func (b *Backend) handleECSTaskCompletion(
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

	reqLogger.Info("pattern matched, processing ECS task completion",
		"execution", map[string]string{
			"execution_id":   executionID,
			"started_at":     taskEvent.StartedAt,
			"stop_code":      taskEvent.StopCode,
			"stopped_at":     taskEvent.StoppedAt,
			"stopped_reason": taskEvent.StoppedReason,
			"task_arn":       taskEvent.TaskArn,
		})

	execution, err := b.executionRepo.GetExecution(ctx, executionID)
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

	if err = b.executionRepo.UpdateExecution(ctx, execution); err != nil {
		reqLogger.Error("failed to update execution", "error", err)
		return fmt.Errorf("failed to update execution: %w", err)
	}

	reqLogger.Info("execution updated successfully", "execution", execution)

	// Notify WebSocket clients about the execution completion
	if err = b.webSocketManager.NotifyExecutionCompletion(ctx, &executionID); err != nil {
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
