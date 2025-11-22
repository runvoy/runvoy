package orchestrator

import (
	"context"
	"testing"

	"runvoy/internal/api"
	awsClient "runvoy/internal/providers/aws/client"
	"runvoy/internal/testutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
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
	getDefaultImageFunc     func(ctx context.Context) (*api.ImageInfo, error)
	listImagesFunc          func(ctx context.Context) ([]api.ImageInfo, error)
	deleteImageFunc         func(ctx context.Context, image string) error
	getAnyImageTaskDefFunc  func(ctx context.Context, image string) (*api.ImageInfo, error)
	getImageTaskDefByIDFunc func(ctx context.Context, imageID string) (*api.ImageInfo, error)
}

func (m *mockImageRepo) GetDefaultImage(ctx context.Context) (*api.ImageInfo, error) {
	if m.getDefaultImageFunc != nil {
		return m.getDefaultImageFunc(ctx)
	}
	return nil, nil
}

func (m *mockImageRepo) GetImageTaskDef(
	_ context.Context, _ string, _, _ *string, _, _ *int, _ *string,
) (*api.ImageInfo, error) {
	return nil, nil
}

func (m *mockImageRepo) GetImageTaskDefByID(ctx context.Context, imageID string) (*api.ImageInfo, error) {
	if m.getImageTaskDefByIDFunc != nil {
		return m.getImageTaskDefByIDFunc(ctx, imageID)
	}
	return nil, nil
}

func (m *mockImageRepo) GetAnyImageTaskDef(ctx context.Context, image string) (*api.ImageInfo, error) {
	if m.getAnyImageTaskDefFunc != nil {
		return m.getAnyImageTaskDefFunc(ctx, image)
	}
	return nil, nil
}

func (m *mockImageRepo) PutImageTaskDef(
	_ context.Context, _ string, _, _, _, _ string, _, _ *string, _, _ int, _ string, _ string, _ bool, _ string) error {
	return nil
}

func (m *mockImageRepo) ListImages(ctx context.Context) ([]api.ImageInfo, error) {
	if m.listImagesFunc != nil {
		return m.listImagesFunc(ctx)
	}
	return nil, nil
}

func (m *mockImageRepo) DeleteImage(ctx context.Context, image string) error {
	if m.deleteImageFunc != nil {
		return m.deleteImageFunc(ctx, image)
	}
	return nil
}

func (m *mockImageRepo) UnmarkAllDefaults(_ context.Context) error {
	return nil
}

func (m *mockImageRepo) SetImageAsOnlyDefault(_ context.Context, _ string, _, _ *string) error {
	return nil
}

