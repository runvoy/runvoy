// Package gcp provides GCP infrastructure deployment via Deployment Manager.
package gcp

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"path"
	"strings"
	"time"

	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"google.golang.org/api/deploymentmanager/v2"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"

	"github.com/runvoy/runvoy/internal/client/infra/core"
	"github.com/runvoy/runvoy/internal/providers/gcp/constants"
)

// ResourceConfig contains configuration for GCP backend resources.
type ResourceConfig struct {
	ProjectID           string
	Region              string
	VPCName             string
	SubnetName          string
	VPCConnectorName    string
	UseDirectVPCEgress  bool
	VPCCIDRRange        string
	FirestoreLocationID string
	OrchestratorImage   string
	EventProcessorImage string
	MaxInstances        int
	MinInstances        int
	TimeoutSeconds      int
	TaskEventsTopic     string
	LogEventsTopic      string
	ProcessorSub        string
	KeyRingName         string
	CryptoKeyName       string
	HealthSchedule      string
	LogRetentionDays    int
	Labels              map[string]string
}

// DefaultResourceConfig returns a ResourceConfig with default values.
func DefaultResourceConfig(projectID, region string) *ResourceConfig {
	return &ResourceConfig{
		ProjectID:           projectID,
		Region:              region,
		VPCName:             constants.VPCName,
		SubnetName:          constants.SubnetName,
		VPCConnectorName:    constants.VPCConnectorName,
		UseDirectVPCEgress:  true,
		VPCCIDRRange:        constants.VPCCIDRRange,
		FirestoreLocationID: constants.FirestoreLocationID,
		MaxInstances:        constants.DefaultMaxInstances,
		MinInstances:        constants.DefaultMinInstances,
		TimeoutSeconds:      constants.DefaultTimeoutSeconds,
		TaskEventsTopic:     constants.TopicTaskEvents,
		LogEventsTopic:      constants.TopicLogEvents,
		ProcessorSub:        constants.SubscriptionProcessor,
		KeyRingName:         constants.KeyRingName,
		CryptoKeyName:       constants.CryptoKeyName,
		HealthSchedule:      constants.HealthReconcileSchedule,
		LogRetentionDays:    constants.DefaultLogRetentionDays,
		Labels: map[string]string{
			"managed-by":  "runvoy",
			"application": constants.ResourcePrefix,
		},
	}
}

// BackendResources holds references to created GCP resources.
type BackendResources struct {
	ProjectID                    string
	ProjectNumber                string
	Region                       string
	VPCName                      string
	SubnetName                   string
	VPCConnectorName             string
	FirestoreDatabase            string
	OrchestratorURL              string
	EventProcessorURL            string
	TaskEventsTopicName          string
	LogEventsTopicName           string
	SubscriptionName             string
	KeyRingName                  string
	CryptoKeyName                string
	CryptoKeyID                  string
	HealthReconcileJobName       string
	OrchestratorServiceAccount   string
	EventProcessorServiceAccount string
	RunnerServiceAccount         string
	ArtifactRegistryRepo         string
	WebSocketEndpoint            string
}

// ServiceClients holds GCP API clients needed for resource management.
type ServiceClients struct {
	ServiceUsage      ServiceUsageClient
	DeploymentManager DeploymentManagerClient
}

// ServiceUsageClient abstracts the Service Usage API.
type ServiceUsageClient interface {
	EnableServices(ctx context.Context, projectID string, services []string) error
}

// DeploymentManagerClient abstracts Deployment Manager operations.
type DeploymentManagerClient interface {
	GetDeployment(ctx context.Context, projectID, deploymentName string) (*deploymentmanager.Deployment, error)
	CreateDeployment(
		ctx context.Context,
		projectID string,
		deployment *deploymentmanager.Deployment,
	) (*deploymentmanager.Operation, error)
	UpdateDeployment(
		ctx context.Context,
		projectID, deploymentName string,
		deployment *deploymentmanager.Deployment,
	) (*deploymentmanager.Operation, error)
	DeleteDeployment(ctx context.Context, projectID, deploymentName string) (*deploymentmanager.Operation, error)
	GetOperation(ctx context.Context, projectID, operationName string) (*deploymentmanager.Operation, error)
	GetManifest(ctx context.Context, projectID, deploymentName, manifestName string) (*deploymentmanager.Manifest, error)
}

