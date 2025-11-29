package client

import (
	"context"
	"fmt"

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
	FilterLogEvents(
		ctx context.Context,
		params *cloudwatchlogs.FilterLogEventsInput,
		optFns ...func(*cloudwatchlogs.Options),
	) (*cloudwatchlogs.FilterLogEventsOutput, error)
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
	result, err := a.client.DescribeLogGroups(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to describe log groups: %w", err)
	}
	return result, nil
}

// DescribeLogStreams wraps the AWS SDK DescribeLogStreams operation.
func (a *CloudWatchLogsClientAdapter) DescribeLogStreams(
	ctx context.Context,
	params *cloudwatchlogs.DescribeLogStreamsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	result, err := a.client.DescribeLogStreams(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to describe log streams: %w", err)
	}
	return result, nil
}

// FilterLogEvents wraps the AWS SDK FilterLogEvents operation.
func (a *CloudWatchLogsClientAdapter) FilterLogEvents(
	ctx context.Context,
	params *cloudwatchlogs.FilterLogEventsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	result, err := a.client.FilterLogEvents(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to filter log events: %w", err)
	}
	return result, nil
}

// StartQuery wraps the AWS SDK StartQuery operation for CloudWatch Logs Insights.
func (a *CloudWatchLogsClientAdapter) StartQuery(
	ctx context.Context,
	params *cloudwatchlogs.StartQueryInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.StartQueryOutput, error) {
	result, err := a.client.StartQuery(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to start query: %w", err)
	}
	return result, nil
}

// GetQueryResults wraps the AWS SDK GetQueryResults operation for CloudWatch Logs Insights.
func (a *CloudWatchLogsClientAdapter) GetQueryResults(
	ctx context.Context,
	params *cloudwatchlogs.GetQueryResultsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.GetQueryResultsOutput, error) {
	result, err := a.client.GetQueryResults(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to get query results: %w", err)
	}
	return result, nil
}