func (m *mockImageRepo) GetImagesByRequestID(_ context.Context, _ string) ([]api.ImageInfo, error) {
	return []api.ImageInfo{}, nil
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
				m.getDefaultImageFunc = func(_ context.Context) (*api.ImageInfo, error) {
					return nil, nil // No default exists
				}
			},
			expected: true,
		},
		{
			name:      "nil and default exists - should not become default",
			isDefault: nil,
			mockSetup: func(m *mockImageRepo) {
				m.getDefaultImageFunc = func(_ context.Context) (*api.ImageInfo, error) {
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
				m.getDefaultImageFunc = func(_ context.Context) (*api.ImageInfo, error) {
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

func TestRunner_ListImages(t *testing.T) {
	ctx := testutil.TestContext()

	tests := []struct {
		name           string
		mockSetup      func(*mockImageRepo)
		expectedImages []api.ImageInfo
		expectError    bool
		expectedErr    string
	}{
		{
			name: "successfully lists multiple images",
			mockSetup: func(m *mockImageRepo) {
				isDefaultTrue := true
				isDefaultFalse := false
				m.listImagesFunc = func(_ context.Context) ([]api.ImageInfo, error) {
					return []api.ImageInfo{
						{
							Image:              "alpine:latest",
							IsDefault:          &isDefaultTrue,
							TaskDefinitionName: "runvoy-alpine-latest",
						},
						{
							Image:              "ubuntu:22.04",
							IsDefault:          &isDefaultFalse,
							TaskDefinitionName: "runvoy-ubuntu-2204",
						},
					}, nil
				}
			},
			expectedImages: []api.ImageInfo{
				{
					Image:              "alpine:latest",
					TaskDefinitionName: "runvoy-alpine-latest",
				},
				{
					Image:              "ubuntu:22.04",
					TaskDefinitionName: "runvoy-ubuntu-2204",
				},
			},
			expectError: false,
		},
		{
			name: "handles empty image list",
			mockSetup: func(m *mockImageRepo) {
				m.listImagesFunc = func(_ context.Context) ([]api.ImageInfo, error) {
					return []api.ImageInfo{}, nil
				}
			},
			expectedImages: []api.ImageInfo{},
			expectError:    false,
		},
		{
			name: "handles repository error",
			mockSetup: func(m *mockImageRepo) {
				m.listImagesFunc = func(_ context.Context) ([]api.ImageInfo, error) {
					return nil, assert.AnError
				}
			},
			expectError: true,
			expectedErr: "failed to list images from repository",
		},
		{
			name: "handles nil repo",
			mockSetup: func(_ *mockImageRepo) {
				// Don't set up ListImages
			},
			expectError:    false,
			expectedImages: nil,
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

			images, err := runner.ListImages(ctx)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, len(tt.expectedImages), len(images))
				if len(images) > 0 {
					for i, img := range images {
						assert.Equal(t, tt.expectedImages[i].Image, img.Image)
						assert.Equal(t, tt.expectedImages[i].TaskDefinitionName, img.TaskDefinitionName)
					}
				}
			}
		})
	}
}

func TestRunner_RemoveImage(t *testing.T) {
	ctx := testutil.TestContext()

	tests := []struct {
		name        string
		image       string
		mockSetup   func(*mockImageRepo)
		expectError bool
		expectedErr string
	}{
		{
			name:  "successfully removes image by exact ImageID",
			image: "alpine:latest-a1b2c3d4",
			mockSetup: func(mr *mockImageRepo) {
				mr.getImageTaskDefByIDFunc = func(_ context.Context, imgID string) (*api.ImageInfo, error) {
					if imgID == "alpine:latest-a1b2c3d4" {
						return &api.ImageInfo{
							ImageID:            "alpine:latest-a1b2c3d4",
							Image:              "alpine:latest",
							TaskDefinitionName: "runvoy-alpine-latest",
						}, nil
					}
					return nil, nil
				}
				mr.deleteImageFunc = func(_ context.Context, _ string) error {
					return nil
				}
			},
			expectError: false,
		},
		{
			name:  "rejects partial image name - requires exact ImageID",
			image: "alpine:latest",
			mockSetup: func(_ *mockImageRepo) {
				// mockSetup should not be called when rejecting partial names
			},
			expectError: true,
			expectedErr: "image unregister requires exact ImageID",
		},
		{
			name:  "rejects org/repo format without hash - requires exact ImageID",
			image: "myorg/myapp:v1.0",
			mockSetup: func(_ *mockImageRepo) {
				// mockSetup should not be called when rejecting partial names
			},
			expectError: true,
			expectedErr: "image unregister requires exact ImageID",
		},
		{
			name:  "handles ImageID not found returns ErrNotFound",
			image: "nonexistent:latest-a1b2c3d4",
			mockSetup: func(mr *mockImageRepo) {
				mr.getImageTaskDefByIDFunc = func(_ context.Context, _ string) (*api.ImageInfo, error) {
					return nil, nil
				}
			},
			expectError: true,
			expectedErr: "image not found",
		},
		{
			name:  "handles ImageID repository error",
			image: "alpine:latest-a1b2c3d4",
			mockSetup: func(mr *mockImageRepo) {
				mr.getImageTaskDefByIDFunc = func(_ context.Context, _ string) (*api.ImageInfo, error) {
					return nil, assert.AnError
				}
			},
			expectError: true,
			expectedErr: "failed to get image by ImageID",
		},
		{
			name:  "handles repository delete error",
			image: "alpine:latest-a1b2c3d4",
			mockSetup: func(mr *mockImageRepo) {
				mr.getImageTaskDefByIDFunc = func(_ context.Context, imgID string) (*api.ImageInfo, error) {
					if imgID == "alpine:latest-a1b2c3d4" {
						return &api.ImageInfo{
							ImageID:            "alpine:latest-a1b2c3d4",
							Image:              "alpine:latest",
							TaskDefinitionName: "runvoy-alpine-latest",
						}, nil
					}
					return nil, nil
				}
				mr.deleteImageFunc = func(_ context.Context, _ string) error {
					return assert.AnError
				}
			},
			expectError: true,
			expectedErr: "failed to delete image from repository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockImageRepo{}
			mockECS := &mockECSClient{}

			if tt.mockSetup != nil {
				tt.mockSetup(mockRepo)
			}

			runner := &Runner{
				imageRepo: mockRepo,
				ecsClient: mockECS,
				cfg:       &Config{AccountID: "123456789012"},
				logger:    testutil.SilentLogger(),
			}

			err := runner.RemoveImage(ctx, tt.image)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRunner_GetImage(t *testing.T) {
	ctx := testutil.TestContext()

	tests := []struct {
		name        string
		image       string
		mockSetup   func(*mockImageRepo)
		expected    *api.ImageInfo
		expectErr   bool
		expectedErr string
	}{
		{
			name:  "successfully gets image by name",
			image: "alpine:latest",
			mockSetup: func(m *mockImageRepo) {
				m.getAnyImageTaskDefFunc = func(_ context.Context, img string) (*api.ImageInfo, error) {
					if img == "alpine:latest" {
						return &api.ImageInfo{
							Image:              "alpine:latest",
							TaskDefinitionName: "runvoy-alpine-latest",
						}, nil
					}
					return nil, nil
				}
			},
			expected: &api.ImageInfo{
				Image:              "alpine:latest",
				TaskDefinitionName: "runvoy-alpine-latest",
			},
			expectErr: false,
		},
		{
			name:  "successfully gets image by ImageID",
			image: "alpine:latest-a1b2c3d4",
			mockSetup: func(m *mockImageRepo) {
				m.getImageTaskDefByIDFunc = func(_ context.Context, imgID string) (*api.ImageInfo, error) {
					if imgID == "alpine:latest-a1b2c3d4" {
						return &api.ImageInfo{
							Image:              "alpine:latest",
							TaskDefinitionName: "runvoy-alpine-latest",
						}, nil
					}
					return nil, nil
				}
			},
			expected: &api.ImageInfo{
				Image:              "alpine:latest",
				TaskDefinitionName: "runvoy-alpine-latest",
			},
			expectErr: false,
		},
		{
			name:  "handles image not found",
			image: "nonexistent:latest",
			mockSetup: func(m *mockImageRepo) {
				m.getAnyImageTaskDefFunc = func(_ context.Context, _ string) (*api.ImageInfo, error) {
					return nil, nil
				}
			},
			expected:    nil,
			expectErr:   true,
			expectedErr: "image not found",
		},
		{
			name:  "handles repository error for image name",
			image: "alpine:latest",
			mockSetup: func(m *mockImageRepo) {
				m.getAnyImageTaskDefFunc = func(_ context.Context, _ string) (*api.ImageInfo, error) {
					return nil, assert.AnError
				}
			},
			expected:    nil,
			expectErr:   true,
			expectedErr: "failed to get image",
		},
		{
			name:  "handles repository error for ImageID",
			image: "alpine:latest-a1b2c3d4",
			mockSetup: func(m *mockImageRepo) {
				m.getImageTaskDefByIDFunc = func(_ context.Context, _ string) (*api.ImageInfo, error) {
					return nil, assert.AnError
				}
			},
			expected:    nil,
			expectErr:   true,
			expectedErr: "failed to get image by ImageID",
		},
		{
			name:  "returns default image when image is empty",
			image: "",
			mockSetup: func(m *mockImageRepo) {
				m.getDefaultImageFunc = func(_ context.Context) (*api.ImageInfo, error) {
					isDefault := true
					return &api.ImageInfo{
						Image:              "alpine:latest",
						TaskDefinitionName: "runvoy-alpine-latest",
						IsDefault:          &isDefault,
					}, nil
				}
			},
			expected: &api.ImageInfo{
				Image:              "alpine:latest",
				TaskDefinitionName: "runvoy-alpine-latest",
			},
			expectErr: false,
		},
		{
			name:  "returns error when image is empty and no default image configured",
			image: "",
			mockSetup: func(m *mockImageRepo) {
				m.getDefaultImageFunc = func(_ context.Context) (*api.ImageInfo, error) {
					return nil, nil
				}
			},
			expected:    nil,
			expectErr:   true,
			expectedErr: "no image specified and no default image configured",
		},
		{
			name:  "handles repository error when getting default image",
			image: "",
			mockSetup: func(m *mockImageRepo) {
				m.getDefaultImageFunc = func(_ context.Context) (*api.ImageInfo, error) {
					return nil, assert.AnError
				}
			},
			expected:    nil,
			expectErr:   true,
			expectedErr: "failed to get default image",
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

			imageInfo, err := runner.GetImage(ctx, tt.image)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
				assert.Nil(t, imageInfo)
			} else {
				require.NoError(t, err)
				require.NotNil(t, imageInfo)
				assert.Equal(t, tt.expected.Image, imageInfo.Image)
				assert.Equal(t, tt.expected.TaskDefinitionName, imageInfo.TaskDefinitionName)
			}
		})
	}
}

func TestRunner_GetTaskDefinitionARNForImage(t *testing.T) {
	ctx := testutil.TestContext()

	tests := []struct {
		name            string
		image           string
		mockSetup       func(*mockImageRepo)
		expectedTaskDef string
		expectError     bool
		expectedErr     string
	}{
		{
			name:  "successfully gets task definition for image name",
			image: "alpine:latest",
			mockSetup: func(m *mockImageRepo) {
				m.getAnyImageTaskDefFunc = func(_ context.Context, _ string) (*api.ImageInfo, error) {
					return &api.ImageInfo{
						Image:              "alpine:latest",
						TaskDefinitionName: "runvoy-alpine-latest",
					}, nil
				}
			},
			expectedTaskDef: "runvoy-alpine-latest",
			expectError:     false,
		},
		{
			name:  "successfully gets task definition for ImageID",
			image: "alpine:latest-a1b2c3d4",
			mockSetup: func(m *mockImageRepo) {
				m.getImageTaskDefByIDFunc = func(_ context.Context, _ string) (*api.ImageInfo, error) {
					return &api.ImageInfo{
						Image:              "alpine:latest",
						TaskDefinitionName: "runvoy-alpine-latest",
					}, nil
				}
			},
			expectedTaskDef: "runvoy-alpine-latest",
			expectError:     false,
		},
		{
			name:  "handles task definition not found",
			image: "nonexistent:latest",
			mockSetup: func(m *mockImageRepo) {
				m.getAnyImageTaskDefFunc = func(_ context.Context, _ string) (*api.ImageInfo, error) {
					return nil, nil
				}
			},
			expectedTaskDef: "",
			expectError:     true,
			expectedErr:     "no task definition found",
		},
		{
			name:  "handles repository error",
			image: "alpine:latest",
			mockSetup: func(m *mockImageRepo) {
				m.getAnyImageTaskDefFunc = func(_ context.Context, _ string) (*api.ImageInfo, error) {
					return nil, assert.AnError
				}
			},
			expectedTaskDef: "",
			expectError:     true,
			expectedErr:     "failed to get task definition",
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

			taskDef, err := runner.GetTaskDefinitionARNForImage(ctx, tt.image)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
				assert.Equal(t, "", taskDef)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedTaskDef, taskDef)
			}
		})
	}
}

