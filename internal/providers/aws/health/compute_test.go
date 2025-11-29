package health

import (
	"context"
	"testing"

	"github.com/runvoy/runvoy/internal/api"
	awsConstants "github.com/runvoy/runvoy/internal/providers/aws/constants"
	"github.com/runvoy/runvoy/internal/providers/aws/ecsdefs"
	"github.com/runvoy/runvoy/internal/testutil"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
)

func TestFindOrphanedTaskDefinitions(t *testing.T) {
	mockECS := &mockECSClient{
		listTaskDefinitionsFunc: func(
			_ context.Context,
			input *ecs.ListTaskDefinitionsInput,
			_ ...func(*ecs.Options),
		) (*ecs.ListTaskDefinitionsOutput, error) {
			assert.Equal(t, ecsTypes.TaskDefinitionStatusActive, input.Status)
			return &ecs.ListTaskDefinitionsOutput{
				TaskDefinitionArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:task-definition/" + awsConstants.TaskDefinitionFamilyPrefix + "-kept:1",
					"arn:aws:ecs:us-east-1:123456789012:task-definition/" + awsConstants.TaskDefinitionFamilyPrefix + "-orphan:3",
				},
			}, nil
		},
	}

	m := &Manager{
		ecsClient: mockECS,
		cfg:       &Config{},
		logger:    testutil.SilentLogger(),
	}

	seen := map[string]bool{
		awsConstants.TaskDefinitionFamilyPrefix + "-kept": true,
	}

	orphaned, err := m.findOrphanedTaskDefinitions(context.Background(), seen, testutil.SilentLogger())

	assert.NoError(t, err)
	assert.Equal(t, []string{awsConstants.TaskDefinitionFamilyPrefix + "-orphan"}, orphaned)
}

func TestBuildTaskDefParamsDefaults(t *testing.T) {
	m := &Manager{cfg: &Config{
		AccountID:              "123456789012",
		DefaultTaskRoleARN:     "arn:aws:iam::123456789012:role/default-task",
		DefaultTaskExecRoleARN: "arn:aws:iam::123456789012:role/default-exec",
	}}

	params := m.buildTaskDefParams(&api.ImageInfo{Image: "alpine:latest"})

	assert.Equal(t, "arn:aws:iam::123456789012:role/default-task", params.taskRoleARN)
	assert.Equal(t, "arn:aws:iam::123456789012:role/default-exec", params.taskExecRoleARN)
	assert.Equal(t, awsConstants.DefaultCPU, params.cpu)
	assert.Equal(t, awsConstants.DefaultMemory, params.memory)
	assert.Equal(t, awsConstants.DefaultRuntimePlatform, params.runtimePlatform)
	assert.False(t, params.isDefault)
}

func TestBuildTaskDefParamsUsesImageValues(t *testing.T) {
	isDefault := true
	arm64Platform := awsConstants.DefaultRuntimePlatformOSFamily + "/" + awsConstants.RuntimePlatformArchARM64
	m := &Manager{cfg: &Config{AccountID: "123456789012"}}
	params := m.buildTaskDefParams(&api.ImageInfo{
		Image:                 "alpine:3.19",
		TaskRoleName:          stringPtr("custom-task"),
		TaskExecutionRoleName: stringPtr("custom-exec"),
		CPU:                   512,
		Memory:                1024,
		RuntimePlatform:       arm64Platform,
		IsDefault:             &isDefault,
	})

	assert.Equal(t, "arn:aws:iam::123456789012:role/custom-task", params.taskRoleARN)
	assert.Equal(t, "arn:aws:iam::123456789012:role/custom-exec", params.taskExecRoleARN)
	assert.Equal(t, 512, params.cpu)
	assert.Equal(t, 1024, params.memory)
	assert.Equal(t, arm64Platform, params.runtimePlatform)
	assert.True(t, params.isDefault)
}

func TestCompareTags(t *testing.T) {
	t.Run("matches expected and standard tags", func(t *testing.T) {
		m := &Manager{}
		expected := ecsdefs.BuildTaskDefinitionTags("alpine:latest", awsStd.Bool(true))

		current := append([]ecsTypes.Tag{}, expected...)
		current = append(current, ecsTypes.Tag{Key: awsStd.String("extra"), Value: awsStd.String("value")})

		assert.True(t, m.compareTags(current, expected))
	})

	t.Run("detects mismatched tags", func(t *testing.T) {
		m := &Manager{}
		expected := ecsdefs.BuildTaskDefinitionTags("alpine:latest", nil)

		// Copy expected tags but alter a standard tag value
		current := append([]ecsTypes.Tag{}, expected...)
		current[1].Value = awsStd.String("wrong")

		assert.False(t, m.compareTags(current, expected))
	})
}
