// Package cloudwatch provides CloudWatch-based implementations of repository interfaces.
package cloudwatch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// LogRepository implements database.LogRepository using CloudWatch Logs.
type LogRepository struct {
	logGroup string
	logger   *slog.Logger
}

// NewLogRepository creates a new CloudWatch-backed log repository.
func NewLogRepository(logGroup string, log *slog.Logger) database.LogRepository {
	return &LogRepository{
		logGroup: logGroup,
		logger:   log,
	}
}

// GetLogsByExecutionID retrieves all logs for an execution from CloudWatch Logs.
func (r *LogRepository) GetLogsByExecutionID(
	ctx context.Context,
	executionID string,
) ([]api.LogEvent, error) {
	if executionID == "" {
		return nil, appErrors.ErrBadRequest("executionID is required", nil)
	}

	cwl, err := r.newCloudWatchLogsClient(ctx)
	if err != nil {
		return nil, err
	}

	return r.fetchLogs(ctx, cwl, executionID, nil)
}

// GetLogsByExecutionIDSince retrieves logs newer than the given timestamp from CloudWatch Logs.
func (r *LogRepository) GetLogsByExecutionIDSince(
	ctx context.Context,
	executionID string,
	sinceTimestampMS *int64,
) ([]api.LogEvent, error) {
	if executionID == "" {
		return nil, appErrors.ErrBadRequest("executionID is required", nil)
	}

	cwl, err := r.newCloudWatchLogsClient(ctx)
	if err != nil {
		return nil, err
	}

	return r.fetchLogs(ctx, cwl, executionID, sinceTimestampMS)
}

// fetchLogs is the internal method that retrieves logs with optional timestamp filtering.
func (r *LogRepository) fetchLogs(
	ctx context.Context,
	cwl *cloudwatchlogs.Client,
	executionID string,
	sinceTimestampMS *int64,
) ([]api.LogEvent, error) {
	stream := constants.BuildLogStreamName(executionID)
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	if verifyErr := r.verifyLogStreamExists(ctx, cwl, stream, executionID, reqLogger); verifyErr != nil {
		return nil, verifyErr
	}

	logArgs := []any{
		"operation", "CloudWatchLogs.GetLogEvents",
		"log_group", r.logGroup,
		"log_stream", stream,
		"execution_id", executionID,
		"paginated", "true",
	}
	if sinceTimestampMS != nil {
		logArgs = append(logArgs, "since_timestamp_ms", *sinceTimestampMS)
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	events, err := r.getAllLogEvents(ctx, cwl, stream, sinceTimestampMS)
	if err != nil {
		return nil, err
	}

	reqLogger.Debug("log events fetched successfully", "context", map[string]string{
		"events_count": fmt.Sprintf("%d", len(events)),
	})

	// Sort events by timestamp (ascending order)
	sort.SliceStable(events, func(i, j int) bool { return events[i].Timestamp < events[j].Timestamp })
	return events, nil
}

// verifyLogStreamExists checks if the log stream exists and returns an error if it doesn't.
func (r *LogRepository) verifyLogStreamExists(
	ctx context.Context,
	cwl *cloudwatchlogs.Client,
	stream, executionID string,
	reqLogger *slog.Logger,
) error {
	describeLogArgs := []any{
		"operation", "CloudWatchLogs.DescribeLogStreams",
		"log_group", r.logGroup,
		"stream_prefix", stream,
		"execution_id", executionID,
	}
	describeLogArgs = append(describeLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(describeLogArgs))

	lsOut, err := cwl.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String(r.logGroup),
		LogStreamNamePrefix: aws.String(stream),
		Limit:               aws.Int32(constants.CloudWatchLogsDescribeLimit),
	})
	if err != nil {
		return appErrors.ErrInternalError("failed to describe log streams", err)
	}

	found := false
	for _, s := range lsOut.LogStreams {
		if aws.ToString(s.LogStreamName) == stream {
			found = true
			break
		}
	}

	if !found {
		return appErrors.ErrNotFound(fmt.Sprintf("log stream '%s' does not exist yet", stream), nil)
	}

	return nil
}

// getAllLogEvents paginates through CloudWatch Logs GetLogEvents to collect all events.
// If sinceTimestampMS is provided, filters to only return events newer than that timestamp.
func (r *LogRepository) getAllLogEvents(
	ctx context.Context,
	cwl *cloudwatchlogs.Client,
	stream string,
	sinceTimestampMS *int64,
) ([]api.LogEvent, error) {
	var events []api.LogEvent
	var nextToken *string

	for {
		out, err := cwl.GetLogEvents(ctx, &cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  aws.String(r.logGroup),
			LogStreamName: aws.String(stream),
			NextToken:     nextToken,
			StartFromHead: aws.Bool(true),
			Limit:         aws.Int32(constants.CloudWatchLogsEventsLimit),
		})

		if err != nil {
			var rte *cwlTypes.ResourceNotFoundException
			if errors.As(err, &rte) {
				break
			}
			return nil, appErrors.ErrInternalError("failed to get log events", err)
		}

		for _, e := range out.Events {
			timestamp := aws.ToInt64(e.Timestamp)
			// Filter by sinceTimestampMS if provided
			if sinceTimestampMS != nil && timestamp <= *sinceTimestampMS {
				continue
			}
			events = append(events, api.LogEvent{
				Timestamp: timestamp,
				Message:   aws.ToString(e.Message),
			})
		}

		if out.NextForwardToken == nil || (nextToken != nil && *out.NextForwardToken == *nextToken) {
			break
		}
		nextToken = out.NextForwardToken
	}

	return events, nil
}

// newCloudWatchLogsClient creates a new CloudWatch Logs client.
func (r *LogRepository) newCloudWatchLogsClient(ctx context.Context) (*cloudwatchlogs.Client, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, appErrors.ErrInternalError("failed to load AWS configuration", err)
	}
	return cloudwatchlogs.NewFromConfig(awsCfg), nil
}
