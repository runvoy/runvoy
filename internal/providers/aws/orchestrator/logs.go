package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	appErrors "github.com/runvoy/runvoy/internal/errors"
	"github.com/runvoy/runvoy/internal/logger"
	awsClient "github.com/runvoy/runvoy/internal/providers/aws/client"
	awsConstants "github.com/runvoy/runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// verifyLogStreamExists checks if the log stream exists and returns an error if it doesn't
func verifyLogStreamExists(
	ctx context.Context,
	cwl awsClient.CloudWatchLogsClient,
	logGroup, stream, executionID string,
	reqLogger *slog.Logger,
) error {
	describeLogArgs := []any{
		"operation", "CloudWatchLogs.DescribeLogStreams",
		"log_group", logGroup,
		"stream_prefix", stream,
		"execution_id", executionID,
	}
	describeLogArgs = append(describeLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(describeLogArgs))

	lsOut, err := cwl.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String(logGroup),
		LogStreamNamePrefix: aws.String(stream),
		Limit:               aws.Int32(awsConstants.CloudWatchLogsDescribeLimit),
	})
	if err != nil {
		return appErrors.ErrInternalError("failed to describe log streams", err)
	}

	if !slices.ContainsFunc(lsOut.LogStreams, func(s cwlTypes.LogStream) bool {
		return aws.ToString(s.LogStreamName) == stream
	}) {
		return appErrors.ErrServiceUnavailable(fmt.Sprintf("log stream '%s' does not exist yet", stream), nil)
	}

	return nil
}

// getAllLogEvents paginates through CloudWatch Logs FilterLogEvents to collect all events
// for the provided log group and stream. It returns the aggregated events with eventIDs
// sorted by timestamp or an error.
func getAllLogEvents(ctx context.Context,
	cwl awsClient.CloudWatchLogsClient, logGroup string, stream string) ([]api.LogEvent, error) {
	var events []api.LogEvent
	var nextToken *string
	pageCount := 0
	for {
		pageCount++

		out, err := cwl.FilterLogEvents(ctx, &cloudwatchlogs.FilterLogEventsInput{
			LogGroupName:   aws.String(logGroup),
			LogStreamNames: []string{stream},
			NextToken:      nextToken,
			Limit:          aws.Int32(awsConstants.CloudWatchLogsEventsLimit),
		})

		if err != nil {
			var rte *cwlTypes.ResourceNotFoundException
			if errors.As(err, &rte) {
				break
			}
			return nil, appErrors.ErrInternalError("failed to filter log events", err)
		}
		for _, e := range out.Events {
			message := aws.ToString(e.Message)
			timestamp := aws.ToInt64(e.Timestamp)

			eventID := ""
			if e.EventId != nil && *e.EventId != "" {
				eventID = *e.EventId
			} else {
				// Generate deterministic eventID from timestamp + message hash
				eventID = generateEventID(timestamp, message)
			}

			events = append(events, api.LogEvent{
				EventID:   eventID,
				Timestamp: timestamp,
				Message:   message,
			})
		}
		if out.NextToken == nil || (nextToken != nil && *out.NextToken == *nextToken) {
			break
		}
		nextToken = out.NextToken
	}
	return events, nil
}

// parseMessageTimestamp extracts the timestamp from JSON-formatted log messages.
// Expected format: {"time":"2025-11-21T01:00:24.951407774Z",...}
// Returns true if successfully parsed, false otherwise.
func parseMessageTimestamp(logEntry *api.LogEvent, message string) bool {
	var jsonMsg map[string]any
	if err := json.Unmarshal([]byte(message), &jsonMsg); err != nil {
		return false // Not JSON, will try CloudWatch timestamp instead
	}

	timeStr, ok := jsonMsg["time"].(string)
	if !ok {
		return false
	}

	// CloudWatch logs use RFC3339Nano format
	t, err := time.Parse(time.RFC3339Nano, timeStr)
	if err != nil {
		return false
	}

	logEntry.Timestamp = t.UnixMilli()
	return true
}

// parseCloudWatchTimestamp parses the @timestamp field from CloudWatch Logs Insights.
// Handles both RFC3339 and RFC3339Nano formats.
func parseCloudWatchTimestamp(logEntry *api.LogEvent, timestamp string) {
	if timestamp == "" {
		return
	}

	// Try RFC3339Nano first (most common from CloudWatch)
	t, err := time.Parse(time.RFC3339Nano, timestamp)
	if err == nil {
		logEntry.Timestamp = t.UnixMilli()
		return
	}

	// Fall back to RFC3339
	t, err = time.Parse(time.RFC3339, timestamp)
	if err == nil {
		logEntry.Timestamp = t.UnixMilli()
	}
}

// generateEventID creates a deterministic unique event ID from timestamp and message.
func generateEventID(timestamp int64, message string) string {
	var buf []byte
	buf = fmt.Appendf(buf, "%d:%s", timestamp, message)
	hash := sha256.Sum256(buf)
	return hex.EncodeToString(hash[:])[:16] // Use first 16 bytes (32 hex chars)
}
