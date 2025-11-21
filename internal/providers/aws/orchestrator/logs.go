package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"runvoy/internal/api"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/logger"
	awsClient "runvoy/internal/providers/aws/client"
	awsConstants "runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// verifyLogStreamExists checks if the log stream exists and returns an error if it doesn't
func verifyLogStreamExists(
	ctx context.Context,
	cwl awsClient.CloudWatchLogsClient,
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
	cwl awsClient.CloudWatchLogsClient, logGroup string, stream string) ([]api.LogEvent, error) {
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

// FetchBackendLogs retrieves backend infrastructure logs using CloudWatch Logs Insights
// Queries logs from Lambda execution and API Gateway for debugging and tracing
//
//nolint:funlen // Complex AWS Logs Insights query with multiple steps is inherently longer
func (r *Runner) FetchBackendLogs(ctx context.Context, requestID string) (*api.BackendLogsResponse, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	// Build the CloudWatch Logs Insights query
	queryString := fmt.Sprintf(`fields @timestamp, @message, @logStream
		| filter @message like /%s/
		| sort @timestamp asc
		| limit 10000`, requestID)

	// Start the query
	startQueryArgs := []any{
		"operation", "CloudWatchLogs.StartQuery",
		"log_group", r.cfg.LogGroup,
		"request_id", requestID,
	}
	startQueryArgs = append(startQueryArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("starting CloudWatch Logs Insights query", "context", logger.SliceToMap(startQueryArgs))

	startOutput, err := r.cwlClient.StartQuery(ctx, &cloudwatchlogs.StartQueryInput{
		LogGroupName: aws.String(r.cfg.LogGroup),
		QueryString:  aws.String(queryString),
		StartTime:    aws.Int64(time.Now().Add(-24 * time.Hour).Unix()), // Last 24 hours
		EndTime:      aws.Int64(time.Now().Unix()),
	})
	if err != nil {
		return nil, appErrors.ErrInternalError("failed to start CloudWatch Logs Insights query", err)
	}

	queryID := aws.ToString(startOutput.QueryId)
	reqLogger.Info("CloudWatch Logs Insights query started", "query_id", queryID)

	// Poll for query results
	var queryOutput *cloudwatchlogs.GetQueryResultsOutput
	const (
		maxQueryAttempts    = 30
		queryPollIntervalMs = 500
	)
	for i := range maxQueryAttempts {
		time.Sleep(time.Duration(queryPollIntervalMs) * time.Millisecond)

		getResultsArgs := []any{
			"operation", "CloudWatchLogs.GetQueryResults",
			"query_id", queryID,
			"attempt", i + 1,
		}
		getResultsArgs = append(getResultsArgs, logger.GetDeadlineInfo(ctx)...)
		reqLogger.Debug("polling for query results", "context", logger.SliceToMap(getResultsArgs))

		queryOutput, err = r.cwlClient.GetQueryResults(ctx, &cloudwatchlogs.GetQueryResultsInput{
			QueryId: aws.String(queryID),
		})
		if err != nil {
			return nil, appErrors.ErrInternalError("failed to get CloudWatch Logs Insights query results", err)
		}

		if queryOutput.Status == cwlTypes.QueryStatusComplete {
			break
		}

		if queryOutput.Status == cwlTypes.QueryStatusFailed ||
			queryOutput.Status == cwlTypes.QueryStatusCancelled {
			return nil, appErrors.ErrInternalError(
				fmt.Sprintf("CloudWatch Logs Insights query failed with status: %s", queryOutput.Status), nil)
		}
	}

	if queryOutput.Status != cwlTypes.QueryStatusComplete {
		return nil, appErrors.ErrServiceUnavailable("CloudWatch Logs Insights query timed out", nil)
	}

	reqLogger.Info("CloudWatch Logs Insights query completed",
		"query_id", queryID,
		"result_count", len(queryOutput.Results))

	// Transform results to API format
	logs := make([]api.LogEvent, 0, len(queryOutput.Results))
	for _, result := range queryOutput.Results {
		logEntry := api.LogEvent{
			Fields: make(map[string]string),
		}

		for _, field := range result {
			fieldName := aws.ToString(field.Field)
			fieldValue := aws.ToString(field.Value)

			switch fieldName {
			case "@timestamp":
				// Parse timestamp
				t, parseErr := time.Parse(time.RFC3339, fieldValue)
				if parseErr == nil {
					logEntry.Timestamp = t.UnixMilli()
				}
			case "@message":
				logEntry.Message = fieldValue
			default:
				logEntry.Fields[fieldName] = fieldValue
			}
		}

		logs = append(logs, logEntry)
	}

	return &api.BackendLogsResponse{
		RequestID: requestID,
		Logs:      logs,
		Status:    string(queryOutput.Status),
	}, nil
}
