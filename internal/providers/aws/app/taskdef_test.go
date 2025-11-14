package aws

import (
	"context"
	"errors"
	"testing"

	"runvoy/internal/constants"
	"runvoy/internal/testutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeImageNameForTaskDef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple image name",
			input:    "ubuntu:22.04",
			expected: "ubuntu-22-04",
		},
		{
			name:     "image with slashes",
			input:    "hashicorp/terraform:1.6",
			expected: "hashicorp-terraform-1-6",
		},
		{
			name:     "image with registry",
			input:    "myregistry.com/my-image:latest",
			expected: "myregistry-com-my-image-latest",
		},
		{
			name:     "image with dots",
			input:    "gcr.io/my.project/image:v1.2.3",
			expected: "gcr-io-my-project-image-v1-2-3",
		},
		{
			name:     "image with underscores",
			input:    "my_image:tag",
			expected: "my_image-tag",
		},
		{
			name:     "image with multiple consecutive special chars",
			input:    "image///name:::tag",
			expected: "image-name-tag",
		},
		{
			name:     "image with leading/trailing special chars",
			input:    "///image:tag///",
			expected: "image-tag",
		},
		{
			name:     "already sanitized",
			input:    "image-name-tag",
			expected: "image-name-tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeImageNameForTaskDef(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTaskDefinitionFamilyName(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "simple image",
			image:    "ubuntu:22.04",
			expected: constants.TaskDefinitionFamilyPrefix + "-ubuntu-22-04",
		},
		{
			name:     "image with slashes",
			image:    "hashicorp/terraform:1.6",
			expected: constants.TaskDefinitionFamilyPrefix + "-hashicorp-terraform-1-6",
		},
		{
			name:     "image with registry",
			image:    "myregistry.com/my-image:latest",
			expected: constants.TaskDefinitionFamilyPrefix + "-myregistry-com-my-image-latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TaskDefinitionFamilyName(tt.image)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractImageFromTaskDefFamily(t *testing.T) {
	tests := []struct {
		name       string
		familyName string
		expected   string
	}{
		{
			name:       "valid family name",
			familyName: constants.TaskDefinitionFamilyPrefix + "-ubuntu-22-04",
			expected:   "ubuntu-22-04",
		},
		{
			name:       "family name with slashes",
			familyName: constants.TaskDefinitionFamilyPrefix + "-hashicorp-terraform-1-6",
			expected:   "hashicorp-terraform-1-6",
		},
		{
			name:       "family name without prefix",
			familyName: "other-prefix-ubuntu-22-04",
			expected:   "",
		},
		{
			name:       "empty family name",
			familyName: "",
			expected:   "",
		},
		{
			name:       "family name equals prefix",
			familyName: constants.TaskDefinitionFamilyPrefix,
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractImageFromTaskDefFamily(tt.familyName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildTaskDefinitionTags(t *testing.T) {
	tests := []struct {
		name      string
		image     string
		isDefault *bool
		expected  int // Expected number of tags
	}{
		{
			name:      "image without default flag",
			image:     "ubuntu:22.04",
			isDefault: nil,
			expected:  2, // DockerImage and Application tags
		},
		{
			name:      "image with default flag false",
			image:     "ubuntu:22.04",
			isDefault: aws.Bool(false),
			expected:  2, // DockerImage and Application tags
		},
		{
			name:      "image with default flag true",
			image:     "ubuntu:22.04",
			isDefault: aws.Bool(true),
			expected:  3, // DockerImage, Application, and IsDefault tags
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := buildTaskDefinitionTags(tt.image, tt.isDefault)
			assert.Len(t, tags, tt.expected)

			// Verify DockerImage tag is always present
			foundDockerImage := false
			foundApplication := false
			foundIsDefault := false

			for _, tag := range tags {
				if tag.Key != nil && tag.Value != nil {
					switch *tag.Key {
					case constants.TaskDefinitionDockerImageTagKey:
						assert.Equal(t, tt.image, *tag.Value)
						foundDockerImage = true
					case "Application":
						assert.Equal(t, constants.ProjectName, *tag.Value)
						foundApplication = true
					case constants.TaskDefinitionIsDefaultTagKey:
						if tt.isDefault != nil && *tt.isDefault {
							assert.Equal(t, constants.TaskDefinitionIsDefaultTagValue, *tag.Value)
							foundIsDefault = true
						}
					}
				}
			}

			assert.True(t, foundDockerImage, "DockerImage tag should be present")
			assert.True(t, foundApplication, "Application tag should be present")
			if tt.isDefault != nil && *tt.isDefault {
				assert.True(t, foundIsDefault, "IsDefault tag should be present when isDefault is true")
			} else {
				assert.False(t, foundIsDefault, "IsDefault tag should not be present when isDefault is false or nil")
			}
		})
	}
}

func TestBuildTaskDefinitionTags_Values(t *testing.T) {
	image := "test-image:1.0"
	isDefault := aws.Bool(true)

	tags := buildTaskDefinitionTags(image, isDefault)

	// Verify all expected tags are present with correct values
	tagMap := make(map[string]string)
	for _, tag := range tags {
		if tag.Key != nil && tag.Value != nil {
			tagMap[*tag.Key] = *tag.Value
		}
	}

	assert.Equal(t, image, tagMap[constants.TaskDefinitionDockerImageTagKey])
	assert.Equal(t, constants.ProjectName, tagMap["Application"])
	assert.Equal(t, constants.TaskDefinitionIsDefaultTagValue, tagMap[constants.TaskDefinitionIsDefaultTagKey])
}

func TestBuildTaskDefinitionTags_ECSCompatible(t *testing.T) {
	// Verify tags are in the correct format for ECS
	image := "ubuntu:22.04"
	isDefault := aws.Bool(true)

	tags := buildTaskDefinitionTags(image, isDefault)

	for _, tag := range tags {
		assert.NotNil(t, tag.Key, "Tag key should not be nil")
		assert.NotNil(t, tag.Value, "Tag value should not be nil")
		assert.NotEmpty(t, *tag.Key, "Tag key should not be empty")
		assert.NotEmpty(t, *tag.Value, "Tag value should not be empty")
	}
}

type mockECSClient struct {
	runTaskFunc func(
		context.Context, *ecs.RunTaskInput, ...func(*ecs.Options),
	) (*ecs.RunTaskOutput, error)
	tagResourceFunc func(
		context.Context, *ecs.TagResourceInput, ...func(*ecs.Options),
	) (*ecs.TagResourceOutput, error)
	listTasksFunc func(
		context.Context, *ecs.ListTasksInput, ...func(*ecs.Options),
	) (*ecs.ListTasksOutput, error)
	describeTasksFunc func(
		context.Context, *ecs.DescribeTasksInput, ...func(*ecs.Options),
	) (*ecs.DescribeTasksOutput, error)
	stopTaskFunc func(
		context.Context, *ecs.StopTaskInput, ...func(*ecs.Options),
	) (*ecs.StopTaskOutput, error)
	describeTaskDefinitionFunc func(
		context.Context, *ecs.DescribeTaskDefinitionInput, ...func(*ecs.Options),
	) (*ecs.DescribeTaskDefinitionOutput, error)
	listTagsForResourceFunc func(
		context.Context, *ecs.ListTagsForResourceInput, ...func(*ecs.Options),
	) (*ecs.ListTagsForResourceOutput, error)
	listTaskDefinitionsFunc func(
		context.Context, *ecs.ListTaskDefinitionsInput, ...func(*ecs.Options),
	) (*ecs.ListTaskDefinitionsOutput, error)
	registerTaskDefinitionFunc func(
		context.Context, *ecs.RegisterTaskDefinitionInput, ...func(*ecs.Options),
	) (*ecs.RegisterTaskDefinitionOutput, error)
	deregisterTaskDefinitionFunc func(
		context.Context, *ecs.DeregisterTaskDefinitionInput, ...func(*ecs.Options),
	) (*ecs.DeregisterTaskDefinitionOutput, error)
	untagResourceFunc func(
		context.Context, *ecs.UntagResourceInput, ...func(*ecs.Options),
	) (*ecs.UntagResourceOutput, error)
}

func (m *mockECSClient) RunTask(
	ctx context.Context,
	params *ecs.RunTaskInput,
	optFns ...func(*ecs.Options),
) (*ecs.RunTaskOutput, error) {
	if m.runTaskFunc != nil {
		return m.runTaskFunc(ctx, params, optFns...)
	}
	return &ecs.RunTaskOutput{}, nil
}

func (m *mockECSClient) TagResource(
	ctx context.Context,
	params *ecs.TagResourceInput,
	optFns ...func(*ecs.Options),
) (*ecs.TagResourceOutput, error) {
	if m.tagResourceFunc != nil {
		return m.tagResourceFunc(ctx, params, optFns...)
	}
	return &ecs.TagResourceOutput{}, nil
}

func (m *mockECSClient) ListTasks(
	ctx context.Context,
	params *ecs.ListTasksInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTasksOutput, error) {
	if m.listTasksFunc != nil {
		return m.listTasksFunc(ctx, params, optFns...)
	}
	return &ecs.ListTasksOutput{}, nil
}

func (m *mockECSClient) DescribeTasks(
	ctx context.Context,
	params *ecs.DescribeTasksInput,
	optFns ...func(*ecs.Options),
) (*ecs.DescribeTasksOutput, error) {
	if m.describeTasksFunc != nil {
		return m.describeTasksFunc(ctx, params, optFns...)
	}
	return &ecs.DescribeTasksOutput{}, nil
}

func (m *mockECSClient) StopTask(
	ctx context.Context,
	params *ecs.StopTaskInput,
	optFns ...func(*ecs.Options),
) (*ecs.StopTaskOutput, error) {
	if m.stopTaskFunc != nil {
		return m.stopTaskFunc(ctx, params, optFns...)
	}
	return &ecs.StopTaskOutput{}, nil
}

func (m *mockECSClient) DescribeTaskDefinition(
	ctx context.Context,
	params *ecs.DescribeTaskDefinitionInput,
	optFns ...func(*ecs.Options),
) (*ecs.DescribeTaskDefinitionOutput, error) {
	if m.describeTaskDefinitionFunc != nil {
		return m.describeTaskDefinitionFunc(ctx, params, optFns...)
	}
	return &ecs.DescribeTaskDefinitionOutput{}, nil
}

func (m *mockECSClient) ListTagsForResource(
	ctx context.Context,
	params *ecs.ListTagsForResourceInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTagsForResourceOutput, error) {
	if m.listTagsForResourceFunc != nil {
		return m.listTagsForResourceFunc(ctx, params, optFns...)
	}
	return &ecs.ListTagsForResourceOutput{}, nil
}

func (m *mockECSClient) ListTaskDefinitions(
	ctx context.Context,
	params *ecs.ListTaskDefinitionsInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTaskDefinitionsOutput, error) {
	if m.listTaskDefinitionsFunc != nil {
		return m.listTaskDefinitionsFunc(ctx, params, optFns...)
	}
	return &ecs.ListTaskDefinitionsOutput{}, nil
}

func (m *mockECSClient) RegisterTaskDefinition(
	ctx context.Context,
	params *ecs.RegisterTaskDefinitionInput,
	optFns ...func(*ecs.Options),
) (*ecs.RegisterTaskDefinitionOutput, error) {
	if m.registerTaskDefinitionFunc != nil {
		return m.registerTaskDefinitionFunc(ctx, params, optFns...)
	}
	return &ecs.RegisterTaskDefinitionOutput{}, nil
}

func (m *mockECSClient) DeregisterTaskDefinition(
	ctx context.Context,
	params *ecs.DeregisterTaskDefinitionInput,
	optFns ...func(*ecs.Options),
) (*ecs.DeregisterTaskDefinitionOutput, error) {
	if m.deregisterTaskDefinitionFunc != nil {
		return m.deregisterTaskDefinitionFunc(ctx, params, optFns...)
	}
	return &ecs.DeregisterTaskDefinitionOutput{}, nil
}

func (m *mockECSClient) UntagResource(
	ctx context.Context,
	params *ecs.UntagResourceInput,
	optFns ...func(*ecs.Options),
) (*ecs.UntagResourceOutput, error) {
	if m.untagResourceFunc != nil {
		return m.untagResourceFunc(ctx, params, optFns...)
	}
	return &ecs.UntagResourceOutput{}, nil
}

func TestListTaskDefinitionsByPrefix(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully lists task definitions with prefix", func(t *testing.T) {
		mockClient := &mockECSClient{
			listTaskDefinitionsFunc: func(
				_ context.Context,
				input *ecs.ListTaskDefinitionsInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTaskDefinitionsOutput, error) {
				assert.Equal(t, ecsTypes.TaskDefinitionStatusActive, input.Status)
				return &ecs.ListTaskDefinitionsOutput{
					TaskDefinitionArns: []string{
						"arn:aws:ecs:us-east-1:123456789012:task-definition/runvoy-image-alpine-latest:1",
						"arn:aws:ecs:us-east-1:123456789012:task-definition/runvoy-image-ubuntu-22-04:2",
						"arn:aws:ecs:us-east-1:123456789012:task-definition/other-prefix-image:1",
					},
				}, nil
			},
		}

		result, err := listTaskDefinitionsByPrefix(ctx, mockClient, "runvoy-image-")

		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Contains(t, result, "arn:aws:ecs:us-east-1:123456789012:task-definition/runvoy-image-alpine-latest:1")
		assert.Contains(t, result, "arn:aws:ecs:us-east-1:123456789012:task-definition/runvoy-image-ubuntu-22-04:2")
	})

	t.Run("handles pagination", func(t *testing.T) {
		callCount := 0
		mockClient := &mockECSClient{
			listTaskDefinitionsFunc: func(
				_ context.Context,
				_ *ecs.ListTaskDefinitionsInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTaskDefinitionsOutput, error) {
				callCount++
				if callCount == 1 {
					return &ecs.ListTaskDefinitionsOutput{
						TaskDefinitionArns: []string{
							"arn:aws:ecs:us-east-1:123456789012:task-definition/runvoy-image-alpine:1",
						},
						NextToken: aws.String("next-token"),
					}, nil
				}
				return &ecs.ListTaskDefinitionsOutput{
					TaskDefinitionArns: []string{
						"arn:aws:ecs:us-east-1:123456789012:task-definition/runvoy-image-ubuntu:2",
					},
				}, nil
			},
		}

		result, err := listTaskDefinitionsByPrefix(ctx, mockClient, "runvoy-image-")

		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, 2, callCount)
	})

	t.Run("handles empty results", func(t *testing.T) {
		mockClient := &mockECSClient{
			listTaskDefinitionsFunc: func(
				_ context.Context,
				_ *ecs.ListTaskDefinitionsInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTaskDefinitionsOutput, error) {
				return &ecs.ListTaskDefinitionsOutput{
					TaskDefinitionArns: []string{},
				}, nil
			},
		}

		result, err := listTaskDefinitionsByPrefix(ctx, mockClient, "runvoy-image-")

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("handles ListTaskDefinitions error", func(t *testing.T) {
		mockClient := &mockECSClient{
			listTaskDefinitionsFunc: func(
				_ context.Context,
				_ *ecs.ListTaskDefinitionsInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTaskDefinitionsOutput, error) {
				return nil, errors.New("database error")
			},
		}

		result, err := listTaskDefinitionsByPrefix(ctx, mockClient, "runvoy-image-")

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to list task definitions")
	})
}

func TestGetTaskDefinitionForImage(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	t.Run("successfully finds task definition", func(t *testing.T) {
		expectedARN := "arn:aws:ecs:us-east-1:123456789012:task-definition/runvoy-image-ubuntu-22-04:5"
		mockClient := &mockECSClient{
			listTaskDefinitionsFunc: func(
				_ context.Context,
				input *ecs.ListTaskDefinitionsInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTaskDefinitionsOutput, error) {
				familyPrefix := constants.TaskDefinitionFamilyPrefix + "-ubuntu-22-04"
				assert.Equal(t, familyPrefix, *input.FamilyPrefix)
				assert.Equal(t, ecsTypes.TaskDefinitionStatusActive, input.Status)
				assert.Equal(t, int32(1), *input.MaxResults)
				return &ecs.ListTaskDefinitionsOutput{
					TaskDefinitionArns: []string{expectedARN},
				}, nil
			},
		}

		result, err := GetTaskDefinitionForImage(ctx, mockClient, "ubuntu:22.04", logger)

		require.NoError(t, err)
		assert.Equal(t, expectedARN, result)
	})

	t.Run("handles task definition not found", func(t *testing.T) {
		mockClient := &mockECSClient{
			listTaskDefinitionsFunc: func(
				_ context.Context,
				_ *ecs.ListTaskDefinitionsInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTaskDefinitionsOutput, error) {
				return &ecs.ListTaskDefinitionsOutput{
					TaskDefinitionArns: []string{},
				}, nil
			},
		}

		result, err := GetTaskDefinitionForImage(ctx, mockClient, "ubuntu:22.04", logger)

		require.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "task definition for image")
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("handles ListTaskDefinitions error", func(t *testing.T) {
		mockClient := &mockECSClient{
			listTaskDefinitionsFunc: func(
				_ context.Context,
				_ *ecs.ListTaskDefinitionsInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTaskDefinitionsOutput, error) {
				return nil, errors.New("database error")
			},
		}

		result, err := GetTaskDefinitionForImage(ctx, mockClient, "ubuntu:22.04", logger)

		require.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "failed to list task definitions")
	})
}

func TestGetDefaultImage(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	t.Run("successfully finds default image", func(t *testing.T) {
		expectedImage := "ubuntu:22.04"
		callCount := 0
		mockClient := &mockECSClient{
			listTaskDefinitionsFunc: func(
				_ context.Context,
				_ *ecs.ListTaskDefinitionsInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTaskDefinitionsOutput, error) {
				return &ecs.ListTaskDefinitionsOutput{
					TaskDefinitionArns: []string{
						"arn:aws:ecs:us-east-1:123456789012:task-definition/runvoy-image-alpine:1",
						"arn:aws:ecs:us-east-1:123456789012:task-definition/runvoy-image-ubuntu-22-04:2",
					},
				}, nil
			},
			listTagsForResourceFunc: func(
				_ context.Context,
				input *ecs.ListTagsForResourceInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTagsForResourceOutput, error) {
				callCount++
				arn := *input.ResourceArn
				if arn == "arn:aws:ecs:us-east-1:123456789012:task-definition/runvoy-image-ubuntu-22-04:2" {
					return &ecs.ListTagsForResourceOutput{
						Tags: []ecsTypes.Tag{
							{
								Key:   aws.String(constants.TaskDefinitionIsDefaultTagKey),
								Value: aws.String(constants.TaskDefinitionIsDefaultTagValue),
							},
							{
								Key:   aws.String(constants.TaskDefinitionDockerImageTagKey),
								Value: aws.String(expectedImage),
							},
						},
					}, nil
				}
				return &ecs.ListTagsForResourceOutput{
					Tags: []ecsTypes.Tag{},
				}, nil
			},
		}

		result, err := GetDefaultImage(ctx, mockClient, logger)

		require.NoError(t, err)
		assert.Equal(t, expectedImage, result)
		assert.GreaterOrEqual(t, callCount, 1)
	})

	t.Run("returns empty string when no default image found", func(t *testing.T) {
		mockClient := &mockECSClient{
			listTaskDefinitionsFunc: func(
				_ context.Context,
				_ *ecs.ListTaskDefinitionsInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTaskDefinitionsOutput, error) {
				return &ecs.ListTaskDefinitionsOutput{
					TaskDefinitionArns: []string{
						"arn:aws:ecs:us-east-1:123456789012:task-definition/runvoy-image-alpine:1",
					},
				}, nil
			},
			listTagsForResourceFunc: func(
				_ context.Context,
				_ *ecs.ListTagsForResourceInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTagsForResourceOutput, error) {
				return &ecs.ListTagsForResourceOutput{
					Tags: []ecsTypes.Tag{},
				}, nil
			},
		}

		result, err := GetDefaultImage(ctx, mockClient, logger)

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("handles ListTaskDefinitions error", func(t *testing.T) {
		mockClient := &mockECSClient{
			listTaskDefinitionsFunc: func(
				_ context.Context,
				_ *ecs.ListTaskDefinitionsInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTaskDefinitionsOutput, error) {
				return nil, errors.New("database error")
			},
		}

		result, err := GetDefaultImage(ctx, mockClient, logger)

		require.Error(t, err)
		assert.Empty(t, result)
	})

	t.Run("continues on ListTagsForResource error", func(t *testing.T) {
		callCount := 0
		mockClient := &mockECSClient{
			listTaskDefinitionsFunc: func(
				_ context.Context,
				_ *ecs.ListTaskDefinitionsInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTaskDefinitionsOutput, error) {
				return &ecs.ListTaskDefinitionsOutput{
					TaskDefinitionArns: []string{
						"arn:aws:ecs:us-east-1:123456789012:task-definition/runvoy-image-alpine:1",
						"arn:aws:ecs:us-east-1:123456789012:task-definition/runvoy-image-ubuntu-22-04:2",
					},
				}, nil
			},
			listTagsForResourceFunc: func(
				_ context.Context,
				_ *ecs.ListTagsForResourceInput,
				_ ...func(*ecs.Options),
			) (*ecs.ListTagsForResourceOutput, error) {
				callCount++
				if callCount == 1 {
					return nil, errors.New("tags error")
				}
				return &ecs.ListTagsForResourceOutput{
					Tags: []ecsTypes.Tag{
						{
							Key:   aws.String(constants.TaskDefinitionIsDefaultTagKey),
							Value: aws.String(constants.TaskDefinitionIsDefaultTagValue),
						},
						{
							Key:   aws.String(constants.TaskDefinitionDockerImageTagKey),
							Value: aws.String("ubuntu:22.04"),
						},
					},
				}, nil
			},
		}

		result, err := GetDefaultImage(ctx, mockClient, logger)

		require.NoError(t, err)
		assert.Equal(t, "ubuntu:22.04", result)
	})
}