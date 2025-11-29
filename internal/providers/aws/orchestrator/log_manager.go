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

// fetchLogsFromStream fetches logs from a CloudWatch log stream.
func (l *LogManagerImpl) fetchLogsFromStream(
	ctx context.Context,
	reqLogger *slog.Logger,
	stream string,
	executionID string,
) ([]api.LogEvent, error) {
	reqLogger.Debug("calling external service", "context", map[string]string{
		"operation":    "CloudWatchLogs.GetLogEvents",
		"log_group":    l.cfg.LogGroup,
		"log_stream":   stream,
		"execution_id": executionID,
		"paginated":    "true",
	})

	events, fetchErr := getAllLogEvents(ctx, l.cwlClient, l.cfg.LogGroup, stream)
	if fetchErr != nil {
		return nil, fetchErr
	}

	reqLogger.Debug("log events fetched successfully", "context", map[string]string{
		"log_stream":   stream,
		"events_count": fmt.Sprintf("%d", len(events)),
	})

	return events, nil
}

// mergeAndSortLogs merges logs from runner and sidecar streams and sorts by timestamp.
func mergeAndSortLogs(runnerEvents, sidecarEvents []api.LogEvent) []api.LogEvent {
	allEvents := make([]api.LogEvent, 0, len(runnerEvents)+len(sidecarEvents))
	allEvents = append(allEvents, runnerEvents...)
	allEvents = append(allEvents, sidecarEvents...)

	slices.SortFunc(allEvents, func(a, b api.LogEvent) int {
		if a.Timestamp < b.Timestamp {
			return -1
		}
		if a.Timestamp > b.Timestamp {
			return 1
		}
		return 0
	})

	return allEvents
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

	// Fetch logs from runner stream
	runnerEvents, err := l.fetchLogsFromStream(ctx, reqLogger, runnerStream, executionID)
	if err != nil {
		return nil, err
	}

	// Fetch logs from sidecar stream (mandatory)
	sidecarEvents, err := l.fetchLogsFromStream(ctx, reqLogger, sidecarStream, executionID)
	if err != nil {
		return nil, err
	}

	// Merge and sort logs from both streams
	allEvents := mergeAndSortLogs(runnerEvents, sidecarEvents)

	reqLogger.Debug("log events fetched and merged successfully", "context", map[string]string{
		"total_events":   fmt.Sprintf("%d", len(allEvents)),
		"runner_events":  fmt.Sprintf("%d", len(runnerEvents)),
		"sidecar_events": fmt.Sprintf("%d", len(sidecarEvents)),
	})

	return allEvents, nil
}
