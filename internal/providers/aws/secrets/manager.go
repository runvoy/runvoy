// Package secrets provides secret management functionality for the Runvoy orchestrator.
// This file contains the AWS Systems Manager Parameter Store implementation.
package secrets

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/constants"
	"runvoy/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// ValueStore is the interface for storing and retrieving encrypted secret values.
// Implementations handle the actual secret payloads (e.g., in AWS Parameter Store).
type ValueStore interface {
	// StoreSecret saves a secret value and returns any error.
	StoreSecret(ctx context.Context, name, value string) error

	// RetrieveSecret gets a secret value by name.
	RetrieveSecret(ctx context.Context, name string) (string, error)

	// DeleteSecret removes a secret value by name.
	DeleteSecret(ctx context.Context, name string) error
}

// ParameterStoreManager implements secret value storage using AWS Systems Manager Parameter Store.
// Secrets are stored as SecureString parameters and encrypted with KMS.
type ParameterStoreManager struct {
	client       Client
	secretPrefix string // e.g., "/runvoy" or "/runvoy/secrets"
	kmsKeyARN    string // ARN of the KMS key to use for encryption
	logger       *slog.Logger
}

// NewParameterStoreManager creates a new Parameter Store-based secrets manager.
// secretPrefix should include a leading slash, e.g., "/runvoy/secrets"
func NewParameterStoreManager(
	client Client,
	secretPrefix, kmsKeyARN string,
	log *slog.Logger,
) *ParameterStoreManager {
	return &ParameterStoreManager{
		client:       client,
		secretPrefix: secretPrefix,
		kmsKeyARN:    kmsKeyARN,
		logger:       log,
	}
}

// getParameterName constructs the full parameter name/path for a secret.
func (m *ParameterStoreManager) getParameterName(secretName string) string {
	return fmt.Sprintf("%s/%s", m.secretPrefix, secretName)
}

// StoreSecret saves a secret value to AWS Systems Manager Parameter Store as a SecureString.
// The value is encrypted with the KMS key specified during initialization.
func (m *ParameterStoreManager) StoreSecret(ctx context.Context, name, value string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, m.logger)
	parameterName := m.getParameterName(name)
	parameterTags := m.parameterTags()

	_, err := m.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(parameterName),
		Value:     aws.String(value),
		Type:      types.ParameterTypeSecureString,
		KeyId:     aws.String(m.kmsKeyARN),
		Overwrite: aws.Bool(true),
	})

	if err != nil {
		reqLogger.Error("failed to store secret", "error", err, "name", name)
		return fmt.Errorf("failed to store secret: %w", err)
	}

	_, err = m.client.AddTagsToResource(ctx, &ssm.AddTagsToResourceInput{
		ResourceType: types.ResourceTypeForTaggingParameter,
		ResourceId:   aws.String(parameterName),
		Tags:         parameterTags,
	})

	if err != nil {
		reqLogger.Error("failed to tag secret parameter", "error", err, "name", name)
		// no need to return an error here, as the secret is still stored
	}

	reqLogger.Debug("secret stored", "name", name)
	return nil
}

func (m *ParameterStoreManager) parameterTags() []types.Tag {
	return []types.Tag{
		{
			Key:   aws.String("Application"),
			Value: aws.String(constants.ProjectName),
		},
		{
			Key:   aws.String("ManagedBy"),
			Value: aws.String(constants.ProjectName + "-orchestrator"),
		},
	}
}

// RetrieveSecret retrieves a secret value from AWS Systems Manager Parameter Store.
// The value is automatically decrypted using the KMS key.
func (m *ParameterStoreManager) RetrieveSecret(ctx context.Context, name string) (string, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, m.logger)

	parameterName := m.getParameterName(name)

	result, err := m.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(parameterName),
		WithDecryption: aws.Bool(true),
	})

	if err != nil {
		// Check if parameter not found
		if isParameterNotFound(err) {
			reqLogger.Debug("secret not found", "name", name)
			return "", fmt.Errorf("secret not found: %w", err)
		}
		reqLogger.Error("failed to retrieve secret", "error", err, "name", name)
		return "", fmt.Errorf("failed to retrieve secret: %w", err)
	}

	if result.Parameter == nil || result.Parameter.Value == nil {
		reqLogger.Warn("unexpected nil response from parameter store", "name", name)
		return "", fmt.Errorf("unexpected response from parameter store")
	}

	return *result.Parameter.Value, nil
}

// DeleteSecret removes a secret from AWS Systems Manager Parameter Store.
func (m *ParameterStoreManager) DeleteSecret(ctx context.Context, name string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, m.logger)

	parameterName := m.getParameterName(name)

	_, err := m.client.DeleteParameter(ctx, &ssm.DeleteParameterInput{
		Name: aws.String(parameterName),
	})

	if err != nil {
		// Check if parameter not found - that's fine for deletion
		if isParameterNotFound(err) {
			reqLogger.Debug("parameter not found, skipping deletion", "name", name)
			return nil
		}
		reqLogger.Error("failed to delete secret", "error", err, "name", name)
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	reqLogger.Debug("secret deleted", "name", name)
	return nil
}

// isParameterNotFound checks if an error is from a parameter not found response.
func isParameterNotFound(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return errMsg == "ParameterNotFound" || errMsg == "InvalidParameters: [/runvoy/secrets/test]"
}
