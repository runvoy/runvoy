package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/auth"
	awsConstants "github.com/runvoy/runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-lambda-go/events"
)

// convertCloudWatchLogEvents converts CloudWatch Logs events to api.LogEvent format.
func convertCloudWatchLogEvents(reqLogger *slog.Logger, cwLogEvents []events.CloudwatchLogsLogEvent) []api.LogEvent {
	logEvents := make([]api.LogEvent, 0, len(cwLogEvents))
	for _, cwLogEvent := range cwLogEvents {
		eventID := cwLogEvent.ID
		if eventID == "" {
			eventID = auth.GenerateEventID(cwLogEvent.Timestamp, cwLogEvent.Message)
			reqLogger.Warn("no event ID found, generating from timestamp + message", "context", map[string]any{
				"timestamp":          cwLogEvent.Timestamp,
				"message":            cwLogEvent.Message,
				"generated_event_id": eventID,
			})
		}
		logEvents = append(logEvents, api.LogEvent{
			EventID:   eventID,
			Timestamp: cwLogEvent.Timestamp,
			Message:   cwLogEvent.Message,
		})
	}
	return logEvents
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

	logEvents := convertCloudWatchLogEvents(reqLogger, data.LogEvents)

	if err = p.logEventRepo.SaveLogEvents(ctx, executionID, logEvents); err != nil {
		reqLogger.Error("failed to persist log events", "error", err, "execution_id", executionID)
		return true, fmt.Errorf("failed to persist log events: %w", err)
	}

	sendErr := p.webSocketManager.SendLogsToExecution(ctx, &executionID)
	if sendErr != nil {
		reqLogger.Error("failed to send logs to WebSocket connections",
			"error", sendErr,
			"execution_id", executionID,
		)
		// Don't return error - logs were processed correctly, connection issue shouldn't fail processing
	}

	return true, nil
}
