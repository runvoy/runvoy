package aws

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// FetchLogsByExecutionID queries CloudWatch Logs for events associated with the ECS task ID
// Returns an indexed slice of LogEvent sorted by timestamp.
func FetchLogsByExecutionID(ctx context.Context, cfg *Config, executionID string) ([]api.LogEvent, error) {
	if cfg == nil || cfg.LogGroup == "" {
		return nil, appErrors.ErrInternalError("CloudWatch Logs group not configured", nil)
	}
	if executionID == "" {
		return nil, appErrors.ErrBadRequest("executionID is required", nil)
	}

	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, appErrors.ErrInternalError("failed to load AWS configuration", err)
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
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(describeLogArgs))

	lsOut, err := cwl.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String(cfg.LogGroup),
		LogStreamNamePrefix: aws.String(stream),
		Limit:               aws.Int32(50),
	})
	if err != nil {
		return nil, appErrors.ErrInternalError("failed to describe log streams", err)
	}

	if !slices.ContainsFunc(lsOut.LogStreams, func(s cwlTypes.LogStream) bool {
		return aws.ToString(s.LogStreamName) == stream
	}) {
		return nil, appErrors.ErrNotFound(fmt.Sprintf("log stream '%s' does not exist yet", stream), nil)
	}

	reqLogger.Debug("calling external service", "context", map[string]string{
		"operation":   "CloudWatchLogs.GetLogEvents",
		"logGroup":    cfg.LogGroup,
		"logStream":   stream,
		"executionID": executionID,
		"paginated":   "true",
	})

	var events []api.LogEvent
	var nextToken *string
	pageCount := 0
	for {
		pageCount++

		out, err := cwl.GetLogEvents(ctx, &cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  &cfg.LogGroup,
			LogStreamName: &stream,
			NextToken:     nextToken,
			StartFromHead: aws.Bool(true),
			Limit:         aws.Int32(10000),
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

	reqLogger.Debug("log events fetched successfully", "context", map[string]int{
		"events_count": len(events),
	})

	sort.SliceStable(events, func(i, j int) bool { return events[i].Timestamp < events[j].Timestamp })
	for i := range events {
		events[i].Line = i + 1
	}
	return events, nil
}
