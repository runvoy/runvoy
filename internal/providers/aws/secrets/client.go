package secrets

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// Client defines the interface for SSM operations used by the ParameterStoreManager.
// This interface makes the code easier to test by allowing mock implementations.
type Client interface {
	PutParameter(
		ctx context.Context,
		params *ssm.PutParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.PutParameterOutput, error)
	AddTagsToResource(
		ctx context.Context,
		params *ssm.AddTagsToResourceInput,
		optFns ...func(*ssm.Options),
	) (*ssm.AddTagsToResourceOutput, error)
	GetParameter(
		ctx context.Context,
		params *ssm.GetParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.GetParameterOutput, error)
	DeleteParameter(
		ctx context.Context,
		params *ssm.DeleteParameterInput,
		optFns ...func(*ssm.Options),
	) (*ssm.DeleteParameterOutput, error)
	ListTagsForResource(
		ctx context.Context,
		params *ssm.ListTagsForResourceInput,
		optFns ...func(*ssm.Options),
	) (*ssm.ListTagsForResourceOutput, error)
	DescribeParameters(
		ctx context.Context,
		params *ssm.DescribeParametersInput,
		optFns ...func(*ssm.Options),
	) (*ssm.DescribeParametersOutput, error)
}

// ClientAdapter wraps the AWS SDK SSM client to implement Client interface.
// This allows us to use the real AWS client in production while maintaining testability.
type ClientAdapter struct {
	client *ssm.Client
}

// NewClientAdapter creates a new adapter wrapping the AWS SDK SSM client.
func NewClientAdapter(client *ssm.Client) *ClientAdapter {
	return &ClientAdapter{client: client}
}

// PutParameter wraps the AWS SDK PutParameter operation.
func (a *ClientAdapter) PutParameter(
	ctx context.Context,
	params *ssm.PutParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.PutParameterOutput, error) {
	result, err := a.client.PutParameter(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to put parameter: %w", err)
	}
	return result, nil
}

// AddTagsToResource wraps the AWS SDK AddTagsToResource operation.
func (a *ClientAdapter) AddTagsToResource(
	ctx context.Context,
	params *ssm.AddTagsToResourceInput,
	optFns ...func(*ssm.Options),
) (*ssm.AddTagsToResourceOutput, error) {
	result, err := a.client.AddTagsToResource(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to add tags to resource: %w", err)
	}
	return result, nil
}

// GetParameter wraps the AWS SDK GetParameter operation.
func (a *ClientAdapter) GetParameter(
	ctx context.Context,
	params *ssm.GetParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.GetParameterOutput, error) {
	result, err := a.client.GetParameter(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to get parameter: %w", err)
	}
	return result, nil
}

// DeleteParameter wraps the AWS SDK DeleteParameter operation.
func (a *ClientAdapter) DeleteParameter(
	ctx context.Context,
	params *ssm.DeleteParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.DeleteParameterOutput, error) {
	result, err := a.client.DeleteParameter(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to delete parameter: %w", err)
	}
	return result, nil
}

// ListTagsForResource wraps the AWS SDK ListTagsForResource operation.
func (a *ClientAdapter) ListTagsForResource(
	ctx context.Context,
	params *ssm.ListTagsForResourceInput,
	optFns ...func(*ssm.Options),
) (*ssm.ListTagsForResourceOutput, error) {
	result, err := a.client.ListTagsForResource(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags for resource: %w", err)
	}
	return result, nil
}

// DescribeParameters wraps the AWS SDK DescribeParameters operation.
func (a *ClientAdapter) DescribeParameters(
	ctx context.Context,
	params *ssm.DescribeParametersInput,
	optFns ...func(*ssm.Options),
) (*ssm.DescribeParametersOutput, error) {
	result, err := a.client.DescribeParameters(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to describe parameters: %w", err)
	}
	return result, nil
}
