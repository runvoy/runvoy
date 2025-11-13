package aws

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"runvoy/internal/api"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/logger"
	awsConstants "runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// FetchLogsByExecutionID queries CloudWatch Logs for events associated with the ECS task ID
// Returns a slice of LogEvent sorted by timestamp.
func FetchLogsByExecutionID(ctx context.Context, cfg *Config, awsCfg *aws.Config, executionID string) ([]api.LogEvent, error) {
	if executionID == "" {
		return nil, appErrors.ErrBadRequest("executionID is required", nil)
	}

	cwl := cloudwatchlogs.NewFromConfig(*awsCfg)
	stream := awsConstants.BuildLogStreamName(executionID)
	reqLogger := logger.DeriveRequestLogger(ctx, slog.Default())

	if verifyErr := verifyLogStreamExists(ctx, cwl, cfg.LogGroup, stream, executionID, reqLogger); verifyErr != nil {
		return nil, verifyErr
	}

	reqLogger.Debug("calling external service", "context", map[string]string{
		"operation":    "CloudWatchLogs.GetLogEvents",
		"log_group":    cfg.LogGroup,
		"log_stream":   stream,
		"execution_id": executionID,
		"paginated":    "true",
	})

	var events []api.LogEvent
	events, err := getAllLogEvents(ctx, cwl, cfg.LogGroup, stream)
	if err != nil {
		return nil, err
	}
	reqLogger.Debug("log events fetched successfully", "context", map[string]string{
		"events_count": fmt.Sprintf("%d", len(events)),
	})

	return events, nil
}

// verifyLogStreamExists checks if the log stream exists and returns an error if it doesn't
func verifyLogStreamExists(
	ctx context.Context,
	cwl *cloudwatchlogs.Client,
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

// getAllLogEvents paginates through CloudWatch Logs GetLogEvents to collect all events
// for the provided log group and stream. It returns the aggregated sorted by timestamp
// events or an error.
func getAllLogEvents(ctx context.Context,
	cwl *cloudwatchlogs.Client, logGroup string, stream string) ([]api.LogEvent, error) {
	var events []api.LogEvent
	var nextToken *string
	pageCount := 0
	for {
		pageCount++

		out, err := cwl.GetLogEvents(ctx, &cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  &logGroup,
			LogStreamName: &stream,
			NextToken:     nextToken,
			StartFromHead: aws.Bool(true),
			Limit:         aws.Int32(awsConstants.CloudWatchLogsEventsLimit),
		})

		if err != nil {
			var rte *cwlTypes.ResourceNotFoundException
			if errors.As(err, &rte) {
				break
			}
			return nil, appErrors.ErrInternalError("failed to get log events", err)
		}
		for _, e := range out.Events {
			events = append(events, api.LogEvent{
				Timestamp: aws.ToInt64(e.Timestamp),
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
