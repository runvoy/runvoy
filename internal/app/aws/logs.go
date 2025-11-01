package aws

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// FetchLogsByExecutionID queries CloudWatch Logs for events associated with the ECS task ID
func FetchLogsByExecutionID(ctx context.Context, cfg *Config, executionID string) ([]api.LogEvent, error) {
	if cfg == nil || cfg.LogGroup == "" {
		return nil, apperrors.ErrInternalError("CloudWatch Logs group not configured", nil)
	}
	if executionID == "" {
		return nil, apperrors.ErrBadRequest("executionID is required", nil)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, apperrors.ErrInternalError("failed to load AWS configuration", err)
	}
	cwl := cloudwatchlogs.NewFromConfig(awsCfg)
	stream := fmt.Sprintf("task/%s/%s", constants.RunnerContainerName, executionID)

	reqLogger := logger.DeriveRequestLogger(ctx, slog.Default())

	// Log before calling CloudWatch Logs DescribeLogStreams
	describeLogArgs := []any{
		"operation", "CloudWatchLogs.DescribeLogStreams",
		"logGroup", cfg.LogGroup,
		"streamPrefix", stream,
		"executionID", executionID,
	}
	describeLogArgs = append(describeLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "args", logger.SliceToMap(describeLogArgs))

	lsOut, err := cwl.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String(cfg.LogGroup),
		LogStreamNamePrefix: aws.String(stream),
		Limit:               aws.Int32(50),
	})
	if err != nil {
		return nil, apperrors.ErrInternalError("failed to describe log streams", err)
	}

	streamExists := false
	for _, s := range lsOut.LogStreams {
		if aws.ToString(s.LogStreamName) == stream {
			streamExists = true
			break
		}
	}
	if !streamExists {
		return nil, apperrors.ErrNotFound(fmt.Sprintf("log stream '%s' does not exist yet", stream), nil)
	}

	var events []api.LogEvent
	var nextToken *string
	pageCount := 0
	for {
		pageCount++

		// Log before calling CloudWatch Logs GetLogEvents
		getLogArgs := []any{
			"operation", "CloudWatchLogs.GetLogEvents",
			"logGroup", cfg.LogGroup,
			"logStream", stream,
			"executionID", executionID,
			"pageNumber", pageCount,
			"hasNextToken", nextToken != nil,
		}
		getLogArgs = append(getLogArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("calling external service", "args", logger.SliceToMap(getLogArgs))

		out, err := cwl.GetLogEvents(ctx, &cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  &cfg.LogGroup,
			LogStreamName: &stream,
			NextToken:     nextToken,
			StartFromHead: aws.Bool(true),
			Limit:         aws.Int32(10000),
		})

		if err != nil {
			var rte *cwltypes.ResourceNotFoundException
			if errors.As(err, &rte) {
				break
			}
			return nil, apperrors.ErrInternalError("failed to get log events", err)
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

	sort.SliceStable(events, func(i, j int) bool { return events[i].Timestamp < events[j].Timestamp })
	// Assign serial line numbers starting at 1
	for i := range events {
		events[i].Line = i + 1
	}
	return events, nil
}