func TestRunner_GetDefaultImageFromDB(t *testing.T) {
	ctx := testutil.TestContext()

	tests := []struct {
		name        string
		mockSetup   func(*mockImageRepo)
		expected    string
		expectError bool
		expectedErr string
	}{
		{
			name: "successfully gets default image",
			mockSetup: func(m *mockImageRepo) {
				m.getDefaultImageFunc = func(_ context.Context) (*api.ImageInfo, error) {
					isDefault := true
					return &api.ImageInfo{
						Image:     "alpine:latest",
						IsDefault: &isDefault,
					}, nil
				}
			},
			expected:    "alpine:latest",
			expectError: false,
		},
		{
			name: "handles no default image",
			mockSetup: func(m *mockImageRepo) {
				m.getDefaultImageFunc = func(_ context.Context) (*api.ImageInfo, error) {
					return nil, nil
				}
			},
			expected:    "",
			expectError: false,
		},
		{
			name: "handles repository error",
			mockSetup: func(m *mockImageRepo) {
				m.getDefaultImageFunc = func(_ context.Context) (*api.ImageInfo, error) {
					return nil, assert.AnError
				}
			},
			expected:    "",
			expectError: true,
			expectedErr: "failed to get default image",
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

			image, err := runner.GetDefaultImageFromDB(ctx)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, image)
			}
		})
	}
}

