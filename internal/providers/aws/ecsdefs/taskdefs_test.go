package ecsdefs

import (
	"testing"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"

	"runvoy/internal/constants"
	awsConstants "runvoy/internal/providers/aws/constants"
)

func TestBuildTaskDefinitionTags(t *testing.T) {
	tests := []struct {
		name      string
		image     string
		isDefault *bool
		wantTags  int
	}{
		{
			name:      "basic tags without default",
			image:     "ubuntu:22.04",
			isDefault: nil,
			wantTags:  3,
		},
		{
			name:      "tags with default flag",
			image:     "ubuntu:22.04",
			isDefault: awsStd.Bool(true),
			wantTags:  4,
		},
		{
			name:      "tags with default false",
			image:     "ubuntu:22.04",
			isDefault: awsStd.Bool(false),
			wantTags:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := BuildTaskDefinitionTags(tt.image, tt.isDefault)

			assert.Len(t, tags, tt.wantTags)

			dockerImageTagFound := false
			isDefaultTagFound := false
			applicationTagFound := false
			managedByTagFound := false

			for _, tag := range tags {
				if tag.Key != nil && tag.Value != nil {
					key := *tag.Key
					value := *tag.Value

					switch key {
					case awsConstants.TaskDefinitionDockerImageTagKey:
						assert.Equal(t, tt.image, value)
						dockerImageTagFound = true
					case awsConstants.TaskDefinitionIsDefaultTagKey:
						if tt.isDefault != nil && *tt.isDefault {
							assert.Equal(t, awsConstants.TaskDefinitionIsDefaultTagValue, value)
							isDefaultTagFound = true
						}
					case constants.ResourceApplicationTagKey:
						applicationTagFound = true
					case constants.ResourceManagedByTagKey:
						managedByTagFound = true
					}
				}
			}

			assert.True(t, dockerImageTagFound, "Docker image tag should be present")
			assert.True(t, applicationTagFound, "Application tag should be present")
			assert.True(t, managedByTagFound, "ManagedBy tag should be present")

			if tt.isDefault != nil && *tt.isDefault {
				assert.True(t, isDefaultTagFound, "IsDefault tag should be present when isDefault is true")
			} else {
				assert.False(t, isDefaultTagFound, "IsDefault tag should not be present when isDefault is false or nil")
			}
		})
	}
}

