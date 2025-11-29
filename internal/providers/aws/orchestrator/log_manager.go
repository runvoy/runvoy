package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"runvoy/internal/api"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/logger"
	awsClient "runvoy/internal/providers/aws/client"
	awsConstants "runvoy/internal/providers/aws/constants"
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

// FetchLogsByExecutionID returns CloudWatch log events for the given execution ID.
// It fetches logs from both the runner and sidecar containers, merges them, and sorts by timestamp.
func (l *LogManagerImpl) FetchLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error) {
	if executionID == "" {
		return nil, appErrors.ErrBadRequest("executionID is required", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, l.logger)

	runnerStream := awsConstants.BuildLogStreamName(executionID)
	sidecarStream := buildSidecarLogStreamName(executionID)

	// Verify runner stream exists (required)
	if verifyErr := verifyLogStreamExists(
		ctx, l.cwlClient, l.cfg.LogGroup, runnerStream, executionID, reqLogger,
	); verifyErr != nil {
		return nil, verifyErr
	}

	// Fetch logs from runner stream
	reqLogger.Debug("calling external service", "context", map[string]string{
		"operation":    "CloudWatchLogs.GetLogEvents",
		"log_group":    l.cfg.LogGroup,
		"log_stream":   runnerStream,
		"execution_id": executionID,
		"paginated":    "true",
	})

	runnerEvents, err := getAllLogEvents(ctx, l.cwlClient, l.cfg.LogGroup, runnerStream)
	if err != nil {
		return nil, err
	}
	reqLogger.Debug("runner log events fetched successfully", "context", map[string]string{
		"events_count": fmt.Sprintf("%d", len(runnerEvents)),
	})

	// Try to fetch logs from sidecar stream (optional - may not exist if sidecar wasn't used)
	sidecarEvents := []api.LogEvent{}
	if verifyErr := verifyLogStreamExists(
		ctx, l.cwlClient, l.cfg.LogGroup, sidecarStream, executionID, reqLogger,
	); verifyErr == nil {
		reqLogger.Debug("calling external service", "context", map[string]string{
			"operation":    "CloudWatchLogs.GetLogEvents",
			"log_group":    l.cfg.LogGroup,
			"log_stream":   sidecarStream,
			"execution_id": executionID,
			"paginated":    "true",
		})

		fetchedSidecarEvents, err := getAllLogEvents(ctx, l.cwlClient, l.cfg.LogGroup, sidecarStream)
		if err != nil {
			// Log error but don't fail - sidecar logs are optional
			reqLogger.Debug("failed to fetch sidecar logs, continuing without them", "context", map[string]string{
				"error": err.Error(),
			})
		} else {
			sidecarEvents = fetchedSidecarEvents
			reqLogger.Debug("sidecar log events fetched successfully", "context", map[string]string{
				"events_count": fmt.Sprintf("%d", len(sidecarEvents)),
			})
		}
	} else {
		reqLogger.Debug("sidecar log stream does not exist, skipping", "context", map[string]string{
			"log_stream": sidecarStream,
		})
	}

	// Merge logs from both streams
	allEvents := append(runnerEvents, sidecarEvents...)

	// Sort by timestamp (ascending)
	slices.SortFunc(allEvents, func(a, b api.LogEvent) int {
		if a.Timestamp < b.Timestamp {
			return -1
		}
		if a.Timestamp > b.Timestamp {
			return 1
		}
		// If timestamps are equal, maintain relative order (stable sort)
		return 0
	})

	reqLogger.Debug("log events fetched and merged successfully", "context", map[string]string{
		"total_events":    fmt.Sprintf("%d", len(allEvents)),
		"runner_events":   fmt.Sprintf("%d", len(runnerEvents)),
		"sidecar_events":  fmt.Sprintf("%d", len(sidecarEvents)),
	})

	return allEvents, nil
}
