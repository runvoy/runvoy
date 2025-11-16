package aws

import (
	"fmt"
	"log/slog"
	"time"
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
