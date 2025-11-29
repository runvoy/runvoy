package constants

import (
	"time"

	"github.com/runvoy/runvoy/internal/constants"
)

// CloudWatchLogsDescribeLimit is the limit for CloudWatch Logs DescribeLogStreams API.
const CloudWatchLogsDescribeLimit = int32(50)

// CloudWatchLogsEventsLimit is the limit for CloudWatch Logs FilterLogEvents API.
const CloudWatchLogsEventsLimit = int32(10000)

// LogGroupPrefix is the prefix for all runvoy Lambda log groups.
const LogGroupPrefix = "/aws/lambda/" + constants.ProjectName

// CloudWatchLogsQueryMaxAttempts is the maximum number of polling attempts
// for CloudWatch Logs Insights query results.
const CloudWatchLogsQueryMaxAttempts = 30

// CloudWatchLogsQueryPollInterval is the polling interval in milliseconds
// for checking CloudWatch Logs Insights query results.
const CloudWatchLogsQueryPollInterval = time.Second

// CloudWatchLogsQueryInitialDelay is the initial delay in seconds
// to allow CloudWatch Logs Insights query to become ready before polling.
const CloudWatchLogsQueryInitialDelay = 10 * time.Second

// ScheduledEventHealthReconcile is the expected runvoy_event payload value
// for EventBridge scheduled events that trigger health reconciliation.
const ScheduledEventHealthReconcile = "health_reconcile"
