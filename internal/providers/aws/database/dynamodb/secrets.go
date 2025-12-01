package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/database"
	appErrors "github.com/runvoy/runvoy/internal/errors"
	"github.com/runvoy/runvoy/internal/logger"

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
	SecretName          string    `dynamodbav:"secret_name"` // Partition key
	KeyName             string    `dynamodbav:"key_name"`    // Environment variable name
	Description         string    `dynamodbav:"description"`
	CreatedBy           string    `dynamodbav:"created_by"`
	OwnedBy             []string  `dynamodbav:"owned_by"`
	CreatedAt           time.Time `dynamodbav:"created_at"`
	UpdatedAt           time.Time `dynamodbav:"updated_at"`
	UpdatedBy           string    `dynamodbav:"updated_by"`
	CreatedByRequestID  string    `dynamodbav:"created_by_request_id,omitempty"`
	ModifiedByRequestID string    `dynamodbav:"modified_by_request_id,omitempty"`
}

// toAPISecret converts a secretItem to an API Secret.
func (si *secretItem) toAPISecret() *api.Secret {
	return &api.Secret{
		Name:                si.SecretName,
		KeyName:             si.KeyName,
		Description:         si.Description,
		CreatedBy:           si.CreatedBy,
		OwnedBy:             si.OwnedBy,
		CreatedAt:           si.CreatedAt,
		UpdatedAt:           si.UpdatedAt,
		UpdatedBy:           si.UpdatedBy,
		CreatedByRequestID:  si.CreatedByRequestID,
		ModifiedByRequestID: si.ModifiedByRequestID,
	}
}

// CreateSecret stores a new secret's metadata in DynamoDB.
func (r *SecretsRepository) CreateSecret(ctx context.Context, secret *api.Secret) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	now := time.Now().UTC()
	item := secretItem{
		SecretName:          secret.Name,
		KeyName:             secret.KeyName,
		Description:         secret.Description,
		CreatedBy:           secret.CreatedBy,
		OwnedBy:             secret.OwnedBy,
		CreatedAt:           now,
		UpdatedAt:           now,
		UpdatedBy:           secret.CreatedBy,
		CreatedByRequestID:  secret.CreatedByRequestID,
		ModifiedByRequestID: secret.ModifiedByRequestID,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		reqLogger.Error("failed to marshal secret item", "error", err)
		return appErrors.ErrInternalError("failed to marshal secret", err)
	}

	av["_all"] = &types.AttributeValueMemberS{Value: "SECRET"}

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

	var items []secretItem
	var lastKey map[string]types.AttributeValue

	exprNames := map[string]string{
		"#all": "_all",
	}
	exprValues := map[string]types.AttributeValue{
		":all": &types.AttributeValueMemberS{Value: "SECRET"},
	}

	for {
		result, err := r.client.Query(ctx, &dynamodb.QueryInput{
			TableName:                 aws.String(r.tableName),
			IndexName:                 aws.String("all-secret_name"),
			KeyConditionExpression:    aws.String("#all = :all"),
			ExpressionAttributeNames:  exprNames,
			ExpressionAttributeValues: exprValues,
			ScanIndexForward:          aws.Bool(true),
			ExclusiveStartKey:         lastKey,
		})
		if err != nil {
			reqLogger.Error("failed to query secrets", "error", err)
			return nil, appErrors.ErrInternalError("failed to list secrets", err)
		}

		var batchItems []secretItem
		if err = attributevalue.UnmarshalListOfMaps(result.Items, &batchItems); err != nil {
			reqLogger.Error("failed to unmarshal secret items", "error", err)
			return nil, appErrors.ErrInternalError("failed to unmarshal secrets", err)
		}

		items = append(items, batchItems...)

		if len(result.LastEvaluatedKey) == 0 {
			break
		}
		lastKey = result.LastEvaluatedKey
	}

	secrets := make([]*api.Secret, 0, len(items))
	for i := range items {
		secrets = append(secrets, items[i].toAPISecret())
	}

	return secrets, nil
}

