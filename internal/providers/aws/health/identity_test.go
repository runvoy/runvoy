package health

import (
	"context"
	"errors"
	"testing"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/constants"
	"github.com/runvoy/runvoy/internal/testutil"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/stretchr/testify/assert"
)

func TestVerifyRole_MissingDefaultRole(t *testing.T) {
	m := &Manager{
		iamClient: &mockIAMClient{
			getRoleFunc: func(_ context.Context, input *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
				assert.Equal(t, "missing-role", *input.RoleName)
				return nil, errors.New("NoSuchEntity")
			},
		},
		cfg: &Config{
			DefaultTaskRoleARN: "arn:aws:iam::123456789012:role/missing-role",
		},
		logger: testutil.SilentLogger(),
	}

	status := api.IdentityHealthStatus{}
	issues := m.verifyRole(context.Background(), m.cfg.DefaultTaskRoleARN, "Default task role", &status)

	assert.Len(t, issues, 1)
	assert.Contains(t, status.MissingRoles, m.cfg.DefaultTaskRoleARN)
	assert.Equal(t, "requires_manual_intervention", issues[0].Action)
}

func TestVerifyCustomRoles(t *testing.T) {
	iamCalls := 0
	m := &Manager{
		iamClient: &mockIAMClient{
			getRoleFunc: func(_ context.Context, input *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
				iamCalls++
				if *input.RoleName == "missing-role" {
					return nil, errors.New("NoSuchEntity")
				}
				return &iam.GetRoleOutput{}, nil
			},
		},
		imageRepo: &mockImageRepo{
			images: []api.ImageInfo{
				sampleImage("missing-role", "present-role"),
			},
		},
		cfg: &Config{
			AccountID: "123456789012",
		},
		logger: testutil.SilentLogger(),
	}

	status := api.IdentityHealthStatus{}
	issues, err := m.verifyCustomRoles(context.Background(), &status)

	assert.NoError(t, err)
	assert.Equal(t, 2, status.CustomRolesTotal)
	assert.Equal(t, 1, status.CustomRolesVerified)
	assert.Len(t, status.MissingRoles, 1)
	assert.Equal(t, "arn:aws:iam::123456789012:role/missing-role", status.MissingRoles[0])
	assert.Len(t, issues, 1)
	assert.Equal(t, "requires_manual_intervention", issues[0].Action)
	assert.Equal(t, 2, iamCalls)
	assert.Equal(t, constants.AWS, constants.AWS) // keep package reference to avoid unused import
}
