package client

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"
)

// IAMClient defines the interface for IAM operations used across AWS provider packages.
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
	result, err := a.client.GetRole(ctx, params, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}
	return result, nil
}