// Deployer implements Deployer for GCP Resource Manager.
type Deployer struct {
	client   *resourcemanager.ProjectsClient
	region   string
	services *ServiceClients
}

// NewDeployer creates a new GCP deployer with the given region.
func NewDeployer(ctx context.Context, region string) (*Deployer, error) {
	client, err := resourcemanager.NewProjectsClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP projects client: %w", err)
	}

	if region == "" {
		region = constants.DefaultRegion
	}

	serviceClients, err := newDefaultServiceClients(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCP service clients: %w", err)
	}

	return &Deployer{
		client:   client,
		region:   region,
		services: serviceClients,
	}, nil
}

// NewDeployerWithClient creates a new GCP deployer with a custom client.
func NewDeployerWithClient(client *resourcemanager.ProjectsClient, region string) *Deployer {
	return &Deployer{
		client: client,
		region: region,
	}
}

// GetRegion returns the GCP region being used.
func (d *Deployer) GetRegion() string {
	return d.region
}

// Deploy creates or updates a GCP project and applies the Deployment Manager config.
func (d *Deployer) Deploy(ctx context.Context, opts *core.DeployOptions) (*core.DeployResult, error) {
	if opts.Name == "" {
		return nil, errors.New("project ID is required for GCP")
	}

	projectID := opts.Name
	result := &core.DeployResult{
		Name:    projectID,
		Outputs: make(map[string]string),
	}

	exists, err := d.checkProjectExists(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if project exists: %w", err)
	}

	result.OperationType = core.OperationTypeUpdate
	result.Status = core.StatusUpdateComplete
	result.NoChanges = false

	var createdProject *resourcemanagerpb.Project

	projectReady, ensureErr := d.ensureProject(ctx, opts, exists, result)
	if ensureErr != nil {
		return result, ensureErr
	}
	createdProject = projectReady

	if result.Status == core.StatusInProgress {
		return result, nil
	}

	if apiErr := d.ensureAPIs(ctx, projectID); apiErr != nil {
		return result, fmt.Errorf("failed to enable required APIs: %w", apiErr)
	}

	applyErr := d.applyBackend(ctx, projectID, opts)
	if applyErr != nil {
		return result, applyErr
	}

	outputs, outputsErr := d.collectOutputs(ctx, projectID, createdProject)
	if outputsErr != nil {
		return result, outputsErr
	}
	result.Outputs = outputs

	return result, nil
}

func (d *Deployer) collectOutputs(
	ctx context.Context,
	projectID string,
	createdProject *resourcemanagerpb.Project,
) (map[string]string, error) {
	deploymentOutputs, err := d.getDeploymentOutputs(ctx, projectID)
	if err != nil {
		return nil, err
	}

	projectOutputs, err := d.getProjectOutputs(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve project outputs: %w", err)
	}

	outputs := make(map[string]string)
	maps.Copy(outputs, deploymentOutputs)
	maps.Copy(outputs, projectOutputs)

	if createdProject != nil {
		d.addProjectInfoToOutputs(outputs, createdProject)
	}

	return outputs, nil
}

func (d *Deployer) ensureProject(
	ctx context.Context,
	opts *core.DeployOptions,
	exists bool,
	result *core.DeployResult,
) (*resourcemanagerpb.Project, error) {
	if exists {
		return nil, nil
	}

	result.OperationType = core.OperationTypeCreate

	if !opts.Wait {
		if startErr := d.startProjectCreation(ctx, opts.Name, opts); startErr != nil {
			return nil, fmt.Errorf("failed to create project: %w", startErr)
		}

		result.Status = core.StatusInProgress
		return nil, nil
	}

	project, createErr := d.createNewProject(ctx, opts.Name, opts)
	if createErr != nil {
		return nil, fmt.Errorf("failed to create project: %w", createErr)
	}

	if waitErr := d.waitForProjectReady(ctx, opts.Name); waitErr != nil {
		return nil, fmt.Errorf("project creation failed: %w", waitErr)
	}

	result.Status = core.StatusCreateComplete

	return project, nil
}

