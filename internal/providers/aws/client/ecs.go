package client

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// ECSClient defines the interface for ECS operations used across AWS provider packages.
// This interface makes the code easier to test by allowing mock implementations.
type ECSClient interface {
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

// ECSClientAdapter wraps the AWS SDK ECS client to implement ECSClient interface.
// This allows us to use the real AWS client in production while maintaining testability.
type ECSClientAdapter struct {
	client *ecs.Client
}

// NewECSClientAdapter creates a new adapter wrapping the AWS SDK ECS client.
func NewECSClientAdapter(client *ecs.Client) *ECSClientAdapter {
	return &ECSClientAdapter{client: client}
}

// RunTask wraps the AWS SDK RunTask operation.
func (a *ECSClientAdapter) RunTask(
	ctx context.Context,
	params *ecs.RunTaskInput,
	optFns ...func(*ecs.Options),
) (*ecs.RunTaskOutput, error) {
	return a.client.RunTask(ctx, params, optFns...)
}

// TagResource wraps the AWS SDK TagResource operation.
func (a *ECSClientAdapter) TagResource(
	ctx context.Context,
	params *ecs.TagResourceInput,
	optFns ...func(*ecs.Options),
) (*ecs.TagResourceOutput, error) {
	return a.client.TagResource(ctx, params, optFns...)
}

// ListTasks wraps the AWS SDK ListTasks operation.
func (a *ECSClientAdapter) ListTasks(
	ctx context.Context,
	params *ecs.ListTasksInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTasksOutput, error) {
	return a.client.ListTasks(ctx, params, optFns...)
}

// DescribeTasks wraps the AWS SDK DescribeTasks operation.
func (a *ECSClientAdapter) DescribeTasks(
	ctx context.Context,
	params *ecs.DescribeTasksInput,
	optFns ...func(*ecs.Options),
) (*ecs.DescribeTasksOutput, error) {
	return a.client.DescribeTasks(ctx, params, optFns...)
}

// StopTask wraps the AWS SDK StopTask operation.
func (a *ECSClientAdapter) StopTask(
	ctx context.Context,
	params *ecs.StopTaskInput,
	optFns ...func(*ecs.Options),
) (*ecs.StopTaskOutput, error) {
	return a.client.StopTask(ctx, params, optFns...)
}

// DescribeTaskDefinition wraps the AWS SDK DescribeTaskDefinition operation.
func (a *ECSClientAdapter) DescribeTaskDefinition(
	ctx context.Context,
	params *ecs.DescribeTaskDefinitionInput,
	optFns ...func(*ecs.Options),
) (*ecs.DescribeTaskDefinitionOutput, error) {
	return a.client.DescribeTaskDefinition(ctx, params, optFns...)
}

// ListTagsForResource wraps the AWS SDK ListTagsForResource operation.
func (a *ECSClientAdapter) ListTagsForResource(
	ctx context.Context,
	params *ecs.ListTagsForResourceInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTagsForResourceOutput, error) {
	return a.client.ListTagsForResource(ctx, params, optFns...)
}

// ListTaskDefinitions wraps the AWS SDK ListTaskDefinitions operation.
func (a *ECSClientAdapter) ListTaskDefinitions(
	ctx context.Context,
	params *ecs.ListTaskDefinitionsInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTaskDefinitionsOutput, error) {
	return a.client.ListTaskDefinitions(ctx, params, optFns...)
}

// RegisterTaskDefinition wraps the AWS SDK RegisterTaskDefinition operation.
func (a *ECSClientAdapter) RegisterTaskDefinition(
	ctx context.Context,
	params *ecs.RegisterTaskDefinitionInput,
	optFns ...func(*ecs.Options),
) (*ecs.RegisterTaskDefinitionOutput, error) {
	return a.client.RegisterTaskDefinition(ctx, params, optFns...)
}

// DeregisterTaskDefinition wraps the AWS SDK DeregisterTaskDefinition operation.
func (a *ECSClientAdapter) DeregisterTaskDefinition(
	ctx context.Context,
	params *ecs.DeregisterTaskDefinitionInput,
	optFns ...func(*ecs.Options),
) (*ecs.DeregisterTaskDefinitionOutput, error) {
	return a.client.DeregisterTaskDefinition(ctx, params, optFns...)
}

// DeleteTaskDefinitions wraps the AWS SDK DeleteTaskDefinitions operation.
func (a *ECSClientAdapter) DeleteTaskDefinitions(
	ctx context.Context,
	params *ecs.DeleteTaskDefinitionsInput,
	optFns ...func(*ecs.Options),
) (*ecs.DeleteTaskDefinitionsOutput, error) {
	return a.client.DeleteTaskDefinitions(ctx, params, optFns...)
}

// UntagResource wraps the AWS SDK UntagResource operation.
func (a *ECSClientAdapter) UntagResource(
	ctx context.Context,
	params *ecs.UntagResourceInput,
	optFns ...func(*ecs.Options),
) (*ecs.UntagResourceOutput, error) {
	return a.client.UntagResource(ctx, params, optFns...)
}
