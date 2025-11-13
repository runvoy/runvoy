package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
)

// Client defines the interface for API Gateway Management API operations used by the Manager.
// This interface makes the code easier to test by allowing mock implementations.
type Client interface {
	PostToConnection(
		ctx context.Context,
		params *apigatewaymanagementapi.PostToConnectionInput,
		optFns ...func(*apigatewaymanagementapi.Options),
	) (*apigatewaymanagementapi.PostToConnectionOutput, error)
}

// ClientAdapter wraps the AWS SDK API Gateway Management API client to implement Client interface.
// This allows us to use the real AWS client in production while maintaining testability.
type ClientAdapter struct {
	client *apigatewaymanagementapi.Client
}

// NewClientAdapter creates a new adapter wrapping the AWS SDK API Gateway Management API client.
func NewClientAdapter(client *apigatewaymanagementapi.Client) *ClientAdapter {
	return &ClientAdapter{client: client}
}

// PostToConnection wraps the AWS SDK PostToConnection operation.
func (a *ClientAdapter) PostToConnection(
	ctx context.Context,
	params *apigatewaymanagementapi.PostToConnectionInput,
	optFns ...func(*apigatewaymanagementapi.Options),
) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
	return a.client.PostToConnection(ctx, params, optFns...)
}
