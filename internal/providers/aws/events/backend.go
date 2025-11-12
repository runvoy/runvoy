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
	"runvoy/internal/app/websocket"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	"runvoy/internal/logger"
	awsConstants "runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-lambda-go/events"
)

// Processor implements the events.Processor interface for AWS.
// It handles CloudWatch events, CloudWatch Logs, and API Gateway WebSocket events.
type Processor struct {
	executionRepo    database.ExecutionRepository
	webSocketManager websocket.Manager
	logger           *slog.Logger
}

// NewProcessor creates a new AWS event processor.
func NewProcessor(
	executionRepo database.ExecutionRepository,
	webSocketManager websocket.Manager,
	log *slog.Logger,
) *Processor {
	return &Processor{
		executionRepo:    executionRepo,
		webSocketManager: webSocketManager,
		logger:           log,
	}
}

// Handle processes a raw AWS event by delegating to the appropriate handler.
// It supports CloudWatch events, CloudWatch Logs, and WebSocket events.
func (p *Processor) Handle(ctx context.Context, rawEvent *json.RawMessage) (*json.RawMessage, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	// Try cloud-specific events
	if handled, err := p.handleCloudEvent(ctx, rawEvent, reqLogger); handled {
		return nil, err
	}

	// Try logs events
	if handled, err := p.handleLogsEvent(ctx, rawEvent, reqLogger); handled {
		return nil, err
	}

	// Try WebSocket events
	if resp, handled := p.handleWebSocketEvent(ctx, rawEvent, reqLogger); handled {
		marshaled, err := json.Marshal(resp)
		if err != nil {
			reqLogger.Error("failed to marshal response", "error", err)
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}
		result := json.RawMessage(marshaled)
		return &result, nil
	}

	return nil, fmt.Errorf("unhandled event type: %s", string(*rawEvent))
}

// HandleEventJSON is a helper for testing that accepts raw JSON and returns an error.
// It's used for test cases that expect error returns.
func (p *Processor) HandleEventJSON(ctx context.Context, eventJSON *json.RawMessage) error {
	var event events.CloudWatchEvent
	if err := json.Unmarshal(*eventJSON, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	if _, err := p.Handle(ctx, eventJSON); err != nil {
		return err
	}
	return nil
}

// handleCloudEvent processes CloudWatch events (ECS task state changes).
func (p *Processor) handleCloudEvent(
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
		return true, p.handleECSTaskCompletion(ctx, &cwEvent, reqLogger)
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

// handleLogsEvent processes CloudWatch Logs events.
func (p *Processor) handleLogsEvent(
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

	executionID := awsConstants.ExtractExecutionIDFromLogStream(data.LogStream)
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

	sendErr := p.webSocketManager.SendLogsToExecution(ctx, &executionID, logEvents)
	if sendErr != nil {
		reqLogger.Error("failed to send logs to WebSocket connections",
			"error", sendErr,
			"execution_id", executionID,
		)
		// Don't return error - logs were processed correctly, connection issue shouldn't fail processing
	}

	return true, nil
}

// handleWebSocketEvent processes API Gateway WebSocket events.
func (p *Processor) handleWebSocketEvent(
	ctx context.Context,
	rawEvent *json.RawMessage,
	reqLogger *slog.Logger,
) (events.APIGatewayProxyResponse, bool) {
	var wsReq events.APIGatewayWebsocketProxyRequest
	if err := json.Unmarshal(*rawEvent, &wsReq); err != nil || wsReq.RequestContext.RouteKey == "" {
		return events.APIGatewayProxyResponse{}, false
	}

	// This is a WebSocket request, handle it through the manager
	if _, err := p.webSocketManager.HandleRequest(ctx, rawEvent, reqLogger); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Internal server error: %v", err),
		}, true
	}

	resp := events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "OK",
	}

	return resp, true
}

// handleECSTaskCompletion processes ECS Task State Change events
func (p *Processor) handleECSTaskCompletion(
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
		// Don't fail for orphaned tasks - they might have been started manually?
		// TODO: figure out what to do with orphaned tasks or if we should fail the Lambda
		return nil
	}

	switch awsConstants.EcsStatus(taskEvent.LastStatus) {
	case awsConstants.EcsStatusRunning:
		return p.updateExecutionToRunning(ctx, executionID, execution, reqLogger)
	case awsConstants.EcsStatusStopped:
		return p.finalizeExecutionFromTaskEvent(ctx, executionID, execution, &taskEvent, reqLogger)
	default:
		reqLogger.Debug("ignoring ECS task status update",
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
	for _, terminal := range constants.TerminalExecutionStatuses() {
		if execution.Status == string(terminal) {
			reqLogger.Debug("skipping RUNNING update for terminal execution",
				"context", map[string]string{
					"execution_id": executionID,
					"status":       execution.Status,
				},
			)
			return nil
		}
	}

	if execution.Status == string(constants.ExecutionRunning) {
		reqLogger.Debug("execution already marked as RUNNING",
			"context", map[string]string{
				"execution_id": executionID,
			},
		)
		return nil
	}

	execution.Status = string(constants.ExecutionRunning)
	execution.CompletedAt = nil

	if err := p.executionRepo.UpdateExecution(ctx, execution); err != nil {
		reqLogger.Error("failed to update execution status to RUNNING",
			"error", err,
			"execution_id", executionID,
		)
		return fmt.Errorf("failed to update execution to running: %w", err)
	}

	reqLogger.Info("execution marked as RUNNING",
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

	execution.Status = status
	execution.ExitCode = exitCode
	execution.CompletedAt = &stoppedAt
	execution.DurationSeconds = durationSeconds

	if err = p.executionRepo.UpdateExecution(ctx, execution); err != nil {
		reqLogger.Error("failed to update execution", "error", err)
		return fmt.Errorf("failed to update execution: %w", err)
	}

	reqLogger.Info("execution updated successfully", "execution", execution)

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