func TestBuildTagMap(t *testing.T) {
	tests := []struct {
		name     string
		tags     []types.Tag
		expected map[string]string
	}{
		{
			name: "normal tags",
			tags: []types.Tag{
				{Key: awsStd.String("key1"), Value: awsStd.String("value1")},
				{Key: awsStd.String("key2"), Value: awsStd.String("value2")},
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "tags with nil values",
			tags: []types.Tag{
				{Key: awsStd.String("key1"), Value: awsStd.String("value1")},
				{Key: nil, Value: awsStd.String("value2")},
				{Key: awsStd.String("key3"), Value: nil},
			},
			expected: map[string]string{
				"key1": "value1",
			},
		},
		{
			name:     "empty tags",
			tags:     []types.Tag{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTagMap(tt.tags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindTagsToAdd(t *testing.T) {
	tests := []struct {
		name         string
		expectedTags []types.Tag
		currentMap   map[string]string
		wantCount    int
		wantKeys     []string
	}{
		{
			name: "all tags need to be added",
			expectedTags: []types.Tag{
				{Key: awsStd.String("key1"), Value: awsStd.String("value1")},
				{Key: awsStd.String("key2"), Value: awsStd.String("value2")},
			},
			currentMap: map[string]string{},
			wantCount:  2,
			wantKeys:   []string{"key1", "key2"},
		},
		{
			name: "some tags need updating",
			expectedTags: []types.Tag{
				{Key: awsStd.String("key1"), Value: awsStd.String("value1")},
				{Key: awsStd.String("key2"), Value: awsStd.String("newvalue")},
			},
			currentMap: map[string]string{
				"key1": "value1",
				"key2": "oldvalue",
			},
			wantCount: 1,
			wantKeys:  []string{"key2"},
		},
		{
			name: "no tags to add",
			expectedTags: []types.Tag{
				{Key: awsStd.String("key1"), Value: awsStd.String("value1")},
			},
			currentMap: map[string]string{
				"key1": "value1",
			},
			wantCount: 0,
			wantKeys:  []string{},
		},
		{
			name: "nil key is skipped",
			expectedTags: []types.Tag{
				{Key: nil, Value: awsStd.String("value1")},
				{Key: awsStd.String("key2"), Value: awsStd.String("value2")},
			},
			currentMap: map[string]string{},
			wantCount:  1,
			wantKeys:   []string{"key2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findTagsToAdd(tt.expectedTags, tt.currentMap)
			assert.Len(t, result, tt.wantCount)

			resultKeys := make([]string, 0, len(result))
			for _, tag := range result {
				if tag.Key != nil {
					resultKeys = append(resultKeys, *tag.Key)
				}
			}
			assert.ElementsMatch(t, tt.wantKeys, resultKeys)
		})
	}
}

func TestFindTagsToRemove(t *testing.T) {
	tests := []struct {
		name        string
		currentMap  map[string]string
		expectedMap map[string]string
		wantCount   int
		wantKeys    []string
	}{
		{
			name: "remove standard tags not in expected",
			currentMap: map[string]string{
				awsConstants.TaskDefinitionDockerImageTagKey: "image1",
				"other-key": "other-value",
			},
			expectedMap: map[string]string{},
			wantCount:   1,
			wantKeys:    []string{awsConstants.TaskDefinitionDockerImageTagKey},
		},
		{
			name: "don't remove non-standard tags",
			currentMap: map[string]string{
				"custom-key": "custom-value",
			},
			expectedMap: map[string]string{},
			wantCount:   0,
			wantKeys:    []string{},
		},
		{
			name: "don't remove tags in expected map",
			currentMap: map[string]string{
				awsConstants.TaskDefinitionDockerImageTagKey: "image1",
			},
			expectedMap: map[string]string{
				awsConstants.TaskDefinitionDockerImageTagKey: "image1",
			},
			wantCount: 0,
			wantKeys:  []string{},
		},
		{
			name: "remove multiple standard tags",
			currentMap: map[string]string{
				awsConstants.TaskDefinitionDockerImageTagKey: "image1",
				awsConstants.TaskDefinitionIsDefaultTagKey:   "true",
				constants.ResourceApplicationTagKey:          "runvoy",
			},
			expectedMap: map[string]string{},
			wantCount:   3,
			wantKeys: []string{
				awsConstants.TaskDefinitionDockerImageTagKey,
				awsConstants.TaskDefinitionIsDefaultTagKey,
				constants.ResourceApplicationTagKey,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findTagsToRemove(tt.currentMap, tt.expectedMap)
			assert.Len(t, result, tt.wantCount)
			assert.ElementsMatch(t, tt.wantKeys, result)
		})
	}
}

func TestIsStandardTag(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "DockerImage tag",
			key:      awsConstants.TaskDefinitionDockerImageTagKey,
			expected: true,
		},
		{
			name:     "IsDefault tag",
			key:      awsConstants.TaskDefinitionIsDefaultTagKey,
			expected: true,
		},
		{
			name:     "Application tag",
			key:      constants.ResourceApplicationTagKey,
			expected: true,
		},
		{
			name:     "ManagedBy tag",
			key:      constants.ResourceManagedByTagKey,
			expected: true,
		},
		{
			name:     "custom tag",
			key:      "custom-key",
			expected: false,
		},
		{
			name:     "empty key",
			key:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isStandardTag(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseRuntimePlatform(t *testing.T) {
	tests := []struct {
		name            string
		runtimePlatform string
		wantOS          string
		wantArch        string
		wantErr         bool
	}{
		{
			name:            "Linux/ARM64",
			runtimePlatform: "Linux/ARM64",
			wantOS:          "Linux",
			wantArch:        "ARM64",
			wantErr:         false,
		},
		{
			name:            "Linux/X86_64",
			runtimePlatform: "Linux/X86_64",
			wantOS:          "Linux",
			wantArch:        "X86_64",
			wantErr:         false,
		},
		{
			name:            "unsupported platform",
			runtimePlatform: "Windows/ARM64",
			wantOS:          "",
			wantArch:        "",
			wantErr:         true,
		},
		{
			name:            "invalid format",
			runtimePlatform: "Linux",
			wantOS:          "",
			wantArch:        "",
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			osFamily, cpuArch, err := parseRuntimePlatform(tt.runtimePlatform)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, osFamily)
				assert.Empty(t, cpuArch)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantOS, osFamily)
				assert.Equal(t, tt.wantArch, cpuArch)
			}
		})
	}
}

func TestConvertOSFamilyToECSEnum(t *testing.T) {
	tests := []struct {
		name     string
		osFamily string
		expected string
	}{
		{
			name:     "Linux",
			osFamily: "Linux",
			expected: "LINUX",
		},
		{
			name:     "linux lowercase",
			osFamily: "linux",
			expected: "LINUX",
		},
		{
			name:     "WINDOWS uppercase",
			osFamily: "WINDOWS",
			expected: "WINDOWS",
		},
		{
			name:     "Windows mixed case",
			osFamily: "Windows",
			expected: "WINDOWS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertOSFamilyToECSEnum(tt.osFamily)
			assert.Equal(t, types.OSFamily(tt.expected), result)
		})
	}
}