func TestSanitizeImageIDForTaskDef(t *testing.T) {
	tests := []struct {
		name     string
		imageID  string
		expected string
	}{
		{
			name:     "simple image ID",
			imageID:  "alpine:latest-a1b2c3d4",
			expected: "runvoy-alpine-latest-a1b2c3d4",
		},
		{
			name:     "image ID with dots",
			imageID:  "myregistry.azurecr.io:my-image-12345678",
			expected: "runvoy-myregistry-azurecr-io-my-image-12345678",
		},
		{
			name:     "image ID with slashes",
			imageID:  "ghcr.io/user/image:tag-abcdef12",
			expected: "runvoy-ghcr-io-user-image-tag-abcdef12",
		},
		{
			name:     "consecutive special characters",
			imageID:  "image...name---tag-aabbccdd",
			expected: "runvoy-image-name-tag-aabbccdd",
		},
		{
			name:     "leading/trailing special chars",
			imageID:  "---image-name-tag-aabbccdd---",
			expected: "runvoy-image-name-tag-aabbccdd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeImageIDForTaskDef(tt.imageID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLooksLikeImageID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid ImageID format",
			input:    "alpine:latest-a1b2c3d4",
			expected: true,
		},
		{
			name:     "valid ImageID with uppercase hash",
			input:    "alpine:latest-A1B2C3D4",
			expected: true,
		},
		{
			name:     "valid ImageID with mixed case hash",
			input:    "alpine:latest-aAbBcCdD",
			expected: true,
		},
		{
			name:     "missing colon",
			input:    "alpinelatest-a1b2c3d4",
			expected: false,
		},
		{
			name:     "wrong hash length (7 chars)",
			input:    "alpine:latest-a1b2c3d",
			expected: false,
		},
		{
			name:     "wrong hash length (9 chars)",
			input:    "alpine:latest-a1b2c3d45",
			expected: false,
		},
		{
			name:     "non-hex hash",
			input:    "alpine:latest-a1b2c3zz",
			expected: false,
		},
		{
			name:     "plain image name",
			input:    "alpine:latest",
			expected: false,
		},
		{
			name:     "image with registry",
			input:    "docker.io/library/alpine:latest",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := looksLikeImageID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// mockIAMClient is a mock implementation of IAMClient for testing
type mockIAMClient struct {
	getRoleFunc func(
		ctx context.Context,
		params *iam.GetRoleInput,
		optFns ...func(*iam.Options),
	) (*iam.GetRoleOutput, error)
}

func (m *mockIAMClient) GetRole(
	ctx context.Context,
	params *iam.GetRoleInput,
	optFns ...func(*iam.Options),
) (*iam.GetRoleOutput, error) {
	if m.getRoleFunc != nil {
		return m.getRoleFunc(ctx, params, optFns...)
	}
	return &iam.GetRoleOutput{}, nil
}

func TestRunner_ValidateIAMRoles(t *testing.T) {
	ctx := testutil.TestContext()

	tests := []struct {
		name                  string
		taskRoleName          *string
		taskExecutionRoleName *string
		region                string
		accountID             string
		mockSetup             func(*mockIAMClient)
		expectError           bool
		expectedError         string
		useNilIAMClient       bool
	}{
		{
			name:                  "both roles nil - no validation needed",
			taskRoleName:          nil,
			taskExecutionRoleName: nil,
			region:                "us-east-1",
			accountID:             "123456789012",
			mockSetup:             nil,
			expectError:           false,
		},
		{
			name:                  "task role exists",
			taskRoleName:          aws.String("existing-task-role"),
			taskExecutionRoleName: nil,
			region:                "us-east-1",
			accountID:             "123456789012",
			mockSetup: func(m *mockIAMClient) {
				m.getRoleFunc = func(
					_ context.Context,
					params *iam.GetRoleInput,
					_ ...func(*iam.Options),
				) (*iam.GetRoleOutput, error) {
					if *params.RoleName == "existing-task-role" {
						return &iam.GetRoleOutput{}, nil
					}
					return nil, &iamTypes.NoSuchEntityException{}
				}
			},
			expectError: false,
		},
		{
			name:                  "task execution role exists",
			taskRoleName:          nil,
			taskExecutionRoleName: aws.String("existing-exec-role"),
			region:                "us-east-1",
			accountID:             "123456789012",
			mockSetup: func(m *mockIAMClient) {
				m.getRoleFunc = func(
					_ context.Context,
					params *iam.GetRoleInput,
					_ ...func(*iam.Options),
				) (*iam.GetRoleOutput, error) {
					if *params.RoleName == "existing-exec-role" {
						return &iam.GetRoleOutput{}, nil
					}
					return nil, &iamTypes.NoSuchEntityException{}
				}
			},
			expectError: false,
		},
		{
			name:                  "both roles exist",
			taskRoleName:          aws.String("existing-task-role"),
			taskExecutionRoleName: aws.String("existing-exec-role"),
			region:                "us-east-1",
			accountID:             "123456789012",
			mockSetup: func(m *mockIAMClient) {
				m.getRoleFunc = func(
					_ context.Context,
					_ *iam.GetRoleInput,
					_ ...func(*iam.Options),
				) (*iam.GetRoleOutput, error) {
					return &iam.GetRoleOutput{}, nil
				}
			},
			expectError: false,
		},
		{
			name:                  "task role does not exist",
			taskRoleName:          aws.String("nonexistent-task-role"),
			taskExecutionRoleName: nil,
			region:                "us-east-1",
			accountID:             "123456789012",
			mockSetup: func(m *mockIAMClient) {
				m.getRoleFunc = func(
					_ context.Context,
					_ *iam.GetRoleInput,
					_ ...func(*iam.Options),
				) (*iam.GetRoleOutput, error) {
					return nil, &iamTypes.NoSuchEntityException{}
				}
			},
			expectError:   true,
			expectedError: "task IAM role does not exist",
		},
		{
			name:                  "task execution role does not exist",
			taskRoleName:          nil,
			taskExecutionRoleName: aws.String("nonexistent-exec-role"),
			region:                "us-east-1",
			accountID:             "123456789012",
			mockSetup: func(m *mockIAMClient) {
				m.getRoleFunc = func(
					_ context.Context,
					_ *iam.GetRoleInput,
					_ ...func(*iam.Options),
				) (*iam.GetRoleOutput, error) {
					return nil, &iamTypes.NoSuchEntityException{}
				}
			},
			expectError:   true,
			expectedError: "task execution IAM role does not exist",
		},
		{
			name:                  "both roles do not exist - task role error first",
			taskRoleName:          aws.String("nonexistent-task-role"),
			taskExecutionRoleName: aws.String("nonexistent-exec-role"),
			region:                "us-east-1",
			accountID:             "123456789012",
			mockSetup: func(m *mockIAMClient) {
				m.getRoleFunc = func(
					_ context.Context,
					_ *iam.GetRoleInput,
					_ ...func(*iam.Options),
				) (*iam.GetRoleOutput, error) {
					return nil, &iamTypes.NoSuchEntityException{}
				}
			},
			expectError:   true,
			expectedError: "task IAM role does not exist",
		},
		{
			name:                  "IAM client not configured",
			taskRoleName:          aws.String("some-role"),
			taskExecutionRoleName: nil,
			region:                "us-east-1",
			accountID:             "123456789012",
			mockSetup:             nil,
			expectError:           true,
			expectedError:         "IAM client not configured",
			useNilIAMClient:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var iamClient awsClient.IAMClient
			if !tt.useNilIAMClient {
				mockIAM := &mockIAMClient{}
				if tt.mockSetup != nil {
					tt.mockSetup(mockIAM)
				}
				iamClient = mockIAM
			}

			runner := &Runner{
				iamClient: iamClient,
				cfg: &Config{
					AccountID: tt.accountID,
				},
				logger: testutil.SilentLogger(),
			}

			err := runner.validateIAMRoles(ctx, tt.taskRoleName, tt.taskExecutionRoleName, tt.region, runner.logger)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
