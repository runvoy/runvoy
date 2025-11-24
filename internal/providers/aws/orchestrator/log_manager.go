// Package orchestrator provides AWS-specific implementations for runvoy orchestrator.
// This file contains the LogManager implementation for execution log retrieval.
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

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

// FetchLogsByExecutionID returns CloudWatch log events for the given execution ID.
//
//nolint:dupl // Duplicate with Provider for backwards compatibility
func (l *LogManagerImpl) FetchLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error) {
	if executionID == "" {
		return nil, appErrors.ErrBadRequest("executionID is required", nil)
	}

	stream := awsConstants.BuildLogStreamName(executionID)
	reqLogger := logger.DeriveRequestLogger(ctx, l.logger)

	if verifyErr := verifyLogStreamExists(
		ctx, l.cwlClient, l.cfg.LogGroup, stream, executionID, reqLogger,
	); verifyErr != nil {
		return nil, verifyErr
	}

	reqLogger.Debug("calling external service", "context", map[string]string{
		"operation":    "CloudWatchLogs.GetLogEvents",
		"log_group":    l.cfg.LogGroup,
		"log_stream":   stream,
		"execution_id": executionID,
		"paginated":    "true",
	})

	events, err := getAllLogEvents(ctx, l.cwlClient, l.cfg.LogGroup, stream)
	if err != nil {
		return nil, err
	}
	reqLogger.Debug("log events fetched successfully", "context", map[string]string{
		"events_count": fmt.Sprintf("%d", len(events)),
	})

	return events, nil
}
