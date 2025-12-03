package constants

import "time"

// CloudWatchLogsDescribeLimit is the limit for CloudWatch Logs DescribeLogStreams API.
const CloudWatchLogsDescribeLimit = int32(50)

// CloudWatchLogsEventsLimit is the limit for CloudWatch Logs FilterLogEvents API.
const CloudWatchLogsEventsLimit = int32(10000)

// CloudWatchLogsObservabilityLookback defines how far back (in time) we search
// when querying backend infrastructure logs for a specific request.
// Set to 0 to fetch all historical logs without a time bound.
const CloudWatchLogsObservabilityLookback time.Duration = 0

// ScheduledEventHealthReconcile is the expected runvoy_event payload value
// for EventBridge scheduled events that trigger health reconciliation.
const ScheduledEventHealthReconcile = "health_reconcile"
