package lambdaapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"runvoy/internal/app"
	apperrors "runvoy/internal/errors"

	"github.com/aws/aws-lambda-go/events"
)

// ECSTaskStateChangeEvent represents the ECS Task State Change event structure
type ECSTaskStateChangeEvent struct {
	Detail struct {
		LastStatus     string `json:"lastStatus"`
		StartedBy      string `json:"startedBy"`
		StoppedAt      string `json:"stoppedAt"` // ECS sends this as a string timestamp
		StoppedReason  string `json:"stoppedReason,omitempty"`
		Containers     []struct {
			ExitCode *int   `json:"exitCode,omitempty"`
			Name     string `json:"name"`
		} `json:"containers"`
		ClusterArn string `json:"clusterArn"`
		TaskArn    string `json:"taskArn"`
	} `json:"detail"`
}

// ECSEventHandler handles EventBridge events for ECS Task State Change
type ECSEventHandler struct {
	svc    *app.Service
	logger *slog.Logger
}

// NewECSEventHandler creates a new handler for ECS Task State Change events
func NewECSEventHandler(svc *app.Service, logger *slog.Logger) *ECSEventHandler {
	return &ECSEventHandler{
		svc:    svc,
		logger: logger,
	}
}

// HandleEventBridgeEvent processes EventBridge events, specifically ECS Task State Change events
func (h *ECSEventHandler) HandleEventBridgeEvent(ctx context.Context, event events.CloudWatchEventsEvent) error {
	h.logger.Debug("received EventBridge event",
		"source", event.Source,
		"detailType", event.DetailType,
		"id", event.ID,
	)

	// Only process ECS Task State Change events
	if event.Source != "aws.ecs" || event.DetailType != "ECS Task State Change" {
		h.logger.Debug("ignoring non-ECS event", "source", event.Source, "detailType", event.DetailType)
		return nil
	}

	var taskEvent ECSTaskStateChangeEvent
	if err := json.Unmarshal(event.Detail, &taskEvent); err != nil {
		h.logger.Error("failed to unmarshal ECS task event", "error", err)
		return apperrors.ErrInternalError("failed to parse ECS task event", err)
	}

	// Only process STOPPED events
	if taskEvent.Detail.LastStatus != "STOPPED" {
		h.logger.Debug("task not stopped, ignoring", "lastStatus", taskEvent.Detail.LastStatus)
		return nil
	}

	// Extract execution ID from startedBy field
	executionID := taskEvent.Detail.StartedBy
	if executionID == "" {
		h.logger.Warn("ECS task event missing startedBy field, cannot correlate with execution",
			"taskArn", taskEvent.Detail.TaskArn,
		)
		return nil
	}

	h.logger.Info("processing ECS task completion event",
		"executionID", executionID,
		"taskArn", taskEvent.Detail.TaskArn,
		"stoppedAt", taskEvent.Detail.StoppedAt,
	)

	// Parse stoppedAt timestamp (ECS provides this as RFC3339 string)
	var stoppedAt time.Time
	if taskEvent.Detail.StoppedAt != "" {
		var err error
		stoppedAt, err = time.Parse(time.RFC3339, taskEvent.Detail.StoppedAt)
		if err != nil {
			h.logger.Error("failed to parse stoppedAt timestamp", "error", err, "stoppedAt", taskEvent.Detail.StoppedAt)
			return apperrors.ErrInternalError("failed to parse stoppedAt timestamp", err)
		}
	}

	// Get the execution record to retrieve StartedAt for duration calculation
	execution, err := h.svc.GetExecution(ctx, executionID)
	if err != nil {
		h.logger.Error("failed to get execution record", "error", err, "executionID", executionID)
		return apperrors.ErrDatabaseError("failed to get execution record", err)
	}

	if execution == nil {
		h.logger.Warn("execution record not found, cannot update",
			"executionID", executionID,
		)
		return nil
	}

	// Determine exit code from containers (use first container's exit code)
	exitCode := 0
	for _, container := range taskEvent.Detail.Containers {
		if container.ExitCode != nil {
			exitCode = *container.ExitCode
			break
		}
	}

	// Determine status based on exit code
	status := "completed"
	if exitCode != 0 {
		status = "failed"
	}

	// Calculate duration
	durationSeconds := 0
	if !stoppedAt.IsZero() {
		durationSeconds = int(stoppedAt.Sub(execution.StartedAt).Seconds())
		if durationSeconds < 0 {
			durationSeconds = 0 // Handle clock skew
		}
	}

	// Update execution record
	execution.Status = status
	execution.ExitCode = exitCode
	execution.CompletedAt = &stoppedAt
	execution.DurationSeconds = durationSeconds

	if err := h.svc.UpdateExecution(ctx, execution); err != nil {
		h.logger.Error("failed to update execution record", "error", err, "executionID", executionID)
		return apperrors.ErrDatabaseError("failed to update execution record", err)
	}

	h.logger.Info("execution record updated successfully",
		"executionID", executionID,
		"status", status,
		"exitCode", exitCode,
		"durationSeconds", durationSeconds,
	)

	return nil
}