func (d *Deployer) startProjectCreation(
	ctx context.Context,
	projectID string,
	opts *core.DeployOptions,
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

func (d *Deployer) createNewProject(
	ctx context.Context,
	projectID string,
	opts *core.DeployOptions,
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

func (d *Deployer) addProjectInfoToOutputs(
	outputs map[string]string,
	project *resourcemanagerpb.Project,
) {
	outputs["ProjectID"] = project.ProjectId
	outputs["ProjectName"] = project.Name
	if project.Name != "" {
		outputs["ProjectNumber"] = strings.TrimPrefix(project.Name, "projects/")
	}
}

func (d *Deployer) createProject(
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

func (d *Deployer) waitForProjectReady(ctx context.Context, projectID string) error {
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
			exists, err := d.checkProjectExists(ctx, projectID)
			if err != nil {
				return fmt.Errorf("failed to check project status: %w", err)
			}
			if exists {
				return nil
			}
		}
	}
}

func (d *Deployer) ensureAPIs(ctx context.Context, projectID string) error {
	if d.services == nil || d.services.ServiceUsage == nil {
		return errors.New("service clients not initialized; call SetServiceClients first")
	}

	if err := d.services.ServiceUsage.EnableServices(ctx, projectID, constants.RequiredServices); err != nil {
		return fmt.Errorf("enable services: %w", err)
	}
	return nil
}

// CheckExists checks if a GCP project exists.
func (d *Deployer) CheckExists(ctx context.Context, name string) (bool, error) {
	return d.checkProjectExists(ctx, name)
}

// checkProjectExists is the internal implementation for checking project existence.
func (d *Deployer) checkProjectExists(ctx context.Context, projectID string) (bool, error) {
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
			if strings.Contains(err.Error(), "or it may not exist") {
				return false, nil
			}
		}

		return false, fmt.Errorf("failed to get project: %w", err)
	}

	return true, nil
}

// GetOutputs retrieves outputs from a GCP project deployment.
func (d *Deployer) GetOutputs(ctx context.Context, name string) (map[string]string, error) {
	outputs, err := d.getDeploymentOutputs(ctx, name)
	if err != nil {
		return nil, err
	}

	projectOutputs, outputsErr := d.getProjectOutputs(ctx, name)
	if outputsErr != nil {
		return nil, outputsErr
	}

	maps.Copy(outputs, projectOutputs)

	return outputs, nil
}

func (d *Deployer) getDeploymentOutputs(ctx context.Context, projectID string) (map[string]string, error) {
	if d.services == nil || d.services.DeploymentManager == nil {
		return nil, errors.New("service clients not initialized; call SetServiceClients first")
	}

	deployment, err := d.services.DeploymentManager.GetDeployment(ctx, projectID, runvoyDeploymentName)
	if err != nil {
		if isNotFound(err) {
			return nil, errors.New("deployment not found")
		}
		return nil, fmt.Errorf("failed to get deployment outputs: %w", err)
	}

	if deployment.Manifest == "" {
		return nil, errors.New("deployment manifest is empty")
	}

	manifestName, err := manifestNameFromSelfLink(deployment.Manifest)
	if err != nil {
		return nil, err
	}

	manifest, err := d.services.DeploymentManager.GetManifest(
		ctx,
		projectID,
		runvoyDeploymentName,
		manifestName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment manifest: %w", err)
	}

	return parseManifestOutputs(manifest)
}

// getProjectOutputs is the internal implementation for retrieving project outputs.
func (d *Deployer) getProjectOutputs(
	ctx context.Context,
	projectID string,
) (map[string]string, error) {
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
		parts := strings.Split(project.Name, "/")
		if len(parts) == expectedPartsCount {
			outputs["ProjectNumber"] = parts[1]
		}
	}

	return outputs, nil
}

// Destroy deletes a GCP project.
func (d *Deployer) Destroy(
	ctx context.Context,
	opts *core.DestroyOptions,
) (*core.DestroyResult, error) {
	projectID := opts.Name
	result := &core.DestroyResult{
		Name: projectID,
	}

	exists, err := d.checkProjectExists(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to check project status: %w", err)
	}

	if !exists {
		result.NotFound = true
		result.Status = core.StatusNotFound
		return result, nil
	}

	op, err := d.deleteProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete project: %w", err)
	}

	if !opts.Wait {
		result.Status = core.StatusInProgress
		return result, nil
	}

	if waitErr := d.waitForProjectDeletion(ctx, op); waitErr != nil {
		return nil, fmt.Errorf("project deletion failed: %w", waitErr)
	}

	result.Status = constants.StatusDeleteRequested

	return result, nil
}

