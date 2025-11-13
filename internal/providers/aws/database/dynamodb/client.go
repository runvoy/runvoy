package dynamodb

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Client defines the interface for DynamoDB operations used by repositories.
// This interface makes repositories easier to test by allowing mock implementations.
type Client interface {
	PutItem(
		ctx context.Context,
		params *dynamodb.PutItemInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.PutItemOutput, error)
	GetItem(
		ctx context.Context,
		params *dynamodb.GetItemInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.GetItemOutput, error)
	Query(
		ctx context.Context,
		params *dynamodb.QueryInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.QueryOutput, error)
	UpdateItem(
		ctx context.Context,
		params *dynamodb.UpdateItemInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.UpdateItemOutput, error)
	DeleteItem(
		ctx context.Context,
		params *dynamodb.DeleteItemInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.DeleteItemOutput, error)
	BatchWriteItem(
		ctx context.Context,
		params *dynamodb.BatchWriteItemInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.BatchWriteItemOutput, error)
	Scan(
		ctx context.Context,
		params *dynamodb.ScanInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.ScanOutput, error)
}

// ClientAdapter wraps the AWS SDK DynamoDB client to implement Client interface.
// This allows us to use the real AWS client in production while maintaining testability.
type ClientAdapter struct {
	client *dynamodb.Client
}

// NewClientAdapter creates a new adapter wrapping the AWS SDK DynamoDB client.
func NewClientAdapter(client *dynamodb.Client) *ClientAdapter {
	return &ClientAdapter{client: client}
}

// PutItem wraps the AWS SDK PutItem operation.
func (a *ClientAdapter) PutItem(
	ctx context.Context,
	params *dynamodb.PutItemInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.PutItemOutput, error) {
	return a.client.PutItem(ctx, params, optFns...)
}

// GetItem wraps the AWS SDK GetItem operation.
func (a *ClientAdapter) GetItem(
	ctx context.Context,
	params *dynamodb.GetItemInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.GetItemOutput, error) {
	return a.client.GetItem(ctx, params, optFns...)
}

// Query wraps the AWS SDK Query operation.
func (a *ClientAdapter) Query(
	ctx context.Context,
	params *dynamodb.QueryInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.QueryOutput, error) {
	return a.client.Query(ctx, params, optFns...)
}

// UpdateItem wraps the AWS SDK UpdateItem operation.
func (a *ClientAdapter) UpdateItem(
	ctx context.Context,
	params *dynamodb.UpdateItemInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.UpdateItemOutput, error) {
	return a.client.UpdateItem(ctx, params, optFns...)
}

// DeleteItem wraps the AWS SDK DeleteItem operation.
func (a *ClientAdapter) DeleteItem(
	ctx context.Context,
	params *dynamodb.DeleteItemInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.DeleteItemOutput, error) {
	return a.client.DeleteItem(ctx, params, optFns...)
}

// BatchWriteItem wraps the AWS SDK BatchWriteItem operation.
func (a *ClientAdapter) BatchWriteItem(
	ctx context.Context,
	params *dynamodb.BatchWriteItemInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.BatchWriteItemOutput, error) {
	return a.client.BatchWriteItem(ctx, params, optFns...)
}

// Scan wraps the AWS SDK Scan operation.
func (a *ClientAdapter) Scan(
	ctx context.Context,
	params *dynamodb.ScanInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.ScanOutput, error) {
	return a.client.Scan(ctx, params, optFns...)
}
