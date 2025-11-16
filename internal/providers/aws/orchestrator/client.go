package orchestrator

import (
	"context"

	awsClient "runvoy/internal/providers/aws/client"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

// Client is a type alias for the shared ECS client interface.
// This maintains backward compatibility within the orchestrator package.
type Client = awsClient.ECSClient

// ClientAdapter is a type alias for the shared ECS client adapter.
// This maintains backward compatibility within the orchestrator package.
type ClientAdapter = awsClient.ECSClientAdapter

// NewClientAdapter creates a new ECS client adapter.
// This is a convenience wrapper around the shared adapter constructor.
func NewClientAdapter(client *ecs.Client) *ClientAdapter {
	return awsClient.NewECSClientAdapter(client)
}

// CloudWatchLogsClient defines the interface for CloudWatch Logs operations used by the runner.
// This interface makes the code easier to test by allowing mock implementations.
type CloudWatchLogsClient interface {
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

// IAMClient is a type alias for the shared IAM client interface.
// This maintains backward compatibility within the orchestrator package.
type IAMClient = awsClient.IAMClient

// IAMClientAdapter is a type alias for the shared IAM client adapter.
// This maintains backward compatibility within the orchestrator package.
type IAMClientAdapter = awsClient.IAMClientAdapter

// NewIAMClientAdapter creates a new IAM client adapter.
// This is a convenience wrapper around the shared adapter constructor.
func NewIAMClientAdapter(client *iam.Client) *IAMClientAdapter {
	return awsClient.NewIAMClientAdapter(client)
}
