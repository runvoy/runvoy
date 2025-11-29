package orchestrator

import (
	"testing"

	"github.com/runvoy/runvoy/internal/constants"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
)

func TestGetStandardECSTags(t *testing.T) {
	tags := GetStandardECSTags()

	// Should return 2 standard tags
	assert.Len(t, tags, 2, "should return 2 standard tags")

	// Check that tags are in ECS format
	for _, tag := range tags {
		assert.NotNil(t, tag.Key, "tag key should not be nil")
		assert.NotNil(t, tag.Value, "tag value should not be nil")
	}

	// Verify specific tag values
	expectedTags := map[string]string{
		"Application": constants.ProjectName,
		"ManagedBy":   constants.ProjectName + "-orchestrator",
	}

	actualTags := make(map[string]string)
	for _, tag := range tags {
		actualTags[*tag.Key] = *tag.Value
	}

	assert.Equal(t, expectedTags, actualTags, "tags should match expected values")
}

func TestGetStandardECSTags_Format(t *testing.T) {
	tags := GetStandardECSTags()

	// Ensure all tags are of the correct type
	for _, tag := range tags {
		assert.IsType(t, ecsTypes.Tag{}, tag, "should be ECS Tag type")
	}

	// Ensure tags have AWS string pointers
	for _, tag := range tags {
		assert.IsType(t, (*string)(nil), tag.Key, "key should be string pointer")
		assert.IsType(t, (*string)(nil), tag.Value, "value should be string pointer")
	}
}

func TestGetStandardECSTags_ConsistentOutput(t *testing.T) {
	// Call multiple times to ensure consistency
	tags1 := GetStandardECSTags()
	tags2 := GetStandardECSTags()

	assert.Equal(t, len(tags1), len(tags2), "should return same number of tags")

	// Compare tag contents
	for i := range tags1 {
		assert.Equal(t, awsStd.ToString(tags1[i].Key), awsStd.ToString(tags2[i].Key),
			"tag keys should be consistent")
		assert.Equal(t, awsStd.ToString(tags1[i].Value), awsStd.ToString(tags2[i].Value),
			"tag values should be consistent")
	}
}
