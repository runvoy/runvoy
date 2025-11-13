// Package secrets provides secret management functionality for the Runvoy orchestrator.
package secrets

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetParameterName(t *testing.T) {
	logger := slog.Default()
	m := NewParameterStoreManager(nil, "/runvoy/secrets", "arn:aws:kms:us-east-1:123456789012:key/abc", logger)

	t.Run("constructs correct parameter name", func(t *testing.T) {
		name := m.getParameterName("db-password")
		assert.Equal(t, "/runvoy/secrets/db-password", name)
	})

	t.Run("handles empty prefix", func(t *testing.T) {
		m2 := NewParameterStoreManager(nil, "", "arn:aws:kms:us-east-1:123456789012:key/abc", logger)
		name := m2.getParameterName("db-password")
		assert.Equal(t, "/db-password", name)
	})

	t.Run("handles prefix without leading slash", func(t *testing.T) {
		m2 := NewParameterStoreManager(nil, "runvoy/secrets", "arn:aws:kms:us-east-1:123456789012:key/abc", logger)
		name := m2.getParameterName("db-password")
		assert.Equal(t, "runvoy/secrets/db-password", name)
	})
}

func TestIsParameterNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "ParameterNotFound error",
			err:      errors.New("ParameterNotFound"),
			expected: true,
		},
		{
			name:     "InvalidParameters error",
			err:      errors.New("InvalidParameters: [/runvoy/secrets/test]"),
			expected: true,
		},
		{
			name:     "other error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "access denied error",
			err:      errors.New("AccessDeniedException"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isParameterNotFound(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewParameterStoreManager(t *testing.T) {
	t.Run("creates manager with correct fields", func(t *testing.T) {
		logger := slog.Default()
		prefix := "/runvoy/secrets"
		kmsKey := "arn:aws:kms:us-east-1:123456789012:key/abc"

		m := NewParameterStoreManager(nil, prefix, kmsKey, logger)

		require.NotNil(t, m)
		assert.Equal(t, prefix, m.secretPrefix)
		assert.Equal(t, kmsKey, m.kmsKeyARN)
		assert.NotNil(t, m.logger)
		assert.Nil(t, m.client)
	})
}

func TestParameterTags(t *testing.T) {
	logger := slog.Default()
	m := NewParameterStoreManager(nil, "/runvoy/secrets", "arn:aws:kms:us-east-1:123456789012:key/abc", logger)

	t.Run("returns correct tags", func(t *testing.T) {
		tags := m.parameterTags()

		require.Len(t, tags, 2)

		// Check Application tag
		assert.Equal(t, "Application", *tags[0].Key)
		assert.Equal(t, "runvoy", *tags[0].Value)

		// Check ManagedBy tag
		assert.Equal(t, "ManagedBy", *tags[1].Key)
		assert.Equal(t, "runvoy-orchestrator", *tags[1].Value)
	})
}

type mockClient struct {
	putParameterFunc func(
		context.Context, *ssm.PutParameterInput, ...func(*ssm.Options),
	) (*ssm.PutParameterOutput, error)
	addTagsToResourceFunc func(
		context.Context, *ssm.AddTagsToResourceInput, ...func(*ssm.Options),
	) (*ssm.AddTagsToResourceOutput, error)
	getParameterFunc func(
		context.Context, *ssm.GetParameterInput, ...func(*ssm.Options),
	) (*ssm.GetParameterOutput, error)
	deleteParameterFunc func(
		context.Context, *ssm.DeleteParameterInput, ...func(*ssm.Options),
	) (*ssm.DeleteParameterOutput, error)
}

func (m *mockClient) PutParameter(
	ctx context.Context,
	params *ssm.PutParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.PutParameterOutput, error) {
	if m.putParameterFunc != nil {
		return m.putParameterFunc(ctx, params, optFns...)
	}
	return &ssm.PutParameterOutput{}, nil
}

func (m *mockClient) AddTagsToResource(
	ctx context.Context,
	params *ssm.AddTagsToResourceInput,
	optFns ...func(*ssm.Options),
) (*ssm.AddTagsToResourceOutput, error) {
	if m.addTagsToResourceFunc != nil {
		return m.addTagsToResourceFunc(ctx, params, optFns...)
	}
	return &ssm.AddTagsToResourceOutput{}, nil
}

func (m *mockClient) GetParameter(
	ctx context.Context,
	params *ssm.GetParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.GetParameterOutput, error) {
	if m.getParameterFunc != nil {
		return m.getParameterFunc(ctx, params, optFns...)
	}
	return &ssm.GetParameterOutput{}, nil
}

func (m *mockClient) DeleteParameter(
	ctx context.Context,
	params *ssm.DeleteParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.DeleteParameterOutput, error) {
	if m.deleteParameterFunc != nil {
		return m.deleteParameterFunc(ctx, params, optFns...)
	}
	return &ssm.DeleteParameterOutput{}, nil
}

func TestStoreSecret(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()
	prefix := "/runvoy/secrets"
	kmsKey := "arn:aws:kms:us-east-1:123456789012:key/abc"

	t.Run("successfully stores secret", func(t *testing.T) {
		mock := &mockClient{
			putParameterFunc: func(
				_ context.Context,
				input *ssm.PutParameterInput,
				_ ...func(*ssm.Options),
			) (*ssm.PutParameterOutput, error) {
				assert.Equal(t, "/runvoy/secrets/test-secret", *input.Name)
				assert.Equal(t, "secret-value", *input.Value)
				assert.Equal(t, types.ParameterTypeSecureString, input.Type)
				assert.Equal(t, kmsKey, *input.KeyId)
				assert.True(t, *input.Overwrite)
				return &ssm.PutParameterOutput{}, nil
			},
			addTagsToResourceFunc: func(
				_ context.Context,
				input *ssm.AddTagsToResourceInput,
				_ ...func(*ssm.Options),
			) (*ssm.AddTagsToResourceOutput, error) {
				assert.Equal(t, types.ResourceTypeForTaggingParameter, input.ResourceType)
				assert.Equal(t, "/runvoy/secrets/test-secret", *input.ResourceId)
				require.Len(t, input.Tags, 2)
				return &ssm.AddTagsToResourceOutput{}, nil
			},
		}

		m := NewParameterStoreManager(mock, prefix, kmsKey, logger)
		err := m.StoreSecret(ctx, "test-secret", "secret-value")

		assert.NoError(t, err)
	})

	t.Run("handles PutParameter error", func(t *testing.T) {
		mock := &mockClient{
			putParameterFunc: func(
				_ context.Context,
				_ *ssm.PutParameterInput,
				_ ...func(*ssm.Options),
			) (*ssm.PutParameterOutput, error) {
				return nil, errors.New("access denied")
			},
		}

		m := NewParameterStoreManager(mock, prefix, kmsKey, logger)
		err := m.StoreSecret(ctx, "test-secret", "secret-value")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to store secret")
	})

	t.Run("continues when tagging fails", func(t *testing.T) {
		mock := &mockClient{
			putParameterFunc: func(
				_ context.Context,
				_ *ssm.PutParameterInput,
				_ ...func(*ssm.Options),
			) (*ssm.PutParameterOutput, error) {
				return &ssm.PutParameterOutput{}, nil
			},
			addTagsToResourceFunc: func(
				_ context.Context,
				_ *ssm.AddTagsToResourceInput,
				_ ...func(*ssm.Options),
			) (*ssm.AddTagsToResourceOutput, error) {
				return nil, errors.New("tagging failed")
			},
		}

		m := NewParameterStoreManager(mock, prefix, kmsKey, logger)
		err := m.StoreSecret(ctx, "test-secret", "secret-value")

		assert.NoError(t, err)
	})
}

func TestRetrieveSecret(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()
	prefix := "/runvoy/secrets"
	kmsKey := "arn:aws:kms:us-east-1:123456789012:key/abc"

	t.Run("successfully retrieves secret", func(t *testing.T) {
		expectedValue := "secret-value"
		mock := &mockClient{
			getParameterFunc: func(
				_ context.Context,
				input *ssm.GetParameterInput,
				_ ...func(*ssm.Options),
			) (*ssm.GetParameterOutput, error) {
				assert.Equal(t, "/runvoy/secrets/test-secret", *input.Name)
				assert.True(t, *input.WithDecryption)
				return &ssm.GetParameterOutput{
					Parameter: &types.Parameter{
						Name:  aws.String("/runvoy/secrets/test-secret"),
						Value: aws.String(expectedValue),
					},
				}, nil
			},
		}

		m := NewParameterStoreManager(mock, prefix, kmsKey, logger)
		value, err := m.RetrieveSecret(ctx, "test-secret")

		require.NoError(t, err)
		assert.Equal(t, expectedValue, value)
	})

	t.Run("handles parameter not found", func(t *testing.T) {
		mock := &mockClient{
			getParameterFunc: func(
				_ context.Context,
				_ *ssm.GetParameterInput,
				_ ...func(*ssm.Options),
			) (*ssm.GetParameterOutput, error) {
				return nil, errors.New("ParameterNotFound")
			},
		}

		m := NewParameterStoreManager(mock, prefix, kmsKey, logger)
		value, err := m.RetrieveSecret(ctx, "test-secret")

		require.Error(t, err)
		assert.Empty(t, value)
		assert.Contains(t, err.Error(), "secret not found")
	})

	t.Run("handles other errors", func(t *testing.T) {
		mock := &mockClient{
			getParameterFunc: func(
				_ context.Context,
				_ *ssm.GetParameterInput,
				_ ...func(*ssm.Options),
			) (*ssm.GetParameterOutput, error) {
				return nil, errors.New("access denied")
			},
		}

		m := NewParameterStoreManager(mock, prefix, kmsKey, logger)
		value, err := m.RetrieveSecret(ctx, "test-secret")

		require.Error(t, err)
		assert.Empty(t, value)
		assert.Contains(t, err.Error(), "failed to retrieve secret")
	})

	t.Run("handles nil parameter in response", func(t *testing.T) {
		mock := &mockClient{
			getParameterFunc: func(
				_ context.Context,
				_ *ssm.GetParameterInput,
				_ ...func(*ssm.Options),
			) (*ssm.GetParameterOutput, error) {
				return &ssm.GetParameterOutput{
					Parameter: nil,
				}, nil
			},
		}

		m := NewParameterStoreManager(mock, prefix, kmsKey, logger)
		value, err := m.RetrieveSecret(ctx, "test-secret")

		require.Error(t, err)
		assert.Empty(t, value)
		assert.Contains(t, err.Error(), "unexpected response")
	})

	t.Run("handles nil value in parameter", func(t *testing.T) {
		mock := &mockClient{
			getParameterFunc: func(
				_ context.Context,
				_ *ssm.GetParameterInput,
				_ ...func(*ssm.Options),
			) (*ssm.GetParameterOutput, error) {
				return &ssm.GetParameterOutput{
					Parameter: &types.Parameter{
						Name:  aws.String("/runvoy/secrets/test-secret"),
						Value: nil,
					},
				}, nil
			},
		}

		m := NewParameterStoreManager(mock, prefix, kmsKey, logger)
		value, err := m.RetrieveSecret(ctx, "test-secret")

		require.Error(t, err)
		assert.Empty(t, value)
		assert.Contains(t, err.Error(), "unexpected response")
	})
}

func TestDeleteSecret(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()
	prefix := "/runvoy/secrets"
	kmsKey := "arn:aws:kms:us-east-1:123456789012:key/abc"

	t.Run("successfully deletes secret", func(t *testing.T) {
		mock := &mockClient{
			deleteParameterFunc: func(
				_ context.Context,
				input *ssm.DeleteParameterInput,
				_ ...func(*ssm.Options),
			) (*ssm.DeleteParameterOutput, error) {
				assert.Equal(t, "/runvoy/secrets/test-secret", *input.Name)
				return &ssm.DeleteParameterOutput{}, nil
			},
		}

		m := NewParameterStoreManager(mock, prefix, kmsKey, logger)
		err := m.DeleteSecret(ctx, "test-secret")

		assert.NoError(t, err)
	})

	t.Run("handles parameter not found gracefully", func(t *testing.T) {
		mock := &mockClient{
			deleteParameterFunc: func(
				_ context.Context,
				_ *ssm.DeleteParameterInput,
				_ ...func(*ssm.Options),
			) (*ssm.DeleteParameterOutput, error) {
				return nil, errors.New("ParameterNotFound")
			},
		}

		m := NewParameterStoreManager(mock, prefix, kmsKey, logger)
		err := m.DeleteSecret(ctx, "test-secret")

		assert.NoError(t, err)
	})

	t.Run("handles other errors", func(t *testing.T) {
		mock := &mockClient{
			deleteParameterFunc: func(
				_ context.Context,
				_ *ssm.DeleteParameterInput,
				_ ...func(*ssm.Options),
			) (*ssm.DeleteParameterOutput, error) {
				return nil, errors.New("access denied")
			},
		}

		m := NewParameterStoreManager(mock, prefix, kmsKey, logger)
		err := m.DeleteSecret(ctx, "test-secret")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete secret")
	})
}
