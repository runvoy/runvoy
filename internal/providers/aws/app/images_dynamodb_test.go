package aws

import (
	"context"
	"testing"

	"runvoy/internal/api"
	"runvoy/internal/testutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRoleARN(t *testing.T) {
	tests := []struct {
		name      string
		roleName  *string
		accountID string
		region    string
		expected  string
	}{
		{
			name:      "nil role name",
			roleName:  nil,
			accountID: "123456789012",
			region:    "us-east-1",
			expected:  "",
		},
		{
			name:      "empty role name",
			roleName:  aws.String(""),
			accountID: "123456789012",
			region:    "us-east-1",
			expected:  "",
		},
		{
			name:      "valid role name",
			roleName:  aws.String("my-task-role"),
			accountID: "123456789012",
			region:    "us-east-1",
			expected:  "arn:aws:iam::123456789012:role/my-task-role",
		},
		{
			name:      "role name with hyphens and underscores",
			roleName:  aws.String("my-complex_role-name-123"),
			accountID: "987654321098",
			region:    "eu-west-1",
			expected:  "arn:aws:iam::987654321098:role/my-complex_role-name-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildRoleARN(tt.roleName, tt.accountID, tt.region)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRunner_BuildRoleARNs(t *testing.T) {
	tests := []struct {
		name                  string
		cfg                   Config
		taskRoleName          *string
		taskExecutionRoleName *string
		region                string
		expectedTaskRole      string
		expectedExecRole      string
	}{
		{
			name: "both roles nil, use config defaults",
			cfg: Config{
				AccountID:              "123456789012",
				DefaultTaskRoleARN:     "arn:aws:iam::123456789012:role/default-task",
				DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
			},
			taskRoleName:          nil,
			taskExecutionRoleName: nil,
			region:                "us-east-1",
			expectedTaskRole:      "arn:aws:iam::123456789012:role/default-task",
			expectedExecRole:      "arn:aws:iam::123456789012:role/default-exec",
		},
		{
			name: "custom task role, default exec role",
			cfg: Config{
				AccountID:              "123456789012",
				DefaultTaskRoleARN:     "arn:aws:iam::123456789012:role/default-task",
				DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
			},
			taskRoleName:          aws.String("custom-task-role"),
			taskExecutionRoleName: nil,
			region:                "us-east-1",
			expectedTaskRole:      "arn:aws:iam::123456789012:role/custom-task-role",
			expectedExecRole:      "arn:aws:iam::123456789012:role/default-exec",
		},
		{
			name: "default task role, custom exec role",
			cfg: Config{
				AccountID:              "123456789012",
				DefaultTaskRoleARN:     "arn:aws:iam::123456789012:role/default-task",
				DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
			},
			taskRoleName:          nil,
			taskExecutionRoleName: aws.String("custom-exec-role"),
			region:                "us-east-1",
			expectedTaskRole:      "arn:aws:iam::123456789012:role/default-task",
			expectedExecRole:      "arn:aws:iam::123456789012:role/custom-exec-role",
		},
		{
			name: "both custom roles",
			cfg: Config{
				AccountID:              "123456789012",
				DefaultTaskRoleARN:     "arn:aws:iam::123456789012:role/default-task",
				DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
			},
			taskRoleName:          aws.String("custom-task-role"),
			taskExecutionRoleName: aws.String("custom-exec-role"),
			region:                "us-east-1",
			expectedTaskRole:      "arn:aws:iam::123456789012:role/custom-task-role",
			expectedExecRole:      "arn:aws:iam::123456789012:role/custom-exec-role",
		},
		{
			name: "no default task role in config",
			cfg: Config{
				AccountID:              "123456789012",
				DefaultTaskRoleARN:     "", // Empty default
				DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
			},
			taskRoleName:          nil,
			taskExecutionRoleName: nil,
			region:                "us-east-1",
			expectedTaskRole:      "", // Should be empty
			expectedExecRole:      "arn:aws:iam::123456789012:role/default-exec",
		},
		{
			name: "empty string role names should use defaults",
			cfg: Config{
				AccountID:              "123456789012",
				DefaultTaskRoleARN:     "arn:aws:iam::123456789012:role/default-task",
				DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
			},
			taskRoleName:          aws.String(""),
			taskExecutionRoleName: aws.String(""),
			region:                "us-east-1",
			expectedTaskRole:      "arn:aws:iam::123456789012:role/default-task",
			expectedExecRole:      "arn:aws:iam::123456789012:role/default-exec",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &Runner{
				cfg:    &tt.cfg,
				logger: testutil.SilentLogger(),
			}

			taskRoleARN, taskExecRoleARN := runner.buildRoleARNs(
				tt.taskRoleName,
				tt.taskExecutionRoleName,
				tt.region,
			)

			assert.Equal(t, tt.expectedTaskRole, taskRoleARN, "task role ARN mismatch")
			assert.Equal(t, tt.expectedExecRole, taskExecRoleARN, "exec role ARN mismatch")
		})
	}
}

// mockImageRepo is a mock implementation of the image repository for testing
type mockImageRepo struct {
	getDefaultImageFunc func(ctx context.Context) (*api.ImageInfo, error)
}

func (m *mockImageRepo) GetDefaultImage(ctx context.Context) (*api.ImageInfo, error) {
	if m.getDefaultImageFunc != nil {
		return m.getDefaultImageFunc(ctx)
	}
	return nil, nil
}

func (m *mockImageRepo) GetImageTaskDef(ctx context.Context, image string, taskRoleName, taskExecutionRoleName *string) (*api.ImageInfo, error) {
	return nil, nil
}

func (m *mockImageRepo) PutImageTaskDef(ctx context.Context, image, imageRegistry, imageName, imageTag string, taskRoleName, taskExecutionRoleName *string, taskDefFamily string, isDefault bool) error {
	return nil
}

func (m *mockImageRepo) ListImages(ctx context.Context) ([]api.ImageInfo, error) {
	return nil, nil
}

func (m *mockImageRepo) DeleteImage(ctx context.Context, image string) error {
	return nil
}

func (m *mockImageRepo) UnmarkAllDefaults(ctx context.Context) error {
	return nil
}

func (m *mockImageRepo) SetImageAsOnlyDefault(ctx context.Context, image string, taskRoleName, taskExecutionRoleName *string) error {
	return nil
}

func TestRunner_DetermineDefaultStatus(t *testing.T) {
	ctx := testutil.TestContext()

	tests := []struct {
		name        string
		isDefault   *bool
		mockSetup   func(*mockImageRepo)
		expected    bool
		expectError bool
	}{
		{
			name:      "explicitly set to true",
			isDefault: aws.Bool(true),
			expected:  true,
		},
		{
			name:      "explicitly set to false",
			isDefault: aws.Bool(false),
			expected:  false,
		},
		{
			name:      "nil and no default exists - should become default",
			isDefault: nil,
			mockSetup: func(m *mockImageRepo) {
				m.getDefaultImageFunc = func(ctx context.Context) (*api.ImageInfo, error) {
					return nil, nil // No default exists
				}
			},
			expected: true,
		},
		{
			name:      "nil and default exists - should not become default",
			isDefault: nil,
			mockSetup: func(m *mockImageRepo) {
				m.getDefaultImageFunc = func(ctx context.Context) (*api.ImageInfo, error) {
					isDefault := true
					return &api.ImageInfo{
						Image:     "existing-default:latest",
						IsDefault: &isDefault,
					}, nil
				}
			},
			expected: false,
		},
		{
			name:      "error checking for default",
			isDefault: nil,
			mockSetup: func(m *mockImageRepo) {
				m.getDefaultImageFunc = func(ctx context.Context) (*api.ImageInfo, error) {
					return nil, assert.AnError
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockImageRepo{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockRepo)
			}

			runner := &Runner{
				imageRepo: mockRepo,
				logger:    testutil.SilentLogger(),
			}

			result, err := runner.determineDefaultStatus(ctx, tt.isDefault)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
