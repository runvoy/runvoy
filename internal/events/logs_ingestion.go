package events

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"runvoy/internal/api"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/events"
)

// handleCloudWatchLogs processes CloudWatch Logs subscription filter events
// and ingests them into the DynamoDB logs cache.
// The event is passed as a CloudWatchEvent by EventBridge.
func (p *Processor) handleCloudWatchLogs(
	ctx context.Context,
	event *events.CloudWatchEvent,
) error {
	reqLogger := logger.DeriveRequestLogger(ctx, p.logger)

	// The event.Detail contains the CloudWatch Logs event payload
	// Parse the CloudWatch Logs event from the detail field
	cwLogsEvent, err := ParseCloudWatchLogsEvent(event.Detail)
	if err != nil {
		reqLogger.Error("failed to parse CloudWatch Logs event", "error", err)
		return fmt.Errorf("failed to parse CloudWatch Logs event: %w", err)
	}

	// Extract execution ID from log stream name
	// Log stream name format: /aws/ecs/runvoy/runner-EXECUTION_ID
	executionID := extractExecutionIDFromLogStream(cwLogsEvent.LogStream)
	if executionID == "" {
		reqLogger.Warn("could not extract execution ID from log stream",
			"log_stream", cwLogsEvent.LogStream,
		)
		return nil
	}

	reqLogger.Debug("processing CloudWatch Logs ingestion",
		"context", map[string]any{
			"execution_id": executionID,
			"log_stream":   cwLogsEvent.LogStream,
			"event_count":  len(cwLogsEvent.LogEvents),
		},
	)

	// Ingest logs for this execution
	ingestedCount := p.ingestExecutionLogs(ctx, executionID, cwLogsEvent.LogEvents, reqLogger)

	reqLogger.Debug("logs ingested successfully",
		"context", map[string]any{
			"execution_id":   executionID,
			"ingested_count": ingestedCount,
		},
	)

	return nil
}

// ingestExecutionLogs ingests log events into the DynamoDB logs cache.
func (p *Processor) ingestExecutionLogs(
	ctx context.Context,
	executionID string,
	logEvents []CloudWatchLogEvent,
	reqLogger *slog.Logger,
) int {
	if p.logsRepo == nil {
		// Logs repository not initialized - skip ingestion
		return 0
	}

	ingestedCount := 0

	// Process each log event
	for _, logEvent := range logEvents {
		apiLogEvent := &api.LogEvent{
			Timestamp: logEvent.Timestamp,
			Message:   logEvent.Message,
		}

		err := p.logsRepo.CreateLogEvent(ctx, executionID, apiLogEvent)
		if err != nil {
			reqLogger.Warn("failed to ingest log event",
				"error", err,
				"execution_id", executionID,
				"timestamp", logEvent.Timestamp,
			)
			// Continue processing other logs on error (non-fatal)
			continue
		}

		ingestedCount++
	}

	return ingestedCount
}

// extractExecutionIDFromLogStream extracts the execution ID from a CloudWatch log stream name.
// Log stream name format: runner-EXECUTION_ID or similar patterns.
func extractExecutionIDFromLogStream(logStream string) string {
	const runnerPrefix = "runner-"
	const pathSeparator = "/"
	const minPartCount = 2

	// Log stream format from ECS: runner-<execution_id>
	// Extract the part after "runner-"
	if strings.Contains(logStream, runnerPrefix) {
		parts := strings.Split(logStream, runnerPrefix)
		if len(parts) == minPartCount {
			return parts[1]
		}
	}

	// Fallback: try to use the log stream name directly
	// if it looks like an execution ID (alphanumeric-dash pattern)
	if strings.Contains(logStream, "-") {
		// Get the last part after the last slash
		parts := strings.Split(logStream, pathSeparator)
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			// Check if it looks like an execution ID
			if lastPart != "" {
				return lastPart
			}
		}
	}

	return ""
}
