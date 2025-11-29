package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
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
	cwlTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

const (
	cloudWatchTimestampField = "@timestamp"
	cloudWatchMessageField   = "@message"
)

// ObservabilityManagerImpl implements the ObservabilityManager interface for AWS CloudWatch Logs Insights.
// It handles retrieving backend infrastructure logs for debugging and tracing.
type ObservabilityManagerImpl struct {
	cwlClient awsClient.CloudWatchLogsClient
	logger    *slog.Logger

	// Test-only: configurable delays for testing
	testQueryInitialDelay time.Duration
	testQueryPollInterval time.Duration
	testQueryMaxAttempts  int
}

// NewObservabilityManager creates a new AWS observability manager.
func NewObservabilityManager(
	cwlClient awsClient.CloudWatchLogsClient,
	log *slog.Logger,
) *ObservabilityManagerImpl {
	return &ObservabilityManagerImpl{
		cwlClient: cwlClient,
		logger:    log,
	}
}

// FetchBackendLogs retrieves backend infrastructure logs using CloudWatch Logs Insights
// Queries logs from Lambda execution for debugging and tracing
func (o *ObservabilityManagerImpl) FetchBackendLogs(ctx context.Context, requestID string) ([]api.LogEvent, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, o.logger)

	queryID, err := o.startBackendLogsQuery(ctx, reqLogger, requestID)
	if err != nil {
		return nil, err
	}

	// We give some headroom for CloudWatch Logs Insights query to be ready
	// This is a workaround for the fact that the query is not immediately ready
	// and we need to wait for it to be ready before we can get the results
	time.Sleep(o.getQueryInitialDelay())

	queryOutput, err := o.pollBackendLogsQuery(ctx, reqLogger, queryID)
	if err != nil {
		return nil, err
	}

	logs := o.transformBackendLogsResults(queryOutput.Results)

	return logs, nil
}

// getQueryInitialDelay returns the configured initial delay or the default
func (o *ObservabilityManagerImpl) getQueryInitialDelay() time.Duration {
	if o.testQueryInitialDelay > 0 {
		return o.testQueryInitialDelay
	}
	return awsConstants.CloudWatchLogsQueryInitialDelay
}

// getQueryPollInterval returns the configured poll interval or the default
func (o *ObservabilityManagerImpl) getQueryPollInterval() time.Duration {
	if o.testQueryPollInterval > 0 {
		return o.testQueryPollInterval
	}
	return awsConstants.CloudWatchLogsQueryPollInterval
}

// getQueryMaxAttempts returns the configured max attempts or the default
func (o *ObservabilityManagerImpl) getQueryMaxAttempts() int {
	if o.testQueryMaxAttempts > 0 {
		return o.testQueryMaxAttempts
	}
	return awsConstants.CloudWatchLogsQueryMaxAttempts
}

// startBackendLogsQuery starts a CloudWatch Logs Insights query across all runvoy Lambda logs
// Searches for all log entries matching the request ID and returns the query ID or an error if the query fails.
func (o *ObservabilityManagerImpl) startBackendLogsQuery(
	ctx context.Context,
	log *slog.Logger,
	requestID string,
) (string, error) {
	logGroups, err := o.discoverLogGroups(ctx, log)
	if err != nil {
		return "", err
	}

	if len(logGroups) == 0 {
		return "", appErrors.ErrServiceUnavailable("no Lambda log groups found matching prefix", nil)
	}

	queryString := fmt.Sprintf(`fields @timestamp, @message
| filter %s = "%s"
| sort @timestamp asc`, constants.RequestIDLogField, requestID)

	startQueryArgs := []any{
		"operation", "CloudWatchLogs.StartQuery",
		"log_groups", logGroups,
		"request_id", requestID,
	}
	startQueryArgs = append(startQueryArgs, logger.GetDeadlineInfo(ctx)...)
	log.Debug("starting CloudWatch Logs Insights query", "context", logger.SliceToMap(startQueryArgs))

	startOutput, err := o.cwlClient.StartQuery(ctx, &cloudwatchlogs.StartQueryInput{
		LogGroupNames: logGroups,
		QueryString:   aws.String(queryString),
		StartTime:     aws.Int64(0),
		EndTime:       aws.Int64(time.Now().Unix()),
	})
	if err != nil {
		return "", appErrors.ErrInternalError("failed to start CloudWatch Logs Insights query", err)
	}

	queryID := aws.ToString(startOutput.QueryId)
	log.Debug("CloudWatch Logs Insights query started", "context", map[string]any{
		"query_id":   queryID,
		"log_groups": logGroups,
	})
	return queryID, nil
}

// discoverLogGroups discovers all log groups matching the runvoy Lambda prefix
func (o *ObservabilityManagerImpl) discoverLogGroups(ctx context.Context, _ *slog.Logger) ([]string, error) {
	logGroups := []string{}
	var nextToken *string

	for {
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

// pollBackendLogsQuery polls for CloudWatch Logs Insights query results
func (o *ObservabilityManagerImpl) pollBackendLogsQuery(
	ctx context.Context,
	log *slog.Logger,
	queryID string,
) (*cloudwatchlogs.GetQueryResultsOutput, error) {
	var queryOutput *cloudwatchlogs.GetQueryResultsOutput
	maxAttempts := o.getQueryMaxAttempts()
	pollInterval := o.getQueryPollInterval()
	for i := range maxAttempts {
		if i > 0 {
			time.Sleep(pollInterval)
		}

		getResultsArgs := []any{
			"operation", "CloudWatchLogs.GetQueryResults",
			"query_id", queryID,
			"attempt", i + 1,
		}
		getResultsArgs = append(getResultsArgs, logger.GetDeadlineInfo(ctx)...)
		log.Debug("polling for query results", "context", logger.SliceToMap(getResultsArgs))

		var err error
		queryOutput, err = o.cwlClient.GetQueryResults(ctx, &cloudwatchlogs.GetQueryResultsInput{
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

	log.Debug("CloudWatch Logs Insights query completed",
		"query_id", queryID,
		"result_count", len(queryOutput.Results))

	return queryOutput, nil
}

// transformBackendLogsResults transforms CloudWatch Logs Insights results to LogEvent format.
// Attempts to extract timestamps from JSON-formatted log messages first, then falls back
// to CloudWatch's @timestamp field if message parsing fails.
func (o *ObservabilityManagerImpl) transformBackendLogsResults(
	results [][]cwlTypes.ResultField,
) []api.LogEvent {
	logs := make([]api.LogEvent, 0, len(results))
	for _, result := range results {
		logEntry := api.LogEvent{}
		var cloudwatchTimestamp string

		for _, field := range result {
			fieldName := aws.ToString(field.Field)
			fieldValue := aws.ToString(field.Value)

			switch fieldName {
			case cloudWatchTimestampField:
				cloudwatchTimestamp = fieldValue
			case cloudWatchMessageField:
				logEntry.Message = fieldValue
			}
		}

		// Try to parse timestamp from message JSON first (preferred)
		if !parseMessageTimestamp(&logEntry, logEntry.Message) {
			// Fall back to CloudWatch @timestamp if message parsing fails
			parseCloudWatchTimestamp(&logEntry, cloudwatchTimestamp)
		}

		// Generate eventID (deterministic based on timestamp + message)
		// If timestamp is 0 or negative (parsing failed), it still produces a deterministic ID
		logEntry.EventID = auth.GenerateEventID(logEntry.Timestamp, logEntry.Message)

		logs = append(logs, logEntry)
	}
	return logs
}
