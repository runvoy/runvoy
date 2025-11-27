package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"runvoy/internal/api"
	awsConstants "runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-lambda-go/events"
)

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
			EventID:   cwLogEvent.ID,
			Timestamp: cwLogEvent.Timestamp,
			Message:   cwLogEvent.Message,
		})
	}

	if err = p.logEventRepo.SaveLogEvents(ctx, executionID, logEvents); err != nil {
		reqLogger.Error("failed to persist log events", "error", err, "execution_id", executionID)
		return true, fmt.Errorf("failed to persist log events: %w", err)
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
