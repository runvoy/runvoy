package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/runvoy/runvoy/internal/providers/gcp/constants"
)

// GCPDeployer implements Deployer for GCP Resource Manager.
type GCPDeployer struct {
	client *resourcemanager.ProjectsClient
	region string
}

// NewGCPDeployer creates a new GCP deployer with the given region.
// Authentication is handled via Application Default Credentials (ADC).
func NewGCPDeployer(ctx context.Context, region string) (*GCPDeployer, error) {
	client, err := resourcemanager.NewProjectsClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP projects client: %w", err)
	}

	if region == "" {
		region = constants.DefaultRegion
	}

	return &GCPDeployer{
		client: client,
		region: region,
	}, nil
}

// NewGCPDeployerWithClient creates a new GCP deployer with a custom client (for testing).
func NewGCPDeployerWithClient(client *resourcemanager.ProjectsClient, region string) *GCPDeployer {
	return &GCPDeployer{
		client: client,
		region: region,
	}
}

// GetRegion returns the GCP region being used.
func (d *GCPDeployer) GetRegion() string {
	return d.region
}

// Deploy creates a new GCP project.
// For GCP, the deployment name is used as the project ID.
func (d *GCPDeployer) Deploy(ctx context.Context, opts *DeployOptions) (*DeployResult, error) {
	if opts.StackName == "" {
		return nil, errors.New("project ID is required for GCP")
	}

	projectID := opts.StackName
	result := &DeployResult{
		StackName: projectID,
		Outputs:   make(map[string]string),
	}

	exists, err := d.CheckStackExists(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if project exists: %w", err)
	}

	if exists {
		return d.handleExistingProject(ctx, projectID, result)
	}

	return d.handleNewProject(ctx, projectID, opts, result)
}

// handleExistingProject handles the case where a project already exists.
func (d *GCPDeployer) handleExistingProject(
	ctx context.Context,
	projectID string,
	result *DeployResult,
) (*DeployResult, error) {
	result.OperationType = operationTypeUpdate
	result.Status = statusUpdateComplete
	result.NoChanges = true
	existingOutputs, getErr := d.GetStackOutputs(ctx, projectID)
	if getErr != nil {
		return result, fmt.Errorf("failed to retrieve project outputs: %w", getErr)
	}
	result.Outputs = existingOutputs
	return result, nil
}

// handleNewProject handles the creation of a new GCP project.
func (d *GCPDeployer) handleNewProject(
	ctx context.Context,
	projectID string,
	opts *DeployOptions,
	result *DeployResult,
) (*DeployResult, error) {
	result.OperationType = operationTypeCreate

	if !opts.Wait {
		if err := d.startProjectCreation(ctx, projectID, opts); err != nil {
			return nil, fmt.Errorf("failed to create project: %w", err)
		}

		result.Status = statusInProgress
		return result, nil
	}

	createdProject, err := d.createNewProject(ctx, projectID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	if waitErr := d.waitForProjectReady(ctx, projectID); waitErr != nil {
		return nil, fmt.Errorf("project creation failed: %w", waitErr)
	}

	result.Status = statusCreateComplete

	projectOutputs, getErr := d.GetStackOutputs(ctx, projectID)
	if getErr != nil {
		return result, fmt.Errorf("project created but failed to retrieve outputs: %w", getErr)
	}
	result.Outputs = projectOutputs

	if createdProject != nil {
		d.addProjectInfoToOutputs(result.Outputs, createdProject)
	}

	return result, nil
}

// startProjectCreation initiates a new GCP project creation without waiting
// for the long-running operation to complete.
func (d *GCPDeployer) startProjectCreation(
	ctx context.Context,
	projectID string,
	opts *DeployOptions,
) error {
	project := &resourcemanagerpb.Project{
		ProjectId: projectID,
	}

	if opts.OrgID != "" {
		project.Parent = "organizations/" + opts.OrgID
	}

	req := &resourcemanagerpb.CreateProjectRequest{
		Project: project,
	}

	if _, err := d.client.CreateProject(ctx, req); err != nil {
		return fmt.Errorf("failed to initiate project creation: %w", err)
	}

	return nil
}

// createNewProject creates a new GCP project with the given options and waits
// for the long-running operation to complete.
func (d *GCPDeployer) createNewProject(
	ctx context.Context,
	projectID string,
	opts *DeployOptions,
) (*resourcemanagerpb.Project, error) {
	project := &resourcemanagerpb.Project{
		ProjectId: projectID,
	}

	if opts.OrgID != "" {
		project.Parent = "organizations/" + opts.OrgID
	}

	req := &resourcemanagerpb.CreateProjectRequest{
		Project: project,
	}

	return d.createProject(ctx, req)
}

// addProjectInfoToOutputs adds project information to the outputs map.
func (d *GCPDeployer) addProjectInfoToOutputs(
	outputs map[string]string,
	project *resourcemanagerpb.Project,
) {
	outputs["ProjectID"] = project.ProjectId
	outputs["ProjectName"] = project.Name
	if project.Name != "" {
		outputs["ProjectNumber"] = strings.TrimPrefix(project.Name, "projects/")
	}
}

// createProject creates a new GCP project.
func (d *GCPDeployer) createProject(
	ctx context.Context,
	req *resourcemanagerpb.CreateProjectRequest,
) (*resourcemanagerpb.Project, error) {
	op, err := d.client.CreateProject(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	project, err := op.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for project creation: %w", err)
	}

	return project, nil
}

// waitForProjectReady waits for a project to be ready.
func (d *GCPDeployer) waitForProjectReady(ctx context.Context, projectID string) error {
	ticker := time.NewTicker(constants.ProjectPollInterval)
	defer ticker.Stop()

	timeout := time.After(constants.ProjectOperationTimeout)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled: %w", ctx.Err())
		case <-timeout:
			return errors.New("timeout waiting for project creation")
		case <-ticker.C:
			exists, err := d.CheckStackExists(ctx, projectID)
			if err != nil {
				return fmt.Errorf("failed to check project status: %w", err)
			}
			if exists {
				return nil
			}
		}
	}
}

