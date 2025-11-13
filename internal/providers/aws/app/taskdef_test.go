package aws

import (
	"testing"

	"runvoy/internal/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
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
