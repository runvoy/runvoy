package health

import (
	"context"
	"errors"
	"testing"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/testutil"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmTypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"
)

func TestCheckSecretParameter_MissingParameter(t *testing.T) {
	m := &Manager{
		ssmClient: &mockSSMClient{
			getParameterFunc: func(
				_ context.Context,
				_ *ssm.GetParameterInput,
				_ ...func(*ssm.Options),
			) (*ssm.GetParameterOutput, error) {
				return nil, errors.New("ParameterNotFound")
			},
		},
		cfg:    &Config{SecretsPrefix: "/runvoy"},
		logger: testutil.SilentLogger(),
	}

	status := api.SecretsHealthStatus{}
	issues := m.checkSecretParameter(
		context.Background(),
		"/runvoy/db-password",
		"db-password",
		testutil.SilentLogger(),
		&status,
	)

	assert.Equal(t, 1, status.MissingCount)
	assert.Len(t, issues, 1)
	assert.Equal(t, "requires_manual_intervention", issues[0].Action)
}

func TestCheckSecretParameter_TagUpdate(t *testing.T) {
	addTagsCalled := false
	m := &Manager{
		ssmClient: &mockSSMClient{
			getParameterFunc: func(
				_ context.Context,
				_ *ssm.GetParameterInput,
				_ ...func(*ssm.Options),
			) (*ssm.GetParameterOutput, error) {
				return &ssm.GetParameterOutput{}, nil
			},
			listTagsForResourceFunc: func(
				_ context.Context,
				_ *ssm.ListTagsForResourceInput,
				_ ...func(*ssm.Options),
			) (*ssm.ListTagsForResourceOutput, error) {
				// Return mismatched tags to trigger an update
				return &ssm.ListTagsForResourceOutput{
					TagList: []ssmTypes.Tag{{Key: stringPtr("env"), Value: stringPtr("old")}},
				}, nil
			},
			addTagsToResourceFunc: func(
				_ context.Context,
				_ *ssm.AddTagsToResourceInput,
				_ ...func(*ssm.Options),
			) (*ssm.AddTagsToResourceOutput, error) {
				addTagsCalled = true
				return &ssm.AddTagsToResourceOutput{}, nil
			},
		},
		cfg:           &Config{SecretsPrefix: "/runvoy"},
		secretsPrefix: "/runvoy",
		logger:        testutil.SilentLogger(),
	}

	status := api.SecretsHealthStatus{}
	issues := m.checkSecretParameter(context.Background(), "/runvoy/api-key", "api-key", testutil.SilentLogger(), &status)

	assert.True(t, addTagsCalled, "expected tags to be updated")
	assert.Equal(t, 1, status.TagUpdatedCount)
	assert.Len(t, issues, 1)
	assert.Equal(t, "tag_updated", issues[0].Action)
}
