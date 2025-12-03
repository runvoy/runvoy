package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/auth"
	"github.com/runvoy/runvoy/internal/constants"
	appErrors "github.com/runvoy/runvoy/internal/errors"
	"github.com/runvoy/runvoy/internal/logger"
	awsClient "github.com/runvoy/runvoy/internal/providers/aws/client"
	awsConstants "github.com/runvoy/runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// ObservabilityManagerImpl implements the ObservabilityManager interface for AWS CloudWatch Logs.
// It handles retrieving backend infrastructure logs for debugging and tracing.
type ObservabilityManagerImpl struct {
	cwlClient awsClient.CloudWatchLogsClient
	logger    *slog.Logger
	nowFn     func() time.Time
}

// NewObservabilityManager creates a new AWS observability manager.
func NewObservabilityManager(
	cwlClient awsClient.CloudWatchLogsClient,
	log *slog.Logger,
) *ObservabilityManagerImpl {
	return &ObservabilityManagerImpl{
		cwlClient: cwlClient,
		logger:    log,
		nowFn:     time.Now,
	}
}

// FetchBackendLogs retrieves backend infrastructure logs using CloudWatch Logs FilterLogEvents.
// It scans all runvoy Lambda log groups for entries that contain the provided request ID.
func (o *ObservabilityManagerImpl) FetchBackendLogs(ctx context.Context, requestID string) ([]api.LogEvent, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, o.logger)

	logGroups, err := o.discoverLogGroups(ctx, reqLogger)
	if err != nil {
		return nil, err
	}

	if len(logGroups) == 0 {
		return nil, appErrors.ErrServiceUnavailable("no Lambda log groups found matching prefix", nil)
	}

	startMillis, endMillis := o.lookbackWindowMillis()
	filterPattern := o.buildFilterPattern(requestID)

	var allLogs []api.LogEvent
	for _, group := range logGroups {
		groupLogger := reqLogger.With("log_group", group)
		groupLogs, fetchErr := o.fetchLogEventsForGroup(
			ctx,
			groupLogger,
			group,
			requestID,
			filterPattern,
			startMillis,
			endMillis,
		)
		if fetchErr != nil {
			return nil, fetchErr
		}
		allLogs = append(allLogs, groupLogs...)
	}

	o.sortLogEvents(allLogs)
	return allLogs, nil
}

// discoverLogGroups discovers all log groups matching the runvoy Lambda prefix.
func (o *ObservabilityManagerImpl) discoverLogGroups(ctx context.Context, reqLogger *slog.Logger) ([]string, error) {
	log := reqLogger
	if log == nil {
		log = logger.DeriveRequestLogger(ctx, o.logger)
	}

	logGroups := []string{}
	var nextToken *string
	page := 0

	for {
		page++
		describeArgs := []any{
			"operation", "CloudWatchLogs.DescribeLogGroups",
			"page", page,
		}
		if nextToken != nil {
			describeArgs = append(describeArgs, "next_token", aws.ToString(nextToken))
		}
		describeArgs = append(describeArgs, logger.GetDeadlineInfo(ctx)...)
		log.Debug("calling external service", "context", logger.SliceToMap(describeArgs))

		out, err := o.cwlClient.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
			LogGroupNamePrefix: aws.String(awsConstants.LogGroupPrefix),
			NextToken:          nextToken,
		})
		if err != nil {
			return nil, appErrors.ErrInternalError("failed to discover log groups", err)
		}

		for i := range out.LogGroups {
			logGroups = append(logGroups, aws.ToString(out.LogGroups[i].LogGroupName))
		}

		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	return logGroups, nil
}

func (o *ObservabilityManagerImpl) fetchLogEventsForGroup(
	ctx context.Context,
	reqLogger *slog.Logger,
	logGroup string,
	requestID string,
	filterPattern string,
	startMillis int64,
	endMillis int64,
) ([]api.LogEvent, error) {
	log := reqLogger
	if log == nil {
		log = logger.DeriveRequestLogger(ctx, o.logger)
	}

	var events []api.LogEvent
	var nextToken *string
	page := 0

	for {
		page++
		filterArgs := []any{
			"operation", "CloudWatchLogs.FilterLogEvents",
			"log_group", logGroup,
			"request_id", requestID,
			"page", page,
		}
		if nextToken != nil {
			filterArgs = append(filterArgs, "next_token", aws.ToString(nextToken))
		}
		filterArgs = append(filterArgs, logger.GetDeadlineInfo(ctx)...)
		log.Debug("calling external service", "context", logger.SliceToMap(filterArgs))

		input := &cloudwatchlogs.FilterLogEventsInput{
			LogGroupName:  aws.String(logGroup),
			FilterPattern: aws.String(filterPattern),
			NextToken:     nextToken,
			Limit:         aws.Int32(awsConstants.CloudWatchLogsEventsLimit),
			StartTime:     aws.Int64(startMillis),
			EndTime:       aws.Int64(endMillis),
		}

		out, err := o.cwlClient.FilterLogEvents(ctx, input)
		if err != nil {
			return nil, appErrors.ErrInternalError("failed to filter backend log events", err)
		}

		for _, event := range out.Events {
			logEntry := buildLogEventFromFilteredEvent(ctx, log, event)
			parseMessageTimestamp(&logEntry, logEntry.Message)
			logEntry.EventID = auth.GenerateEventID(logEntry.Timestamp, logEntry.Message)
			events = append(events, logEntry)
		}

		if out.NextToken == nil || (nextToken != nil && aws.ToString(out.NextToken) == aws.ToString(nextToken)) {
			break
		}
		nextToken = out.NextToken
	}

	return events, nil
}

func (o *ObservabilityManagerImpl) buildFilterPattern(requestID string) string {
	return fmt.Sprintf("{ $.%s = %q }", constants.RequestIDLogField, requestID)
}

func (o *ObservabilityManagerImpl) lookbackWindowMillis() (startMillis, endMillis int64) {
	now := o.now()
	endMillis = now.UnixMilli()

	lookback := awsConstants.CloudWatchLogsObservabilityLookback
	if lookback <= 0 {
		startMillis = 0
		return startMillis, endMillis
	}

	startMillis = now.Add(-lookback).UnixMilli()
	return startMillis, endMillis
}

func (o *ObservabilityManagerImpl) now() time.Time {
	if o.nowFn != nil {
		return o.nowFn()
	}
	return time.Now()
}

func (o *ObservabilityManagerImpl) sortLogEvents(logs []api.LogEvent) {
	sort.SliceStable(logs, func(i, j int) bool {
		if logs[i].Timestamp == logs[j].Timestamp {
			return logs[i].EventID < logs[j].EventID
		}
		return logs[i].Timestamp < logs[j].Timestamp
	})
}
