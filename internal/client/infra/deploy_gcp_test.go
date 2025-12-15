package infra

import (
	"context"
	"testing"

	resourcemanagerpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGCPDeployer_DeployRequiresProjectID(t *testing.T) {
	t.Helper()

	deployer := NewGCPDeployerWithClient(nil, "us-central1")

	_, err := deployer.Deploy(ctxWithNoop(), &DeployOptions{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "project ID is required for GCP")
}

func TestGCPDeployer_AddProjectInfoToOutputs(t *testing.T) {
	t.Helper()

	deployer := NewGCPDeployerWithClient(nil, "us-central1")
	outputs := make(map[string]string)

	project := &resourcemanagerpb.Project{
		ProjectId: "test-project-id",
		Name:      "projects/123456789",
	}

	deployer.addProjectInfoToOutputs(outputs, project)

	assert.Equal(t, "test-project-id", outputs["ProjectID"])
	assert.Equal(t, "projects/123456789", outputs["ProjectName"])
	assert.Equal(t, "123456789", outputs["ProjectNumber"])
}

// ctxWithNoop returns a background context for tests that do not depend on cancellation.
func ctxWithNoop() context.Context {
	return context.Background()
}
