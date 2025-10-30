package aws

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"

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

	foundStreams := make(map[string]struct{})
	// Deterministic stream from ECS awslogs default pattern: task/<container-name>/<executionID>
	stream := fmt.Sprintf("task/%s/%s", constants.RunnerContainerName, executionID)
	foundStreams[stream] = struct{}{}

	if len(foundStreams) == 0 {
		return []api.LogEvent{}, nil
	}

	var events []api.LogEvent
	for stream := range foundStreams {
		var nextToken *string
		for {
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
	}

	sort.SliceStable(events, func(i, j int) bool { return events[i].Timestamp < events[j].Timestamp })
	return events, nil
}
