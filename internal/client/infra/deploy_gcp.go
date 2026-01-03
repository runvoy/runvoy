package infra

import (
	"context"
	"fmt"
	"os"

	"github.com/runvoy/runvoy/internal/constants"
	"github.com/runvoy/runvoy/internal/infra/gcp"
)

type GCPDeployer struct {
	project string
	region  string
}

func NewGCPDeployer(ctx context.Context, region string) (*GCPDeployer, error) {
	return &GCPDeployer{
		project: "",
		region:  region,
	}, nil
}

func (d *GCPDeployer) GetRegion() string {
	return d.region
}

func (d *GCPDeployer) Deploy(ctx context.Context, opts *DeployOptions) (*DeployResult, error) {
	projectID := os.Getenv("RUNVOY_GCP_PROJECT_ID")
	if projectID == "" {
		projectID = constants.GCPDefaultProjectName
	}

	executor, err := gcp.NewExecutor(ctx, projectID, d.region)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize executor: %w", err)
	}

	resourceFile, err := gcp.ParseResourceFile(opts.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to parse resources: %w", err)
	}

	gcpResults, err := executor.Apply(ctx, resourceFile)
	if err != nil {
		return nil, err
	}

	outputs := make(map[string]string)
	outputs["ProjectID"] = projectID
	if len(results) > 0 && results[0].ResourceID != "" {
		outputs["PrimaryResource"] = results[0].ResourceID
	}

	return &DeployResult{
		StackName: projectID,
		Outputs:   outputs,
		Status:    "CREATE_COMPLETE",
		NoChanges: false,
	}, nil
}

func (d *GCPDeployer) Destroy(ctx context.Context, opts *DestroyOptions) (*DestroyResult, error) {
	projectID := os.Getenv("RUNVOY_GCP_PROJECT_ID")
	if projectID == "" {
		projectID = constants.GCPDefaultProjectName
	}

	if !opts.Force {
		return nil, fmt.Errorf(
			"deleting entire GCP project requires confirmation. " +
				"Use --force flag to proceed: runvoy infra destroy --provider gcp --force")
	}

	executor, err := gcp.NewExecutor(ctx, projectID, d.region)
	if err != nil {
		return nil, err
	}

	err = executor.Destroy(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to destroy project: %w", err)
	}

	return &DestroyResult{
		StackName: projectID,
		Status:    "DELETE_COMPLETE",
		NotFound:  false,
	}, nil
}

func (d *GCPDeployer) CheckStackExists(ctx context.Context, stackName string) (bool, error) {
	return true, nil
}

func (d *GCPDeployer) GetStackOutputs(ctx context.Context, stackName string) (map[string]string, error) {
	outputs := make(map[string]string)
	outputs["ProjectID"] = stackName
	return outputs, nil
}
