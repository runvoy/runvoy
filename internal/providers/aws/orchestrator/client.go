package orchestrator

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

// Client defines the interface for ECS operations used by the runner and task definition functions.
// This interface makes the code easier to test by allowing mock implementations.
type Client interface {
	RunTask(
		ctx context.Context,
		params *ecs.RunTaskInput,
		optFns ...func(*ecs.Options),
	) (*ecs.RunTaskOutput, error)
	TagResource(
		ctx context.Context,
		params *ecs.TagResourceInput,
		optFns ...func(*ecs.Options),
	) (*ecs.TagResourceOutput, error)
	ListTasks(
		ctx context.Context,
		params *ecs.ListTasksInput,
		optFns ...func(*ecs.Options),
	) (*ecs.ListTasksOutput, error)
	DescribeTasks(
		ctx context.Context,
		params *ecs.DescribeTasksInput,
		optFns ...func(*ecs.Options),
	) (*ecs.DescribeTasksOutput, error)
	StopTask(
		ctx context.Context,
		params *ecs.StopTaskInput,
		optFns ...func(*ecs.Options),
	) (*ecs.StopTaskOutput, error)
	DescribeTaskDefinition(
		ctx context.Context,
		params *ecs.DescribeTaskDefinitionInput,
		optFns ...func(*ecs.Options),
	) (*ecs.DescribeTaskDefinitionOutput, error)
	ListTagsForResource(
		ctx context.Context,
		params *ecs.ListTagsForResourceInput,
		optFns ...func(*ecs.Options),
	) (*ecs.ListTagsForResourceOutput, error)
	ListTaskDefinitions(
		ctx context.Context,
		params *ecs.ListTaskDefinitionsInput,
		optFns ...func(*ecs.Options),
	) (*ecs.ListTaskDefinitionsOutput, error)
	RegisterTaskDefinition(
		ctx context.Context,
		params *ecs.RegisterTaskDefinitionInput,
		optFns ...func(*ecs.Options),
	) (*ecs.RegisterTaskDefinitionOutput, error)
	DeregisterTaskDefinition(
		ctx context.Context,
		params *ecs.DeregisterTaskDefinitionInput,
		optFns ...func(*ecs.Options),
	) (*ecs.DeregisterTaskDefinitionOutput, error)
	DeleteTaskDefinitions(
		ctx context.Context,
		params *ecs.DeleteTaskDefinitionsInput,
		optFns ...func(*ecs.Options),
	) (*ecs.DeleteTaskDefinitionsOutput, error)
	UntagResource(
		ctx context.Context,
		params *ecs.UntagResourceInput,
		optFns ...func(*ecs.Options),
	) (*ecs.UntagResourceOutput, error)
}

// ClientAdapter wraps the AWS SDK ECS client to implement Client interface.
// This allows us to use the real AWS client in production while maintaining testability.
type ClientAdapter struct {
	client *ecs.Client
}

// NewClientAdapter creates a new adapter wrapping the AWS SDK ECS client.
func NewClientAdapter(client *ecs.Client) *ClientAdapter {
	return &ClientAdapter{client: client}
}

// RunTask wraps the AWS SDK RunTask operation.
func (a *ClientAdapter) RunTask(
	ctx context.Context,
	params *ecs.RunTaskInput,
	optFns ...func(*ecs.Options),
) (*ecs.RunTaskOutput, error) {
	return a.client.RunTask(ctx, params, optFns...)
}

// TagResource wraps the AWS SDK TagResource operation.
func (a *ClientAdapter) TagResource(
	ctx context.Context,
	params *ecs.TagResourceInput,
	optFns ...func(*ecs.Options),
) (*ecs.TagResourceOutput, error) {
	return a.client.TagResource(ctx, params, optFns...)
}

// ListTasks wraps the AWS SDK ListTasks operation.
func (a *ClientAdapter) ListTasks(
	ctx context.Context,
	params *ecs.ListTasksInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTasksOutput, error) {
	return a.client.ListTasks(ctx, params, optFns...)
}

// DescribeTasks wraps the AWS SDK DescribeTasks operation.
func (a *ClientAdapter) DescribeTasks(
	ctx context.Context,
	params *ecs.DescribeTasksInput,
	optFns ...func(*ecs.Options),
) (*ecs.DescribeTasksOutput, error) {
	return a.client.DescribeTasks(ctx, params, optFns...)
}