// CheckStackExists checks if a GCP project exists.
func (d *GCPDeployer) CheckStackExists(ctx context.Context, projectID string) (bool, error) {
	req := &resourcemanagerpb.GetProjectRequest{
		Name: "projects/" + projectID,
	}

	_, err := d.client.GetProject(ctx, req)
	if err != nil {
		//nolint:exhaustive // only handling NotFound and PermissionDenied specifically
		switch status.Code(err) {
		case codes.NotFound:
			return false, nil
		case codes.PermissionDenied:
			// Common case when caller can create projects but not read non-existent ones:
			// "Permission 'resourcemanager.projects.get' denied ... (or it may not exist)".
			// Treat this as "does not exist" so that creation can proceed.
			if strings.Contains(err.Error(), "or it may not exist") {
				return false, nil
			}
		}

		return false, fmt.Errorf("failed to get project: %w", err)
	}

	return true, nil
}

// GetStackOutputs retrieves outputs from a GCP project.
func (d *GCPDeployer) GetStackOutputs(ctx context.Context, projectID string) (map[string]string, error) {
	req := &resourcemanagerpb.GetProjectRequest{
		Name: "projects/" + projectID,
	}

	project, err := d.client.GetProject(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	outputs := make(map[string]string)
	outputs["ProjectID"] = project.ProjectId
	if project.DisplayName != "" {
		outputs["ProjectName"] = project.DisplayName
	} else {
		outputs["ProjectName"] = project.ProjectId
	}
	const expectedPartsCount = 2
	if project.Name != "" {
		// Extract project number from name (format: "projects/123456789")
		parts := strings.Split(project.Name, "/")
		if len(parts) == expectedPartsCount {
			outputs["ProjectNumber"] = parts[1]
		}
	}

	return outputs, nil
}

// Destroy deletes a GCP project.
// Note: GCP projects are scheduled for deletion and enter DELETE_REQUESTED state.
// After 30 days, they are permanently deleted unless restored.
func (d *GCPDeployer) Destroy(ctx context.Context, opts *DestroyOptions) (*DestroyResult, error) {
	result := &DestroyResult{
		StackName: opts.StackName,
	}

	exists, err := d.CheckStackExists(ctx, opts.StackName)
	if err != nil {
		return nil, fmt.Errorf("failed to check project status: %w", err)
	}

	if !exists {
		result.NotFound = true
		result.Status = statusNotFound
		return result, nil
	}

	op, err := d.deleteProject(ctx, opts.StackName)
	if err != nil {
		return nil, fmt.Errorf("failed to delete project: %w", err)
	}

	if !opts.Wait {
		result.Status = statusInProgress
		return result, nil
	}

	if waitErr := d.waitForProjectDeletion(ctx, op); waitErr != nil {
		return nil, fmt.Errorf("project deletion failed: %w", waitErr)
	}

	result.Status = constants.StatusDeleteRequested

	return result, nil
}

// deleteProject initiates deletion of a GCP project.
func (d *GCPDeployer) deleteProject(
	ctx context.Context,
	projectID string,
) (*resourcemanager.DeleteProjectOperation, error) {
	req := &resourcemanagerpb.DeleteProjectRequest{
		Name: "projects/" + projectID,
	}

	op, err := d.client.DeleteProject(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate project deletion: %w", err)
	}

	return op, nil
}

// waitForProjectDeletion waits for a project deletion operation to complete.
func (d *GCPDeployer) waitForProjectDeletion(
	ctx context.Context,
	op *resourcemanager.DeleteProjectOperation,
) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, constants.ProjectOperationTimeout)
	defer cancel()

	_, err := op.Wait(timeoutCtx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return errors.New("timeout waiting for project deletion")
		}
		return fmt.Errorf("error waiting for project deletion: %w", err)
	}

	return nil
}
