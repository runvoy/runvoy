package dynamodb

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/database"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// SecretsRepository implements the database.SecretsRepository interface using DynamoDB.
type SecretsRepository struct {
	client    Client
	tableName string
	logger    *slog.Logger
}

// NewSecretsRepository creates a new DynamoDB-backed secrets repository.
func NewSecretsRepository(client Client, tableName string, log *slog.Logger) *SecretsRepository {
	return &SecretsRepository{
		client:    client,
		tableName: tableName,
		logger:    log,
	}
}

// secretItem represents the structure stored in DynamoDB.
// This keeps the database schema separate from the API types.
type secretItem struct {
	SecretName  string    `dynamodbav:"secret_name"` // Partition key
	KeyName     string    `dynamodbav:"key_name"`    // Environment variable name
	Description string    `dynamodbav:"description"`
	CreatedBy   string    `dynamodbav:"created_by"`
	CreatedAt   time.Time `dynamodbav:"created_at"`
	UpdatedAt   time.Time `dynamodbav:"updated_at"`
	UpdatedBy   string    `dynamodbav:"updated_by"`
}

// toAPISecret converts a secretItem to an API Secret
func (si *secretItem) toAPISecret() *api.Secret {
	return &api.Secret{
		Name:        si.SecretName,
		KeyName:     si.KeyName,
		Description: si.Description,
		CreatedBy:   si.CreatedBy,
		CreatedAt:   si.CreatedAt,
		UpdatedAt:   si.UpdatedAt,
		UpdatedBy:   si.UpdatedBy,
	}
}

// CreateSecret stores a new secret's metadata in DynamoDB.
func (r *SecretsRepository) CreateSecret(ctx context.Context, secret *api.Secret) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	now := time.Now().UTC()
	item := secretItem{
		SecretName:  secret.Name,
		KeyName:     secret.KeyName,
		Description: secret.Description,
		CreatedBy:   secret.CreatedBy,
		CreatedAt:   now,
		UpdatedAt:   now,
		UpdatedBy:   secret.CreatedBy,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		reqLogger.Error("failed to marshal secret item", "error", err)
		return appErrors.ErrInternalError("failed to marshal secret", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
		// Prevent overwriting existing secrets
		ConditionExpression: aws.String("attribute_not_exists(secret_name)"),
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return database.ErrSecretAlreadyExists
		}
		reqLogger.Error("failed to create secret", "error", err, "name", secret.Name)
		return appErrors.ErrInternalError("failed to create secret", err)
	}

	reqLogger.Debug("secret created", "name", secret.Name, "created_by", secret.CreatedBy)
	return nil
}

// GetSecret retrieves a secret's metadata by name from DynamoDB.
func (r *SecretsRepository) GetSecret(ctx context.Context, name string) (*api.Secret, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"secret_name": &types.AttributeValueMemberS{Value: name},
		},
	})

	if err != nil {
		reqLogger.Error("failed to get secret", "error", err, "name", name)
		return nil, appErrors.ErrInternalError("failed to get secret", err)
	}

	if result.Item == nil {
		return nil, database.ErrSecretNotFound
	}

	var item secretItem
	if err = attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		reqLogger.Error("failed to unmarshal secret item", "error", err, "name", name)
		return nil, appErrors.ErrInternalError("failed to unmarshal secret", err)
	}

	return item.toAPISecret(), nil
}

// ListSecrets retrieves all secrets.
func (r *SecretsRepository) ListSecrets(ctx context.Context) ([]*api.Secret, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	result, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(r.tableName),
	})

	if err != nil {
		reqLogger.Error("failed to scan secrets", "error", err)
		return nil, appErrors.ErrInternalError("failed to list secrets", err)
	}

	var items []secretItem
	if err = attributevalue.UnmarshalListOfMaps(result.Items, &items); err != nil {
		reqLogger.Error("failed to unmarshal secret items", "error", err)
		return nil, appErrors.ErrInternalError("failed to unmarshal secrets", err)
	}

	secrets := make([]*api.Secret, 0, len(items))
	for i := range items {
		secrets = append(secrets, items[i].toAPISecret())
	}

	return secrets, nil
}

// UpdateSecretMetadata updates a secret's metadata (description and keyName) in DynamoDB.
func (r *SecretsRepository) UpdateSecretMetadata(
	ctx context.Context,
	name, keyName, description, updatedBy string,
) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	now := time.Now().UTC()

	updateExpr := expression.NewBuilder().
		WithUpdate(
			expression.Set(
				expression.Name("key_name"), expression.Value(keyName),
			).
				Set(
					expression.Name("description"), expression.Value(description),
				).
				Set(
					expression.Name("updated_at"), expression.Value(now),
				).
				Set(
					expression.Name("updated_by"), expression.Value(updatedBy),
				),
		)

	expr, err := updateExpr.Build()
	if err != nil {
		reqLogger.Error("failed to build update expression", "error", err)
		return appErrors.ErrInternalError("failed to build update", err)
	}

	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"secret_name": &types.AttributeValueMemberS{Value: name},
		},
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		// Ensure the secret exists before updating
		ConditionExpression: aws.String("attribute_exists(secret_name)"),
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return database.ErrSecretNotFound
		}
		reqLogger.Error("failed to update secret", "error", err, "name", name)
		return appErrors.ErrInternalError("failed to update secret", err)
	}

	reqLogger.Debug("secret metadata updated", "name", name)
	return nil
}

// DeleteSecret removes a secret's metadata from DynamoDB.
func (r *SecretsRepository) DeleteSecret(ctx context.Context, name string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"secret_name": &types.AttributeValueMemberS{Value: name},
		},
		// Ensure the secret exists before deleting
		ConditionExpression: aws.String("attribute_exists(secret_name)"),
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return database.ErrSecretNotFound
		}
		reqLogger.Error("failed to delete secret", "error", err, "name", name)
		return appErrors.ErrInternalError("failed to delete secret", err)
	}

	reqLogger.Debug("secret deleted", "name", name)
	return nil
}

// SecretExists checks if a secret with the given name exists in DynamoDB.
func (r *SecretsRepository) SecretExists(ctx context.Context, name string) (bool, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(r.tableName),
		ConsistentRead: aws.Bool(true),
		Key: map[string]types.AttributeValue{
			"secret_name": &types.AttributeValueMemberS{Value: name},
		},
	})

	if err != nil {
		reqLogger.Error("failed to check if secret exists", "error", err, "name", name)
		return false, appErrors.ErrInternalError("failed to check secret existence", err)
	}

	return result.Item != nil, nil
}