// GetSecretsByRequestID retrieves all secrets created or modified by a specific request ID.
//
//nolint:dupl // Similar pattern to GetUsersByRequestID and GetImagesByRequestID, but different types
func (r *SecretsRepository) GetSecretsByRequestID(ctx context.Context, requestID string) ([]*api.Secret, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"request_id", requestID,
		"indexes", []string{createdByRequestIDIndexName, modifiedByRequestIDIndexName},
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	createdSecrets, err := r.querySecretsByRequestIDIndex(ctx, createdByRequestIDIndexName, requestID)
	if err != nil {
		return nil, err
	}

	modifiedSecrets, err := r.querySecretsByRequestIDIndex(ctx, modifiedByRequestIDIndexName, requestID)
	if err != nil {
		return nil, err
	}

	secretMap := make(map[string]*api.Secret)
	for _, secret := range createdSecrets {
		secretMap[secret.Name] = secret
	}
	for _, secret := range modifiedSecrets {
		secretMap[secret.Name] = secret
	}

	secrets := make([]*api.Secret, 0, len(secretMap))
	for _, secret := range secretMap {
		secrets = append(secrets, secret)
	}

	return secrets, nil
}

// querySecretsByRequestIDIndex queries a GSI by request ID and returns all matching secrets.
//
//nolint:dupl // Similar pattern across repositories, but different types
func (r *SecretsRepository) querySecretsByRequestIDIndex(
	ctx context.Context,
	indexName string,
	requestID string,
) ([]*api.Secret, error) {
	secrets := make([]*api.Secret, 0)
	var lastKey map[string]types.AttributeValue

	var attributeName string
	switch indexName {
	case createdByRequestIDIndexName:
		attributeName = createdByRequestIDAttrName
	case modifiedByRequestIDIndexName:
		attributeName = modifiedByRequestIDAttrName
	default:
		return nil, appErrors.ErrInternalError("unknown index name: "+indexName, nil)
	}

	exprNames := map[string]string{
		"#request_id": attributeName,
	}
	exprValues := map[string]types.AttributeValue{
		":request_id": &types.AttributeValueMemberS{Value: requestID},
	}

	for {
		queryInput := &dynamodb.QueryInput{
			TableName:                 aws.String(r.tableName),
			IndexName:                 aws.String(indexName),
			KeyConditionExpression:    aws.String("#request_id = :request_id"),
			ExpressionAttributeNames:  exprNames,
			ExpressionAttributeValues: exprValues,
			ScanIndexForward:          aws.Bool(false),
			ExclusiveStartKey:         lastKey,
		}

		result, err := r.client.Query(ctx, queryInput)
		if err != nil {
			return nil, appErrors.ErrInternalError(
				"failed to query secrets by request ID from "+indexName, err)
		}

		for _, item := range result.Items {
			var si secretItem
			if err = attributevalue.UnmarshalMap(item, &si); err != nil {
				return nil, appErrors.ErrInternalError("failed to unmarshal secret item", err)
			}
			secrets = append(secrets, si.toAPISecret())
		}

		if len(result.LastEvaluatedKey) == 0 {
			break
		}
		lastKey = result.LastEvaluatedKey
	}

	return secrets, nil
}

// buildUpdateExpression builds an update expression for secret metadata updates.
func (r *SecretsRepository) buildUpdateExpression(
	keyName, description, updatedBy string,
	now time.Time,
	requestID string,
) (expression.Expression, error) {
	updateBuilder := expression.NewBuilder().
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

	if requestID != "" {
		updateBuilder = updateBuilder.WithUpdate(
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
				).
				Set(
					expression.Name("modified_by_request_id"), expression.Value(requestID),
				),
		)
	}

	expr, err := updateBuilder.Build()
	if err != nil {
		return expression.Expression{}, fmt.Errorf("failed to build update expression: %w", err)
	}
	return expr, nil
}

// UpdateSecretMetadata updates a secret's metadata (description and keyName) in DynamoDB.
func (r *SecretsRepository) UpdateSecretMetadata(
	ctx context.Context,
	name, keyName, description, updatedBy string,
) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	now := time.Now().UTC()
	requestID := logger.GetRequestID(ctx)

	expr, err := r.buildUpdateExpression(keyName, description, updatedBy, now, requestID)
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
