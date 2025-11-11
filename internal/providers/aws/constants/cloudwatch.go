// Package constants provides AWS-specific constants for CloudWatch logging.
package constants

// CloudWatchLogsDescribeLimit is the limit for CloudWatch Logs DescribeLogStreams API
const CloudWatchLogsDescribeLimit = int32(50)

// CloudWatchLogsEventsLimit is the limit for CloudWatch Logs GetLogEvents API
const CloudWatchLogsEventsLimit = int32(10000)