func (d *Deployer) deleteProject(
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

func (d *Deployer) waitForProjectDeletion(
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

// SetServiceClients sets the GCP service clients for resource management.
func (d *Deployer) SetServiceClients(clients *ServiceClients) {
	d.services = clients
}

// applyBackend deploys backend resources and fails if service clients are not configured.
func (d *Deployer) applyBackend(
	ctx context.Context,
	projectID string,
	opts *core.DeployOptions,
) error {
	if d.services == nil {
		return errors.New("service clients not initialized; call SetServiceClients first")
	}

	params, err := core.ParseParameters(opts.Parameters)
	if err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
	}

	config := DefaultResourceConfig(projectID, d.region)

	if img, ok := params["OrchestratorImage"]; ok {
		config.OrchestratorImage = img
	}
	if img, ok := params["EventProcessorImage"]; ok {
		config.EventProcessorImage = img
	}
	if loc, ok := params["FirestoreLocationID"]; ok {
		config.FirestoreLocationID = loc
	}
	if maxInst, ok := params["MaxInstances"]; ok {
		parsed, parseErr := parseInt(maxInst)
		if parseErr != nil {
			return parseErr
		}
		config.MaxInstances = parsed
	}
	if minInst, ok := params["MinInstances"]; ok {
		parsed, parseErr := parseInt(minInst)
		if parseErr != nil {
			return parseErr
		}
		config.MinInstances = parsed
	}

	_, deployErr := d.DeployBackend(ctx, config)
	return deployErr
}

// parseInt is a helper to parse integer strings.
func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	if err != nil {
		return 0, fmt.Errorf("failed to parse integer from %q: %w", s, err)
	}
	return result, nil
}

// DeployBackend applies the Deployment Manager configuration for backend resources.
func (d *Deployer) DeployBackend(
	ctx context.Context,
	config *ResourceConfig,
) (*BackendResources, error) {
	if d.services == nil || d.services.DeploymentManager == nil {
		return nil, errors.New("service clients not initialized; call SetServiceClients first")
	}

	op, err := d.applyDeployment(ctx, config)
	if err != nil {
		return nil, err
	}

	if op != nil {
		if waitErr := d.waitForDeploymentOperation(ctx, config.ProjectID, op.Name); waitErr != nil {
			return nil, waitErr
		}
	}

	return d.GetBackendResources(ctx, config)
}

func (d *Deployer) applyDeployment(
	ctx context.Context,
	config *ResourceConfig,
) (*deploymentmanager.Operation, error) {
	target, err := buildDeploymentTarget(config)
	if err != nil {
		return nil, err
	}

	deployment := &deploymentmanager.Deployment{
		Name:   runvoyDeploymentName,
		Target: target,
	}

	existing, err := d.services.DeploymentManager.GetDeployment(ctx, config.ProjectID, runvoyDeploymentName)
	if err != nil {
		if isNotFound(err) {
			op, createErr := d.services.DeploymentManager.CreateDeployment(ctx, config.ProjectID, deployment)
			if createErr != nil {
				return nil, fmt.Errorf("create deployment: %w", createErr)
			}
			return op, nil
		}
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	deployment.Fingerprint = existing.Fingerprint
	op, updateErr := d.services.DeploymentManager.UpdateDeployment(
		ctx,
		config.ProjectID,
		runvoyDeploymentName,
		deployment,
	)
	if updateErr != nil {
		return nil, fmt.Errorf("update deployment: %w", updateErr)
	}
	return op, nil
}

func (d *Deployer) waitForDeploymentOperation(
	ctx context.Context,
	projectID, operationName string,
) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, constants.DeploymentOperationTimeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return errors.New("timeout waiting for deployment operation")
		default:
		}

		op, err := d.services.DeploymentManager.GetOperation(timeoutCtx, projectID, operationName)
		if err != nil {
			return fmt.Errorf("failed to get deployment operation: %w", err)
		}
		if strings.EqualFold(op.Status, "DONE") {
			return deploymentOperationError(op)
		}
		time.Sleep(constants.ResourcePollInterval)
	}
}

