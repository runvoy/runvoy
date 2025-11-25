package health

import (
	"context"
	"testing"

	"runvoy/internal/providers/aws/constants"
	"runvoy/internal/testutil"

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
					"arn:aws:ecs:us-east-1:123456789012:task-definition/" + constants.TaskDefinitionFamilyPrefix + "-kept:1",
					"arn:aws:ecs:us-east-1:123456789012:task-definition/" + constants.TaskDefinitionFamilyPrefix + "-orphan:3",
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
		constants.TaskDefinitionFamilyPrefix + "-kept": true,
	}

	orphaned, err := m.findOrphanedTaskDefinitions(context.Background(), seen, testutil.SilentLogger())

	assert.NoError(t, err)
	assert.Equal(t, []string{constants.TaskDefinitionFamilyPrefix + "-orphan"}, orphaned)
}
