package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/runvoy/runvoy/internal/api"
	appErrors "github.com/runvoy/runvoy/internal/errors"
	"github.com/runvoy/runvoy/internal/logger"
	awsClient "github.com/runvoy/runvoy/internal/providers/aws/client"
	awsConstants "github.com/runvoy/runvoy/internal/providers/aws/constants"
)

// LogManagerImpl implements the LogManager interface for AWS CloudWatch Logs.
// It handles retrieving execution logs from CloudWatch.
type LogManagerImpl struct {
	cwlClient awsClient.CloudWatchLogsClient
	cfg       *Config
	logger    *slog.Logger
}

// NewLogManager creates a new AWS log manager.
func NewLogManager(
	cwlClient awsClient.CloudWatchLogsClient,
	cfg *Config,
	log *slog.Logger,
) *LogManagerImpl {
	return &LogManagerImpl{
		cwlClient: cwlClient,
		cfg:       cfg,
		logger:    log,
	}
}

// buildSidecarLogStreamName constructs a CloudWatch Logs stream name for the sidecar container.
// Format: task/sidecar/{execution_id}
func buildSidecarLogStreamName(executionID string) string {
	return "task/" + awsConstants.SidecarContainerName + "/" + executionID
}

// sortLogsByTimestamp sorts log events by timestamp in ascending order.
func sortLogsByTimestamp(events []api.LogEvent) []api.LogEvent {
	slices.SortFunc(events, func(a, b api.LogEvent) int {
		if a.Timestamp < b.Timestamp {
			return -1
		}
		if a.Timestamp > b.Timestamp {
			return 1
		}
		return 0
	})
	return events
}

// FetchLogsByExecutionID returns CloudWatch log events for the given execution ID.
// It fetches logs from both the runner and sidecar containers, merges them, and sorts by timestamp.
// Sidecar logs are mandatory as sidecars always run.
func (l *LogManagerImpl) FetchLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error) {
	if executionID == "" {
		return nil, appErrors.ErrBadRequest("executionID is required", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, l.logger)

	runnerStream := awsConstants.BuildLogStreamName(executionID)
	sidecarStream := buildSidecarLogStreamName(executionID)

	// Verify both streams exist (both are required)
	if verifyErr := verifyLogStreamExists(
		ctx, l.cwlClient, l.cfg.LogGroup, runnerStream, executionID, reqLogger,
	); verifyErr != nil {
		return nil, verifyErr
	}

	if verifyErr := verifyLogStreamExists(
		ctx, l.cwlClient, l.cfg.LogGroup, sidecarStream, executionID, reqLogger,
	); verifyErr != nil {
		return nil, verifyErr
	}

	// Fetch logs from both runner and sidecar streams in a single API call
	reqLogger.Debug("calling external service", "context", map[string]any{
		"operation":    "CloudWatchLogs.FilterLogEvents",
		"log_group":    l.cfg.LogGroup,
		"log_streams":  []string{runnerStream, sidecarStream},
		"execution_id": executionID,
		"paginated":    "true",
	})

	// Pass 0 as startTime to fetch all logs from the beginning of the streams,
	// matching the previous behavior of GetLogEvents with StartFromHead=true.
	allEvents, err := getAllLogEvents(ctx, l.cwlClient, l.cfg.LogGroup, []string{runnerStream, sidecarStream}, 0)
	if err != nil {
		return nil, err
	}

	// Sort logs by timestamp (events from both streams are already merged by AWS)
	allEvents = sortLogsByTimestamp(allEvents)

	reqLogger.Debug("log events fetched successfully", "context", map[string]string{
		"total_events": fmt.Sprintf("%d", len(allEvents)),
	})

	return allEvents, nil
}