// StopTask wraps the AWS SDK StopTask operation.
func (a *ClientAdapter) StopTask(
	ctx context.Context,
	params *ecs.StopTaskInput,
	optFns ...func(*ecs.Options),
) (*ecs.StopTaskOutput, error) {
	return a.client.StopTask(ctx, params, optFns...)
}

// DescribeTaskDefinition wraps the AWS SDK DescribeTaskDefinition operation.
func (a *ClientAdapter) DescribeTaskDefinition(
	ctx context.Context,
	params *ecs.DescribeTaskDefinitionInput,
	optFns ...func(*ecs.Options),
) (*ecs.DescribeTaskDefinitionOutput, error) {
	return a.client.DescribeTaskDefinition(ctx, params, optFns...)
}

// ListTagsForResource wraps the AWS SDK ListTagsForResource operation.
func (a *ClientAdapter) ListTagsForResource(
	ctx context.Context,
	params *ecs.ListTagsForResourceInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTagsForResourceOutput, error) {
	return a.client.ListTagsForResource(ctx, params, optFns...)
}

// ListTaskDefinitions wraps the AWS SDK ListTaskDefinitions operation.
func (a *ClientAdapter) ListTaskDefinitions(
	ctx context.Context,
	params *ecs.ListTaskDefinitionsInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTaskDefinitionsOutput, error) {
	return a.client.ListTaskDefinitions(ctx, params, optFns...)
}

// RegisterTaskDefinition wraps the AWS SDK RegisterTaskDefinition operation.
func (a *ClientAdapter) RegisterTaskDefinition(
	ctx context.Context,
	params *ecs.RegisterTaskDefinitionInput,
	optFns ...func(*ecs.Options),
) (*ecs.RegisterTaskDefinitionOutput, error) {
	return a.client.RegisterTaskDefinition(ctx, params, optFns...)
}

// DeregisterTaskDefinition wraps the AWS SDK DeregisterTaskDefinition operation.
func (a *ClientAdapter) DeregisterTaskDefinition(
	ctx context.Context,
	params *ecs.DeregisterTaskDefinitionInput,
	optFns ...func(*ecs.Options),
) (*ecs.DeregisterTaskDefinitionOutput, error) {
	return a.client.DeregisterTaskDefinition(ctx, params, optFns...)
}

// DeleteTaskDefinitions wraps the AWS SDK DeleteTaskDefinitions operation.
func (a *ClientAdapter) DeleteTaskDefinitions(
	ctx context.Context,
	params *ecs.DeleteTaskDefinitionsInput,
	optFns ...func(*ecs.Options),
) (*ecs.DeleteTaskDefinitionsOutput, error) {
	return a.client.DeleteTaskDefinitions(ctx, params, optFns...)
}

// UntagResource wraps the AWS SDK UntagResource operation.
func (a *ClientAdapter) UntagResource(
	ctx context.Context,
	params *ecs.UntagResourceInput,
	optFns ...func(*ecs.Options),
) (*ecs.UntagResourceOutput, error) {
	return a.client.UntagResource(ctx, params, optFns...)
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

// IAMClient defines the interface for IAM operations used by the runner.
// This interface makes the code easier to test by allowing mock implementations.
type IAMClient interface {
	GetRole(
		ctx context.Context,
		params *iam.GetRoleInput,
		optFns ...func(*iam.Options),
	) (*iam.GetRoleOutput, error)
}

// IAMClientAdapter wraps the AWS SDK IAM client to implement IAMClient interface.
// This allows us to use the real AWS client in production while maintaining testability.
type IAMClientAdapter struct {
	client *iam.Client
}

// NewIAMClientAdapter creates a new adapter wrapping the AWS SDK IAM client.
func NewIAMClientAdapter(client *iam.Client) *IAMClientAdapter {
	return &IAMClientAdapter{client: client}
}

// GetRole wraps the AWS SDK GetRole operation.
func (a *IAMClientAdapter) GetRole(
	ctx context.Context,
	params *iam.GetRoleInput,
	optFns ...func(*iam.Options),
) (*iam.GetRoleOutput, error) {
	return a.client.GetRole(ctx, params, optFns...)
}
