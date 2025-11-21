package client

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// CloudWatchLogsClient defines the interface for CloudWatch Logs operations used by the runner.
// This interface makes the code easier to test by allowing mock implementations.
type CloudWatchLogsClient interface {
	DescribeLogGroups(
		ctx context.Context,
		params *cloudwatchlogs.DescribeLogGroupsInput,
		optFns ...func(*cloudwatchlogs.Options),
	) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	DescribeLogStreams(
		ctx context.Context,
		params *cloudwatchlogs.DescribeLogStreamsInput,
		optFns ...func(*cloudwatchlogs.Options),
	) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
	GetLogEvents(
		ctx context.Context,
		params *cloudwatchlogs.GetLogEventsInput,
		optFns ...func(*cloudwatchlogs.Options),
	) (*cloudwatchlogs.GetLogEventsOutput, error)
	StartQuery(
		ctx context.Context,
		params *cloudwatchlogs.StartQueryInput,
		optFns ...func(*cloudwatchlogs.Options),
	) (*cloudwatchlogs.StartQueryOutput, error)
	GetQueryResults(
		ctx context.Context,
		params *cloudwatchlogs.GetQueryResultsInput,
		optFns ...func(*cloudwatchlogs.Options),
	) (*cloudwatchlogs.GetQueryResultsOutput, error)
}

// CloudWatchLogsClientAdapter wraps the AWS SDK CloudWatch Logs client to implement CloudWatchLogsClient interface.
// This allows us to use the real AWS client in production while maintaining testability.
type CloudWatchLogsClientAdapter struct {
	client *cloudwatchlogs.Client
}

// NewCloudWatchLogsClientAdapter creates a new adapter wrapping the AWS SDK CloudWatch Logs client.
func NewCloudWatchLogsClientAdapter(client *cloudwatchlogs.Client) *CloudWatchLogsClientAdapter {
	return &CloudWatchLogsClientAdapter{client: client}
}

// DescribeLogGroups wraps the AWS SDK DescribeLogGroups operation.
func (a *CloudWatchLogsClientAdapter) DescribeLogGroups(
	ctx context.Context,
	params *cloudwatchlogs.DescribeLogGroupsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return a.client.DescribeLogGroups(ctx, params, optFns...)
}

// DescribeLogStreams wraps the AWS SDK DescribeLogStreams operation.
func (a *CloudWatchLogsClientAdapter) DescribeLogStreams(
	ctx context.Context,
	params *cloudwatchlogs.DescribeLogStreamsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	return a.client.DescribeLogStreams(ctx, params, optFns...)
}

// GetLogEvents wraps the AWS SDK GetLogEvents operation.
func (a *CloudWatchLogsClientAdapter) GetLogEvents(
	ctx context.Context,
	params *cloudwatchlogs.GetLogEventsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.GetLogEventsOutput, error) {
	return a.client.GetLogEvents(ctx, params, optFns...)
}

// StartQuery wraps the AWS SDK StartQuery operation for CloudWatch Logs Insights.
func (a *CloudWatchLogsClientAdapter) StartQuery(
	ctx context.Context,
	params *cloudwatchlogs.StartQueryInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.StartQueryOutput, error) {
	return a.client.StartQuery(ctx, params, optFns...)
}

// GetQueryResults wraps the AWS SDK GetQueryResults operation for CloudWatch Logs Insights.
func (a *CloudWatchLogsClientAdapter) GetQueryResults(
	ctx context.Context,
	params *cloudwatchlogs.GetQueryResultsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.GetQueryResultsOutput, error) {
	return a.client.GetQueryResults(ctx, params, optFns...)
}