func deploymentOperationError(op *deploymentmanager.Operation) error {
	if op == nil || op.Error == nil || len(op.Error.Errors) == 0 {
		return nil
	}

	messages := make([]string, 0, len(op.Error.Errors))
	for _, err := range op.Error.Errors {
		if err == nil {
			continue
		}
		if err.Message != "" {
			messages = append(messages, err.Message)
		} else {
			messages = append(messages, fmt.Sprintf("code %v", err.Code))
		}
	}

	if len(messages) == 0 {
		return errors.New("deployment operation failed")
	}

	return fmt.Errorf("deployment operation failed: %s", strings.Join(messages, "; "))
}

// DestroyBackend deletes the Deployment Manager deployment.
func (d *Deployer) DestroyBackend(ctx context.Context, config *ResourceConfig) error {
	if d.services == nil || d.services.DeploymentManager == nil {
		return errors.New("service clients not initialized; call SetServiceClients first")
	}

	op, err := d.services.DeploymentManager.DeleteDeployment(ctx, config.ProjectID, runvoyDeploymentName)
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	if op == nil {
		return nil
	}

	return d.waitForDeploymentOperation(ctx, config.ProjectID, op.Name)
}

// GetBackendResources retrieves information about deployed GCP backend resources.
func (d *Deployer) GetBackendResources(
	ctx context.Context,
	config *ResourceConfig,
) (*BackendResources, error) {
	outputs, err := d.getDeploymentOutputs(ctx, config.ProjectID)
	if err != nil {
		return nil, err
	}

	resources := &BackendResources{
		ProjectID: config.ProjectID,
		Region:    config.Region,
	}

	resources.TaskEventsTopicName = outputs["TaskEventsTopic"]
	resources.LogEventsTopicName = outputs["LogEventsTopic"]
	resources.CryptoKeyID = outputs["CryptoKeyID"]
	resources.OrchestratorServiceAccount = outputs["OrchestratorServiceAccount"]
	resources.EventProcessorServiceAccount = outputs["EventProcessorServiceAccount"]
	resources.RunnerServiceAccount = outputs["RunnerServiceAccount"]
	resources.ArtifactRegistryRepo = outputs["ArtifactRegistryRepo"]
	resources.OrchestratorURL = outputs["OrchestratorURL"]
	resources.EventProcessorURL = outputs["EventProcessorURL"]
	resources.WebSocketEndpoint = outputs["WebSocketEndpoint"]

	return resources, nil
}

// BackendResourcesExist checks if GCP backend resources exist.
func (d *Deployer) BackendResourcesExist(
	ctx context.Context,
	config *ResourceConfig,
) (bool, error) {
	if d.services == nil || d.services.DeploymentManager == nil {
		return false, errors.New("service clients not initialized; call SetServiceClients first")
	}

	_, err := d.services.DeploymentManager.GetDeployment(ctx, config.ProjectID, runvoyDeploymentName)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check deployment: %w", err)
	}

	return true, nil
}

type manifestLayout struct {
	Outputs []manifestOutput `yaml:"outputs"`
}

type manifestOutput struct {
	Name  string `yaml:"name"`
	Value any    `yaml:"value"`
}

func parseManifestOutputs(manifest *deploymentmanager.Manifest) (map[string]string, error) {
	if manifest == nil {
		return nil, errors.New("manifest is required")
	}

	layout := manifest.Layout
	if layout == "" {
		layout = manifest.ExpandedConfig
	}
	if layout == "" {
		return nil, errors.New("manifest layout is empty")
	}

	var parsed manifestLayout
	if err := yaml.Unmarshal([]byte(layout), &parsed); err != nil {
		return nil, fmt.Errorf("parse manifest layout: %w", err)
	}

	outputs := make(map[string]string)
	for _, out := range parsed.Outputs {
		if out.Name == "" {
			continue
		}
		switch value := out.Value.(type) {
		case string:
			outputs[out.Name] = value
		default:
			outputs[out.Name] = fmt.Sprint(out.Value)
		}
	}

	return outputs, nil
}

func manifestNameFromSelfLink(selfLink string) (string, error) {
	if selfLink == "" {
		return "", errors.New("manifest self link is empty")
	}

	name := path.Base(selfLink)
	if name == "." || name == "/" || name == "" {
		return "", fmt.Errorf("invalid manifest self link: %s", selfLink)
	}

	return name, nil
}

func isNotFound(err error) bool {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == http.StatusNotFound
	}
	return false
}
