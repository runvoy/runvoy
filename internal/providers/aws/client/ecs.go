package client

import (
	"context"
	"fmt"

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
	result, err := a.client.RunTask(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to run task: %w", err)
	}
	return result, nil
}

// TagResource wraps the AWS SDK TagResource operation.
func (a *ECSClientAdapter) TagResource(
	ctx context.Context,
	params *ecs.TagResourceInput,
	optFns ...func(*ecs.Options),
) (*ecs.TagResourceOutput, error) {
	result, err := a.client.TagResource(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to tag resource: %w", err)
	}
	return result, nil
}

// ListTasks wraps the AWS SDK ListTasks operation.
func (a *ECSClientAdapter) ListTasks(
	ctx context.Context,
	params *ecs.ListTasksInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTasksOutput, error) {
	result, err := a.client.ListTasks(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	return result, nil
}

// DescribeTasks wraps the AWS SDK DescribeTasks operation.
func (a *ECSClientAdapter) DescribeTasks(
	ctx context.Context,
	params *ecs.DescribeTasksInput,
	optFns ...func(*ecs.Options),
) (*ecs.DescribeTasksOutput, error) {
	result, err := a.client.DescribeTasks(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to describe tasks: %w", err)
	}
	return result, nil
}

// StopTask wraps the AWS SDK StopTask operation.
func (a *ECSClientAdapter) StopTask(
	ctx context.Context,
	params *ecs.StopTaskInput,
	optFns ...func(*ecs.Options),
) (*ecs.StopTaskOutput, error) {
	result, err := a.client.StopTask(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to stop task: %w", err)
	}
	return result, nil
}

// DescribeTaskDefinition wraps the AWS SDK DescribeTaskDefinition operation.
func (a *ECSClientAdapter) DescribeTaskDefinition(
	ctx context.Context,
	params *ecs.DescribeTaskDefinitionInput,
	optFns ...func(*ecs.Options),
) (*ecs.DescribeTaskDefinitionOutput, error) {
	result, err := a.client.DescribeTaskDefinition(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to describe task definition: %w", err)
	}
	return result, nil
}

// ListTagsForResource wraps the AWS SDK ListTagsForResource operation.
func (a *ECSClientAdapter) ListTagsForResource(
	ctx context.Context,
	params *ecs.ListTagsForResourceInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTagsForResourceOutput, error) {
	result, err := a.client.ListTagsForResource(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags for resource: %w", err)
	}
	return result, nil
}

// ListTaskDefinitions wraps the AWS SDK ListTaskDefinitions operation.
func (a *ECSClientAdapter) ListTaskDefinitions(
	ctx context.Context,
	params *ecs.ListTaskDefinitionsInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTaskDefinitionsOutput, error) {
	result, err := a.client.ListTaskDefinitions(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to list task definitions: %w", err)
	}
	return result, nil
}

// RegisterTaskDefinition wraps the AWS SDK RegisterTaskDefinition operation.
func (a *ECSClientAdapter) RegisterTaskDefinition(
	ctx context.Context,
	params *ecs.RegisterTaskDefinitionInput,
	optFns ...func(*ecs.Options),
) (*ecs.RegisterTaskDefinitionOutput, error) {
	result, err := a.client.RegisterTaskDefinition(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to register task definition: %w", err)
	}
	return result, nil
}

// DeregisterTaskDefinition wraps the AWS SDK DeregisterTaskDefinition operation.
func (a *ECSClientAdapter) DeregisterTaskDefinition(
	ctx context.Context,
	params *ecs.DeregisterTaskDefinitionInput,
	optFns ...func(*ecs.Options),
) (*ecs.DeregisterTaskDefinitionOutput, error) {
	result, err := a.client.DeregisterTaskDefinition(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to deregister task definition: %w", err)
	}
	return result, nil
}

// DeleteTaskDefinitions wraps the AWS SDK DeleteTaskDefinitions operation.
func (a *ECSClientAdapter) DeleteTaskDefinitions(
	ctx context.Context,
	params *ecs.DeleteTaskDefinitionsInput,
	optFns ...func(*ecs.Options),
) (*ecs.DeleteTaskDefinitionsOutput, error) {
	result, err := a.client.DeleteTaskDefinitions(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to delete task definitions: %w", err)
	}
	return result, nil
}

// UntagResource wraps the AWS SDK UntagResource operation.
func (a *ECSClientAdapter) UntagResource(
	ctx context.Context,
	params *ecs.UntagResourceInput,
	optFns ...func(*ecs.Options),
) (*ecs.UntagResourceOutput, error) {
	result, err := a.client.UntagResource(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to untag resource: %w", err)
	}
	return result, nil
}
