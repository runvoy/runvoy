package secrets

import (
	"context"

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
	return a.client.PutParameter(ctx, params, optFns...)
}

// AddTagsToResource wraps the AWS SDK AddTagsToResource operation.
func (a *ClientAdapter) AddTagsToResource(
	ctx context.Context,
	params *ssm.AddTagsToResourceInput,
	optFns ...func(*ssm.Options),
) (*ssm.AddTagsToResourceOutput, error) {
	return a.client.AddTagsToResource(ctx, params, optFns...)
}

// GetParameter wraps the AWS SDK GetParameter operation.
func (a *ClientAdapter) GetParameter(
	ctx context.Context,
	params *ssm.GetParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.GetParameterOutput, error) {
	return a.client.GetParameter(ctx, params, optFns...)
}

// DeleteParameter wraps the AWS SDK DeleteParameter operation.
func (a *ClientAdapter) DeleteParameter(
	ctx context.Context,
	params *ssm.DeleteParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.DeleteParameterOutput, error) {
	return a.client.DeleteParameter(ctx, params, optFns...)
}
