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

	"github.com/runvoy/runvoy/internal/client/output"
	"github.com/runvoy/runvoy/internal/providers/gcp/constants"
)

// GCPResourceConfig contains configuration for GCP backend resources.
type GCPResourceConfig struct {
	ProjectID           string
	Region              string
	VPCName             string
	SubnetName          string
	VPCConnectorName    string
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

// DefaultGCPResourceConfig returns a GCPResourceConfig with default values.
func DefaultGCPResourceConfig(projectID, region string) *GCPResourceConfig {
	return &GCPResourceConfig{
		ProjectID:           projectID,
		Region:              region,
		VPCName:             constants.VPCName,
		SubnetName:          constants.SubnetName,
		VPCConnectorName:    constants.VPCConnectorName,
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

// GCPBackendResources holds references to created GCP resources.
type GCPBackendResources struct {
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

// GCPServiceClients holds GCP API clients needed for resource management.
type GCPServiceClients struct {
	Projects         ProjectsClient
	Firestore        FirestoreClient
	CloudRun         CloudRunClient
	PubSub           PubSubClient
	KMS              KMSClient
	Scheduler        SchedulerClient
	SecretManager    SecretManagerClient
	Compute          ComputeClient
	IAM              IAMClient
	Logging          LoggingClient
	ArtifactRegistry ArtifactRegistryClient
	VPCAccess        VPCAccessClient
}

// ProjectsClient abstracts GCP Resource Manager Projects API operations.
type ProjectsClient interface {
	GetProject(ctx context.Context, name string) (*resourcemanagerpb.Project, error)
	CreateProject(ctx context.Context, project *resourcemanagerpb.Project) error
	DeleteProject(ctx context.Context, name string) error
}

// FirestoreClient abstracts Firestore Admin API operations.
type FirestoreClient interface {
	CreateDatabase(ctx context.Context, projectID, locationID string) error
	GetDatabase(ctx context.Context, projectID string) (bool, error)
	CreateIndex(ctx context.Context, projectID, collectionID, fieldPath string) error
}

// CloudRunClient abstracts Cloud Run Admin API operations.
type CloudRunClient interface {
	CreateService(
		ctx context.Context,
		projectID, region, serviceName, image string,
		envVars map[string]string,
		serviceAccount string,
	) (string, error)
	UpdateService(
		ctx context.Context,
		projectID, region, serviceName, image string,
		envVars map[string]string,
	) error
	DeleteService(ctx context.Context, projectID, region, serviceName string) error
	GetService(ctx context.Context, projectID, region, serviceName string) (bool, string, error)
	SetIAMPolicy(
		ctx context.Context,
		projectID, region, serviceName string,
		allowUnauthenticated bool,
	) error
}

// PubSubClient abstracts Pub/Sub API operations.
type PubSubClient interface {
	CreateTopic(ctx context.Context, projectID, topicID string) error
	DeleteTopic(ctx context.Context, projectID, topicID string) error
	TopicExists(ctx context.Context, projectID, topicID string) (bool, error)
	CreateSubscription(
		ctx context.Context,
		projectID, subscriptionID, topicID, pushEndpoint string,
	) error
	DeleteSubscription(ctx context.Context, projectID, subscriptionID string) error
}

// KMSClient abstracts Cloud KMS API operations.
type KMSClient interface {
	CreateKeyRing(ctx context.Context, projectID, locationID, keyRingID string) error
	KeyRingExists(ctx context.Context, projectID, locationID, keyRingID string) (bool, error)
	CreateCryptoKey(
		ctx context.Context,
		projectID, locationID, keyRingID, cryptoKeyID string,
	) (string, error)
	CryptoKeyExists(
		ctx context.Context,
		projectID, locationID, keyRingID, cryptoKeyID string,
	) (bool, error)
	GetCryptoKeyID(
		ctx context.Context,
		projectID, locationID, keyRingID, cryptoKeyID string,
	) (string, error)
}

// SchedulerClient abstracts Cloud Scheduler API operations.
type SchedulerClient interface {
	CreateJob(
		ctx context.Context,
		projectID, region, jobID, schedule, targetURL, httpMethod string,
	) error
	DeleteJob(ctx context.Context, projectID, region, jobID string) error
	JobExists(ctx context.Context, projectID, region, jobID string) (bool, error)
}

// SecretManagerClient abstracts Secret Manager API operations.
type SecretManagerClient interface {
	CreateSecret(ctx context.Context, projectID, secretID string) error
	AddSecretVersion(ctx context.Context, projectID, secretID string, payload []byte) error
	DeleteSecret(ctx context.Context, projectID, secretID string) error
	SecretExists(ctx context.Context, projectID, secretID string) (bool, error)
	AccessSecretVersion(
		ctx context.Context,
		projectID, secretID, version string,
	) ([]byte, error)
}

// ComputeClient abstracts Compute Engine API operations.
type ComputeClient interface {
	CreateVPC(ctx context.Context, projectID, vpcName string) error
	DeleteVPC(ctx context.Context, projectID, vpcName string) error
	VPCExists(ctx context.Context, projectID, vpcName string) (bool, error)
	CreateSubnet(
		ctx context.Context,
		projectID, region, subnetName, vpcName, cidrRange string,
	) error
	DeleteSubnet(ctx context.Context, projectID, region, subnetName string) error
	CreateFirewallRule(
		ctx context.Context,
		projectID, ruleName, vpcName, direction string,
		allowed []string,
	) error
	DeleteFirewallRule(ctx context.Context, projectID, ruleName string) error
}

// IAMClient abstracts IAM API operations.
type IAMClient interface {
	CreateServiceAccount(
		ctx context.Context,
		projectID, accountID, displayName string,
	) (string, error)
	DeleteServiceAccount(ctx context.Context, projectID, accountEmail string) error
	ServiceAccountExists(ctx context.Context, projectID, accountEmail string) (bool, error)
	AddIAMBinding(ctx context.Context, projectID, member, role string) error
	RemoveIAMBinding(ctx context.Context, projectID, member, role string) error
	AddServiceAccountIAMBinding(
		ctx context.Context,
		projectID, serviceAccountEmail, member, role string,
	) error
}

// LoggingClient abstracts Cloud Logging API operations.
type LoggingClient interface {
	CreateSink(ctx context.Context, projectID, sinkName, filter, destination string) error
	DeleteSink(ctx context.Context, projectID, sinkName string) error
	SinkExists(ctx context.Context, projectID, sinkName string) (bool, error)
	CreateLogBucket(
		ctx context.Context,
		projectID, bucketID, location string,
		retentionDays int,
	) error
}

// ArtifactRegistryClient abstracts Artifact Registry API operations.
type ArtifactRegistryClient interface {
	CreateRepository(ctx context.Context, projectID, location, repoID string) error
	DeleteRepository(ctx context.Context, projectID, location, repoID string) error
	RepositoryExists(ctx context.Context, projectID, location, repoID string) (bool, error)
}

// VPCAccessClient abstracts Serverless VPC Access API operations.
type VPCAccessClient interface {
	CreateConnector(
		ctx context.Context,
		projectID, region, connectorName, network, ipRange string,
		minInstances, maxInstances int,
	) error
	DeleteConnector(ctx context.Context, projectID, region, connectorName string) error
	ConnectorExists(ctx context.Context, projectID, region, connectorName string) (bool, error)
}

// GCPDeployer implements Deployer for GCP Resource Manager.
type GCPDeployer struct {
	client   *resourcemanager.ProjectsClient
	region   string
	services *GCPServiceClients
}

// NewGCPDeployer creates a new GCP deployer with the given region.
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

// NewGCPDeployerWithClient creates a new GCP deployer with a custom client.
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

// Deploy creates a new GCP project and optionally deploys backend resources.
func (d *GCPDeployer) Deploy(ctx context.Context, opts *DeployOptions) (*DeployResult, error) {
	if opts.Name == "" {
		return nil, errors.New("project ID is required for GCP")
	}

	projectID := opts.Name
	result := &DeployResult{
		Name:    projectID,
		Outputs: make(map[string]string),
	}

	exists, err := d.checkProjectExists(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if project exists: %w", err)
	}

	if exists {
		projectResult, handleErr := d.handleExistingProject(ctx, projectID, result)
		if handleErr != nil {
			return projectResult, handleErr
		}
		// Attempt backend deployment if service clients are available
		if deployErr := d.attemptBackendDeployment(ctx, projectID, opts); deployErr != nil {
			output.Warningf("Skipping backend deployment: %v", deployErr)
		}
		return projectResult, nil
	}

	projectResult, createErr := d.handleNewProject(ctx, projectID, opts, result)
	if createErr != nil {
		return projectResult, createErr
	}
	// Attempt backend deployment if service clients are available
	if deployErr := d.attemptBackendDeployment(ctx, projectID, opts); deployErr != nil {
		output.Warningf("Skipping backend deployment: %v", deployErr)
	}
	return projectResult, nil
}

func (d *GCPDeployer) handleExistingProject(
	ctx context.Context,
	projectID string,
	result *DeployResult,
) (*DeployResult, error) {
	result.OperationType = operationTypeUpdate
	result.Status = statusUpdateComplete
	result.NoChanges = false
	existingOutputs, getErr := d.getProjectOutputs(ctx, projectID)
	if getErr != nil {
		return result, fmt.Errorf("failed to retrieve project outputs: %w", getErr)
	}
	result.Outputs = existingOutputs
	return result, nil
}

func (d *GCPDeployer) handleNewProject(
	ctx context.Context,
	projectID string,
	opts *DeployOptions,
	result *DeployResult,
) (*DeployResult, error) {
	result.OperationType = operationTypeCreate

	if !opts.Wait {
		if startErr := d.startProjectCreation(ctx, projectID, opts); startErr != nil {
			return nil, fmt.Errorf("failed to create project: %w", startErr)
		}

		result.Status = statusInProgress
		return result, nil
	}

	createdProject, createErr := d.createNewProject(ctx, projectID, opts)
	if createErr != nil {
		return nil, fmt.Errorf("failed to create project: %w", createErr)
	}

	if waitErr := d.waitForProjectReady(ctx, projectID); waitErr != nil {
		return nil, fmt.Errorf("project creation failed: %w", waitErr)
	}

	result.Status = statusCreateComplete

	projectOutputs, getErr := d.getProjectOutputs(ctx, projectID)
	if getErr != nil {
		return result, fmt.Errorf("project created but failed to retrieve outputs: %w", getErr)
	}
	result.Outputs = projectOutputs

	if createdProject != nil {
		d.addProjectInfoToOutputs(result.Outputs, createdProject)
	}

	return result, nil
}

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

// CheckExists checks if a GCP project exists.
func (d *GCPDeployer) CheckExists(ctx context.Context, name string) (bool, error) {
	return d.checkProjectExists(ctx, name)
}

// checkProjectExists is the internal implementation for checking project existence.
func (d *GCPDeployer) checkProjectExists(ctx context.Context, projectID string) (bool, error) {
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

// GetOutputs retrieves outputs from a GCP project.
func (d *GCPDeployer) GetOutputs(ctx context.Context, name string) (map[string]string, error) {
	return d.getProjectOutputs(ctx, name)
}

// getProjectOutputs is the internal implementation for retrieving project outputs.
func (d *GCPDeployer) getProjectOutputs(
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
func (d *GCPDeployer) Destroy(
	ctx context.Context,
	opts *DestroyOptions,
) (*DestroyResult, error) {
	projectID := opts.Name
	result := &DestroyResult{
		Name: projectID,
	}

	exists, err := d.checkProjectExists(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to check project status: %w", err)
	}

	if !exists {
		result.NotFound = true
		result.Status = statusNotFound
		return result, nil
	}

	op, err := d.deleteProject(ctx, projectID)
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

// SetServiceClients sets the GCP service clients for resource management.
func (d *GCPDeployer) SetServiceClients(clients *GCPServiceClients) {
	d.services = clients
}

// attemptBackendDeployment attempts to deploy backend resources if service clients are available.
func (d *GCPDeployer) attemptBackendDeployment(
	ctx context.Context,
	projectID string,
	opts *DeployOptions,
) error {
	if d.services == nil {
		// Service clients not set, skip backend deployment
		return nil
	}

	params, err := ParseParameters(opts.Parameters)
	if err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
	}

	config := DefaultGCPResourceConfig(projectID, d.region)

	// Override with parameters if provided
	if img, ok := params["OrchestratorImage"]; ok {
		config.OrchestratorImage = img
	}
	if img, ok := params["EventProcessorImage"]; ok {
		config.EventProcessorImage = img
	}
	if maxInst, ok := params["MaxInstances"]; ok {
		if parsed, parseErr := parseInt(maxInst); parseErr == nil {
			config.MaxInstances = parsed
		}
	}
	if minInst, ok := params["MinInstances"]; ok {
		if parsed, parseErr := parseInt(minInst); parseErr == nil {
			config.MinInstances = parsed
		}
	}

	// Deploy backend resources
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

// DeployBackend deploys all GCP backend resources.
func (d *GCPDeployer) DeployBackend(
	ctx context.Context,
	config *GCPResourceConfig,
) (*GCPBackendResources, error) {
	if d.services == nil {
		return nil, errors.New("service clients not initialized; call SetServiceClients first")
	}

	resources := &GCPBackendResources{
		ProjectID: config.ProjectID,
		Region:    config.Region,
	}

	if err := d.deployIAMResources(ctx, config, resources); err != nil {
		return nil, fmt.Errorf("failed to deploy IAM resources: %w", err)
	}

	if err := d.deployNetworkResources(ctx, config, resources); err != nil {
		return nil, fmt.Errorf("failed to deploy network resources: %w", err)
	}

	if err := d.deployFirestore(ctx, config, resources); err != nil {
		return nil, fmt.Errorf("failed to deploy Firestore: %w", err)
	}

	if err := d.deployKMS(ctx, config, resources); err != nil {
		return nil, fmt.Errorf("failed to deploy KMS: %w", err)
	}

	if err := d.deployPubSub(ctx, config, resources); err != nil {
		return nil, fmt.Errorf("failed to deploy Pub/Sub: %w", err)
	}

	if err := d.deployArtifactRegistry(ctx, config, resources); err != nil {
		return nil, fmt.Errorf("failed to deploy Artifact Registry: %w", err)
	}

	if err := d.deployCloudRun(ctx, config, resources); err != nil {
		return nil, fmt.Errorf("failed to deploy Cloud Run: %w", err)
	}

	if err := d.deployCloudScheduler(ctx, config, resources); err != nil {
		return nil, fmt.Errorf("failed to deploy Cloud Scheduler: %w", err)
	}

	if err := d.deployLogging(ctx, config, resources); err != nil {
		return nil, fmt.Errorf("failed to deploy logging: %w", err)
	}

	return resources, nil
}

func (d *GCPDeployer) ensureServiceAccount(
	ctx context.Context,
	projectID, accountID, description string,
) (string, error) {
	email := buildServiceAccountEmail(accountID, projectID)

	exists, err := d.services.IAM.ServiceAccountExists(ctx, projectID, email)
	if err != nil {
		return "", fmt.Errorf("failed to check %s service account: %w", accountID, err)
	}
	if exists {
		return email, nil
	}

	created, createErr := d.services.IAM.CreateServiceAccount(
		ctx,
		projectID,
		accountID,
		description,
	)
	if createErr != nil {
		return "", fmt.Errorf("failed to create %s service account: %w", accountID, createErr)
	}

	return created, nil
}

func (d *GCPDeployer) deployIAMResources(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) error {
	orchestratorSA, err := d.ensureServiceAccount(
		ctx,
		config.ProjectID,
		constants.ServiceAccountOrchestrator,
		"Runvoy Orchestrator Service Account",
	)
	if err != nil {
		return fmt.Errorf("failed to ensure orchestrator service account: %w", err)
	}
	resources.OrchestratorServiceAccount = orchestratorSA

	eventProcessorSA, err := d.ensureServiceAccount(
		ctx,
		config.ProjectID,
		constants.ServiceAccountEventProcessor,
		"Runvoy Event Processor Service Account",
	)
	if err != nil {
		return fmt.Errorf("failed to ensure event processor service account: %w", err)
	}
	resources.EventProcessorServiceAccount = eventProcessorSA

	runnerSA, err := d.ensureServiceAccount(
		ctx,
		config.ProjectID,
		constants.ServiceAccountRunner,
		"Runvoy Runner Service Account",
	)
	if err != nil {
		return fmt.Errorf("failed to ensure runner service account: %w", err)
	}
	resources.RunnerServiceAccount = runnerSA

	if bindErr := d.bindOrchestratorRoles(ctx, config.ProjectID, orchestratorSA); bindErr != nil {
		return fmt.Errorf("failed to bind orchestrator roles: %w", bindErr)
	}

	if bindErr := d.bindEventProcessorRoles(
		ctx,
		config.ProjectID,
		eventProcessorSA,
	); bindErr != nil {
		return fmt.Errorf("failed to bind event processor roles: %w", bindErr)
	}

	if bindErr := d.bindRunnerRoles(ctx, config.ProjectID, runnerSA); bindErr != nil {
		return fmt.Errorf("failed to bind runner roles: %w", bindErr)
	}

	return nil
}

func (d *GCPDeployer) bindOrchestratorRoles(
	ctx context.Context,
	projectID, serviceAccountEmail string,
) error {
	roles := []string{
		"roles/datastore.user",
		"roles/run.invoker",
		"roles/run.developer",
		"roles/pubsub.publisher",
		"roles/cloudkms.cryptoKeyEncrypterDecrypter",
		"roles/secretmanager.secretAccessor",
		"roles/logging.logWriter",
		"roles/iam.serviceAccountUser",
		"roles/artifactregistry.reader",
	}

	member := "serviceAccount:" + serviceAccountEmail
	for _, role := range roles {
		if err := d.services.IAM.AddIAMBinding(ctx, projectID, member, role); err != nil {
			return fmt.Errorf("failed to add role %s: %w", role, err)
		}
	}

	return nil
}

func (d *GCPDeployer) bindEventProcessorRoles(
	ctx context.Context,
	projectID, serviceAccountEmail string,
) error {
	roles := []string{
		"roles/datastore.user",
		"roles/pubsub.subscriber",
		"roles/pubsub.publisher",
		"roles/cloudkms.cryptoKeyDecrypter",
		"roles/secretmanager.secretAccessor",
		"roles/logging.logWriter",
		"roles/run.invoker",
	}

	member := "serviceAccount:" + serviceAccountEmail
	for _, role := range roles {
		if err := d.services.IAM.AddIAMBinding(ctx, projectID, member, role); err != nil {
			return fmt.Errorf("failed to add role %s: %w", role, err)
		}
	}

	return nil
}

func (d *GCPDeployer) bindRunnerRoles(
	ctx context.Context,
	projectID, serviceAccountEmail string,
) error {
	roles := []string{
		"roles/logging.logWriter",
	}

	member := "serviceAccount:" + serviceAccountEmail
	for _, role := range roles {
		if err := d.services.IAM.AddIAMBinding(ctx, projectID, member, role); err != nil {
			return fmt.Errorf("failed to add role %s: %w", role, err)
		}
	}

	return nil
}

func (d *GCPDeployer) deployNetworkResources(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) error {
	if err := d.ensureVPC(ctx, config.ProjectID, config.VPCName); err != nil {
		return err
	}
	resources.VPCName = config.VPCName

	if err := d.ensureSubnet(ctx, config); err != nil {
		return err
	}
	resources.SubnetName = config.SubnetName

	if err := d.ensureEgressFirewall(ctx, config); err != nil {
		return err
	}

	connectorName, err := d.ensureVPCConnector(ctx, config)
	if err != nil {
		return err
	}
	resources.VPCConnectorName = connectorName

	return nil
}

func (d *GCPDeployer) ensureVPC(ctx context.Context, projectID, vpcName string) error {
	exists, err := d.services.Compute.VPCExists(ctx, projectID, vpcName)
	if err != nil {
		return fmt.Errorf("failed to check VPC: %w", err)
	}
	if exists {
		return nil
	}

	if createErr := d.services.Compute.CreateVPC(ctx, projectID, vpcName); createErr != nil {
		return fmt.Errorf("failed to create VPC: %w", createErr)
	}

	return nil
}

func (d *GCPDeployer) ensureSubnet(ctx context.Context, config *GCPResourceConfig) error {
	if err := d.services.Compute.CreateSubnet(
		ctx,
		config.ProjectID,
		config.Region,
		config.SubnetName,
		config.VPCName,
		config.VPCCIDRRange,
	); err != nil {
		return fmt.Errorf("failed to create subnet: %w", err)
	}

	return nil
}

func (d *GCPDeployer) ensureEgressFirewall(
	ctx context.Context,
	config *GCPResourceConfig,
) error {
	if err := d.services.Compute.CreateFirewallRule(
		ctx,
		config.ProjectID,
		constants.FirewallRuleEgress,
		config.VPCName,
		"EGRESS",
		[]string{"all"},
	); err != nil {
		return fmt.Errorf("failed to create egress firewall rule: %w", err)
	}

	return nil
}

func (d *GCPDeployer) ensureVPCConnector(
	ctx context.Context,
	config *GCPResourceConfig,
) (string, error) {
	exists, err := d.services.VPCAccess.ConnectorExists(
		ctx, config.ProjectID, config.Region, config.VPCConnectorName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to check VPC connector: %w", err)
	}
	if !exists {
		if createErr := d.services.VPCAccess.CreateConnector(
			ctx,
			config.ProjectID,
			config.Region,
			config.VPCConnectorName,
			config.VPCName,
			constants.VPCConnectorIPRange,
			constants.VPCConnectorMinInstances,
			constants.VPCConnectorMaxInstances,
		); createErr != nil {
			return "", fmt.Errorf("failed to create VPC connector: %w", createErr)
		}
	}

	return config.VPCConnectorName, nil
}

func (d *GCPDeployer) deployFirestore(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) error {
	exists, err := d.services.Firestore.GetDatabase(ctx, config.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to check Firestore database: %w", err)
	}
	if !exists {
		if createErr := d.services.Firestore.CreateDatabase(
			ctx, config.ProjectID, config.FirestoreLocationID,
		); createErr != nil {
			return fmt.Errorf("failed to create Firestore database: %w", createErr)
		}
	}
	resources.FirestoreDatabase = config.FirestoreLocationID

	collections := []struct {
		name   string
		fields []string
	}{
		{constants.CollectionAPIKeys, []string{"api_key_hash", "user_email"}},
		{constants.CollectionExecutions, []string{"execution_id", "started_at", "created_by_request_id"}},
		{constants.CollectionPendingAPIKeys, []string{"secret_token", "expires_at"}},
		{constants.CollectionSecretsMetadata, []string{"secret_name"}},
		{constants.CollectionImageConfigs, []string{"image_id", "is_default"}},
		{constants.CollectionWebSocketTokens, []string{"token", "execution_id", "expires_at"}},
		{constants.CollectionWebSocketConnection, []string{"connection_id", "execution_id", "expires_at"}},
		{constants.CollectionExecutionLogs, []string{"execution_id", "event_key", "expires_at"}},
	}

	for _, coll := range collections {
		for _, field := range coll.fields {
			if createErr := d.services.Firestore.CreateIndex(
				ctx, config.ProjectID, coll.name, field,
			); createErr != nil {
				return fmt.Errorf("failed to create index on %s.%s: %w", coll.name, field, createErr)
			}
		}
	}

	return nil
}

func (d *GCPDeployer) deployKMS(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) error {
	exists, err := d.services.KMS.KeyRingExists(
		ctx, config.ProjectID, config.Region, config.KeyRingName,
	)
	if err != nil {
		return fmt.Errorf("failed to check key ring: %w", err)
	}
	if !exists {
		if createErr := d.services.KMS.CreateKeyRing(
			ctx, config.ProjectID, config.Region, config.KeyRingName,
		); createErr != nil {
			return fmt.Errorf("failed to create key ring: %w", createErr)
		}
	}
	resources.KeyRingName = config.KeyRingName

	exists, err = d.services.KMS.CryptoKeyExists(
		ctx, config.ProjectID, config.Region, config.KeyRingName, config.CryptoKeyName,
	)
	if err != nil {
		return fmt.Errorf("failed to check crypto key: %w", err)
	}
	if !exists {
		cryptoKeyID, createErr := d.services.KMS.CreateCryptoKey(
			ctx,
			config.ProjectID,
			config.Region,
			config.KeyRingName,
			config.CryptoKeyName,
		)
		if createErr != nil {
			return fmt.Errorf("failed to create crypto key: %w", createErr)
		}
		resources.CryptoKeyID = cryptoKeyID
	} else {
		cryptoKeyID, getErr := d.services.KMS.GetCryptoKeyID(
			ctx, config.ProjectID, config.Region, config.KeyRingName, config.CryptoKeyName,
		)
		if getErr != nil {
			return fmt.Errorf("failed to get crypto key ID: %w", getErr)
		}
		resources.CryptoKeyID = cryptoKeyID
	}
	resources.CryptoKeyName = config.CryptoKeyName

	return nil
}

func (d *GCPDeployer) deployPubSub(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) error {
	exists, err := d.services.PubSub.TopicExists(ctx, config.ProjectID, config.TaskEventsTopic)
	if err != nil {
		return fmt.Errorf("failed to check task events topic: %w", err)
	}
	if !exists {
		if createErr := d.services.PubSub.CreateTopic(ctx, config.ProjectID, config.TaskEventsTopic); createErr != nil {
			return fmt.Errorf("failed to create task events topic: %w", createErr)
		}
	}
	resources.TaskEventsTopicName = config.TaskEventsTopic

	exists, err = d.services.PubSub.TopicExists(ctx, config.ProjectID, config.LogEventsTopic)
	if err != nil {
		return fmt.Errorf("failed to check log events topic: %w", err)
	}
	if !exists {
		if createErr := d.services.PubSub.CreateTopic(ctx, config.ProjectID, config.LogEventsTopic); createErr != nil {
			return fmt.Errorf("failed to create log events topic: %w", createErr)
		}
	}
	resources.LogEventsTopicName = config.LogEventsTopic

	exists, err = d.services.PubSub.TopicExists(ctx, config.ProjectID, constants.TopicWebSocketEvents)
	if err != nil {
		return fmt.Errorf("failed to check websocket events topic: %w", err)
	}
	if !exists {
		if createErr := d.services.PubSub.CreateTopic(
			ctx, config.ProjectID, constants.TopicWebSocketEvents,
		); createErr != nil {
			return fmt.Errorf("failed to create websocket events topic: %w", createErr)
		}
	}

	return nil
}

func (d *GCPDeployer) deployArtifactRegistry(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) error {
	exists, err := d.services.ArtifactRegistry.RepositoryExists(
		ctx, config.ProjectID, config.Region, constants.ArtifactRegistryRepo,
	)
	if err != nil {
		return fmt.Errorf("failed to check artifact registry repository: %w", err)
	}
	if !exists {
		if createErr := d.services.ArtifactRegistry.CreateRepository(
			ctx, config.ProjectID, config.Region, constants.ArtifactRegistryRepo,
		); createErr != nil {
			return fmt.Errorf("failed to create artifact registry repository: %w", createErr)
		}
	}
	resources.ArtifactRegistryRepo = constants.ArtifactRegistryRepo

	return nil
}

func (d *GCPDeployer) deployCloudRun(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) error {
	orchestratorURL, err := d.ensureOrchestratorService(ctx, config, resources)
	if err != nil {
		return err
	}
	resources.OrchestratorURL = orchestratorURL

	eventProcessorURL, err := d.ensureEventProcessorService(ctx, config, resources)
	if err != nil {
		return err
	}
	resources.EventProcessorURL = eventProcessorURL
	resources.WebSocketEndpoint = eventProcessorURL

	if subErr := d.ensureProcessorSubscription(ctx, config, resources); subErr != nil {
		return subErr
	}

	return nil
}

func (d *GCPDeployer) ensureOrchestratorService(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) (string, error) {
	exists, url, err := d.services.CloudRun.GetService(
		ctx, config.ProjectID, config.Region, constants.ServiceOrchestrator,
	)
	if err != nil {
		return "", fmt.Errorf("failed to check orchestrator service: %w", err)
	}

	envVars := d.buildOrchestratorEnvVars(config, resources)
	if !exists {
		orchestratorURL, createErr := d.services.CloudRun.CreateService(
			ctx,
			config.ProjectID,
			config.Region,
			constants.ServiceOrchestrator,
			config.OrchestratorImage,
			envVars,
			resources.OrchestratorServiceAccount,
		)
		if createErr != nil {
			return "", fmt.Errorf("failed to create orchestrator service: %w", createErr)
		}
		url = orchestratorURL
	} else {
		if updateErr := d.services.CloudRun.UpdateService(
			ctx,
			config.ProjectID,
			config.Region,
			constants.ServiceOrchestrator,
			config.OrchestratorImage,
			envVars,
		); updateErr != nil {
			return "", fmt.Errorf("failed to update orchestrator service: %w", updateErr)
		}
	}

	if policyErr := d.services.CloudRun.SetIAMPolicy(
		ctx, config.ProjectID, config.Region, constants.ServiceOrchestrator, true,
	); policyErr != nil {
		return "", fmt.Errorf("failed to set orchestrator IAM policy: %w", policyErr)
	}

	return url, nil
}

func (d *GCPDeployer) ensureEventProcessorService(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) (string, error) {
	exists, url, err := d.services.CloudRun.GetService(
		ctx, config.ProjectID, config.Region, constants.ServiceEventProcessor,
	)
	if err != nil {
		return "", fmt.Errorf("failed to check event processor service: %w", err)
	}

	envVars := d.buildEventProcessorEnvVars(config, resources)
	if !exists {
		eventProcessorURL, createErr := d.services.CloudRun.CreateService(
			ctx,
			config.ProjectID,
			config.Region,
			constants.ServiceEventProcessor,
			config.EventProcessorImage,
			envVars,
			resources.EventProcessorServiceAccount,
		)
		if createErr != nil {
			return "", fmt.Errorf("failed to create event processor service: %w", createErr)
		}
		url = eventProcessorURL
	} else {
		if updateErr := d.services.CloudRun.UpdateService(
			ctx,
			config.ProjectID,
			config.Region,
			constants.ServiceEventProcessor,
			config.EventProcessorImage,
			envVars,
		); updateErr != nil {
			return "", fmt.Errorf("failed to update event processor service: %w", updateErr)
		}
	}

	return url, nil
}

func (d *GCPDeployer) ensureProcessorSubscription(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) error {
	if err := d.services.PubSub.CreateSubscription(
		ctx,
		config.ProjectID,
		config.ProcessorSub,
		config.TaskEventsTopic,
		resources.EventProcessorURL,
	); err != nil {
		return fmt.Errorf("failed to create processor subscription: %w", err)
	}
	resources.SubscriptionName = config.ProcessorSub

	return nil
}

func (d *GCPDeployer) buildOrchestratorEnvVars(
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) map[string]string {
	return map[string]string{
		"RUNVOY_GCP_PROJECT_ID":                       config.ProjectID,
		"RUNVOY_GCP_REGION":                           config.Region,
		"RUNVOY_GCP_API_KEYS_COLLECTION":              constants.CollectionAPIKeys,
		"RUNVOY_GCP_EXECUTIONS_COLLECTION":            constants.CollectionExecutions,
		"RUNVOY_GCP_EXECUTION_LOGS_COLLECTION":        constants.CollectionExecutionLogs,
		"RUNVOY_GCP_IMAGE_CONFIGS_COLLECTION":         constants.CollectionImageConfigs,
		"RUNVOY_GCP_PENDING_API_KEYS_COLLECTION":      constants.CollectionPendingAPIKeys,
		"RUNVOY_GCP_SECRETS_METADATA_COLLECTION":      constants.CollectionSecretsMetadata,
		"RUNVOY_GCP_WEBSOCKET_CONNECTIONS_COLLECTION": constants.CollectionWebSocketConnection,
		"RUNVOY_GCP_WEBSOCKET_TOKENS_COLLECTION":      constants.CollectionWebSocketTokens,
		"RUNVOY_GCP_TASK_EVENTS_TOPIC":                resources.TaskEventsTopicName,
		"RUNVOY_GCP_LOG_EVENTS_TOPIC":                 resources.LogEventsTopicName,
		"RUNVOY_GCP_KMS_KEY_ID":                       resources.CryptoKeyID,
		"RUNVOY_GCP_RUNNER_SERVICE_ACCOUNT":           resources.RunnerServiceAccount,
		"RUNVOY_GCP_VPC_CONNECTOR":                    resources.VPCConnectorName,
		"RUNVOY_GCP_ARTIFACT_REGISTRY":                resources.ArtifactRegistryRepo,
	}
}

func (d *GCPDeployer) buildEventProcessorEnvVars(
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) map[string]string {
	return map[string]string{
		"RUNVOY_GCP_PROJECT_ID":                       config.ProjectID,
		"RUNVOY_GCP_REGION":                           config.Region,
		"RUNVOY_GCP_API_KEYS_COLLECTION":              constants.CollectionAPIKeys,
		"RUNVOY_GCP_EXECUTIONS_COLLECTION":            constants.CollectionExecutions,
		"RUNVOY_GCP_EXECUTION_LOGS_COLLECTION":        constants.CollectionExecutionLogs,
		"RUNVOY_GCP_IMAGE_CONFIGS_COLLECTION":         constants.CollectionImageConfigs,
		"RUNVOY_GCP_SECRETS_METADATA_COLLECTION":      constants.CollectionSecretsMetadata,
		"RUNVOY_GCP_WEBSOCKET_CONNECTIONS_COLLECTION": constants.CollectionWebSocketConnection,
		"RUNVOY_GCP_WEBSOCKET_TOKENS_COLLECTION":      constants.CollectionWebSocketTokens,
		"RUNVOY_GCP_TASK_EVENTS_TOPIC":                resources.TaskEventsTopicName,
		"RUNVOY_GCP_KMS_KEY_ID":                       resources.CryptoKeyID,
	}
}

func (d *GCPDeployer) deployCloudScheduler(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) error {
	exists, err := d.services.Scheduler.JobExists(
		ctx, config.ProjectID, config.Region, constants.SchedulerHealthReconcile,
	)
	if err != nil {
		return fmt.Errorf("failed to check scheduler job: %w", err)
	}
	if !exists {
		healthReconcileURL := resources.EventProcessorURL + "/health-reconcile"
		if createErr := d.services.Scheduler.CreateJob(
			ctx,
			config.ProjectID,
			config.Region,
			constants.SchedulerHealthReconcile,
			config.HealthSchedule,
			healthReconcileURL,
			"POST",
		); createErr != nil {
			return fmt.Errorf("failed to create health reconcile scheduler job: %w", createErr)
		}
	}
	resources.HealthReconcileJobName = constants.SchedulerHealthReconcile

	return nil
}

func (d *GCPDeployer) deployLogging(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) error {
	exists, err := d.services.Logging.SinkExists(
		ctx, config.ProjectID, constants.LogSinkRunner,
	)
	if err != nil {
		return fmt.Errorf("failed to check log sink: %w", err)
	}
	if !exists {
		runnerFilter := fmt.Sprintf(
			`resource.type="cloud_run_revision" AND resource.labels.service_name=%q`,
			constants.ServiceRunner,
		)
		runnerDestination := fmt.Sprintf(
			"pubsub.googleapis.com/projects/%s/topics/%s",
			config.ProjectID, resources.LogEventsTopicName,
		)

		if createErr := d.services.Logging.CreateSink(
			ctx, config.ProjectID, constants.LogSinkRunner, runnerFilter, runnerDestination,
		); createErr != nil {
			return fmt.Errorf("failed to create runner log sink: %w", createErr)
		}
	}

	return nil
}

// DestroyBackend destroys all GCP backend resources.
func (d *GCPDeployer) DestroyBackend(ctx context.Context, config *GCPResourceConfig) error {
	if d.services == nil {
		return errors.New("service clients not initialized; call SetServiceClients first")
	}

	var errs []error

	errs = d.destroyLoggingResources(ctx, config, errs)
	errs = d.destroySchedulerResources(ctx, config, errs)
	errs = d.destroyCloudRunResources(ctx, config, errs)
	errs = d.destroyPubSubResources(ctx, config, errs)
	errs = d.destroyArtifactRegistryResources(ctx, config, errs)
	errs = d.destroyNetworkResources(ctx, config, errs)
	errs = d.destroyIAMResources(ctx, config, errs)

	if len(errs) > 0 {
		return fmt.Errorf("failed to destroy some resources: %v", errs)
	}

	return nil
}

func (d *GCPDeployer) destroyLoggingResources(
	ctx context.Context,
	config *GCPResourceConfig,
	errs []error,
) []error {
	if err := d.services.Logging.DeleteSink(
		ctx, config.ProjectID, constants.LogSinkRunner,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete runner log sink: %w", err))
	}
	return errs
}

func (d *GCPDeployer) destroySchedulerResources(
	ctx context.Context,
	config *GCPResourceConfig,
	errs []error,
) []error {
	if err := d.services.Scheduler.DeleteJob(
		ctx, config.ProjectID, config.Region, constants.SchedulerHealthReconcile,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete scheduler job: %w", err))
	}
	return errs
}

func (d *GCPDeployer) destroyCloudRunResources(
	ctx context.Context,
	config *GCPResourceConfig,
	errs []error,
) []error {
	if err := d.services.CloudRun.DeleteService(
		ctx, config.ProjectID, config.Region, constants.ServiceOrchestrator,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete orchestrator service: %w", err))
	}
	if err := d.services.CloudRun.DeleteService(
		ctx, config.ProjectID, config.Region, constants.ServiceEventProcessor,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete event processor service: %w", err))
	}
	return errs
}

func (d *GCPDeployer) destroyPubSubResources(
	ctx context.Context,
	config *GCPResourceConfig,
	errs []error,
) []error {
	if err := d.services.PubSub.DeleteSubscription(
		ctx, config.ProjectID, config.ProcessorSub,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete processor subscription: %w", err))
	}
	if err := d.services.PubSub.DeleteTopic(ctx, config.ProjectID, config.TaskEventsTopic); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete task events topic: %w", err))
	}
	if err := d.services.PubSub.DeleteTopic(ctx, config.ProjectID, config.LogEventsTopic); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete log events topic: %w", err))
	}
	if err := d.services.PubSub.DeleteTopic(
		ctx, config.ProjectID, constants.TopicWebSocketEvents,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete websocket events topic: %w", err))
	}
	return errs
}

func (d *GCPDeployer) destroyArtifactRegistryResources(
	ctx context.Context,
	config *GCPResourceConfig,
	errs []error,
) []error {
	if err := d.services.ArtifactRegistry.DeleteRepository(
		ctx, config.ProjectID, config.Region, constants.ArtifactRegistryRepo,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete artifact registry repository: %w", err))
	}
	return errs
}

func (d *GCPDeployer) destroyNetworkResources(
	ctx context.Context,
	config *GCPResourceConfig,
	errs []error,
) []error {
	if err := d.services.VPCAccess.DeleteConnector(
		ctx, config.ProjectID, config.Region, config.VPCConnectorName,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete VPC connector: %w", err))
	}
	if err := d.services.Compute.DeleteFirewallRule(
		ctx, config.ProjectID, constants.FirewallRuleEgress,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete firewall rule: %w", err))
	}
	if err := d.services.Compute.DeleteSubnet(
		ctx, config.ProjectID, config.Region, config.SubnetName,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete subnet: %w", err))
	}
	if err := d.services.Compute.DeleteVPC(ctx, config.ProjectID, config.VPCName); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete VPC: %w", err))
	}
	return errs
}

func (d *GCPDeployer) destroyIAMResources(
	ctx context.Context,
	config *GCPResourceConfig,
	errs []error,
) []error {
	orchestratorEmail := buildServiceAccountEmail(
		constants.ServiceAccountOrchestrator, config.ProjectID,
	)
	if err := d.services.IAM.DeleteServiceAccount(
		ctx, config.ProjectID, orchestratorEmail,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete orchestrator service account: %w", err))
	}

	eventProcessorEmail := buildServiceAccountEmail(
		constants.ServiceAccountEventProcessor, config.ProjectID,
	)
	if err := d.services.IAM.DeleteServiceAccount(
		ctx, config.ProjectID, eventProcessorEmail,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete event processor service account: %w", err))
	}

	runnerEmail := buildServiceAccountEmail(constants.ServiceAccountRunner, config.ProjectID)
	if err := d.services.IAM.DeleteServiceAccount(ctx, config.ProjectID, runnerEmail); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete runner service account: %w", err))
	}
	return errs
}

func buildServiceAccountEmail(accountID, projectID string) string {
	return accountID + "@" + projectID + ".iam.gserviceaccount.com"
}

// GetBackendResources retrieves information about deployed GCP backend resources.
func (d *GCPDeployer) GetBackendResources(
	ctx context.Context,
	config *GCPResourceConfig,
) (*GCPBackendResources, error) {
	if d.services == nil {
		return nil, errors.New("service clients not initialized; call SetServiceClients first")
	}

	resources := &GCPBackendResources{
		ProjectID: config.ProjectID,
		Region:    config.Region,
	}

	d.getCloudRunResources(ctx, config, resources)
	d.getKMSResources(ctx, config, resources)
	d.getPubSubResources(ctx, config, resources)
	d.getNetworkResources(ctx, config, resources)
	d.getRegistryResources(ctx, config, resources)
	d.getIAMResources(ctx, config, resources)
	d.getSchedulerResources(ctx, config, resources)

	return resources, nil
}

func (d *GCPDeployer) getCloudRunResources(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) {
	if exists, url, err := d.services.CloudRun.GetService(
		ctx, config.ProjectID, config.Region, constants.ServiceOrchestrator,
	); err == nil && exists {
		resources.OrchestratorURL = url
	}

	if exists, url, err := d.services.CloudRun.GetService(
		ctx, config.ProjectID, config.Region, constants.ServiceEventProcessor,
	); err == nil && exists {
		resources.EventProcessorURL = url
		resources.WebSocketEndpoint = url
	}
}

func (d *GCPDeployer) getKMSResources(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) {
	exists, err := d.services.KMS.CryptoKeyExists(
		ctx, config.ProjectID, config.Region, config.KeyRingName, config.CryptoKeyName,
	)
	if err != nil || !exists {
		return
	}

	resources.KeyRingName = config.KeyRingName
	resources.CryptoKeyName = config.CryptoKeyName

	keyID, keyErr := d.services.KMS.GetCryptoKeyID(
		ctx, config.ProjectID, config.Region, config.KeyRingName, config.CryptoKeyName,
	)
	if keyErr == nil {
		resources.CryptoKeyID = keyID
	}
}

func (d *GCPDeployer) getPubSubResources(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) {
	if exists, err := d.services.PubSub.TopicExists(
		ctx, config.ProjectID, config.TaskEventsTopic,
	); err == nil && exists {
		resources.TaskEventsTopicName = config.TaskEventsTopic
	}
	if exists, err := d.services.PubSub.TopicExists(
		ctx, config.ProjectID, config.LogEventsTopic,
	); err == nil && exists {
		resources.LogEventsTopicName = config.LogEventsTopic
	}
}

func (d *GCPDeployer) getNetworkResources(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) {
	if exists, err := d.services.Compute.VPCExists(
		ctx, config.ProjectID, config.VPCName,
	); err == nil && exists {
		resources.VPCName = config.VPCName
	}

	if exists, err := d.services.VPCAccess.ConnectorExists(
		ctx, config.ProjectID, config.Region, config.VPCConnectorName,
	); err == nil && exists {
		resources.VPCConnectorName = config.VPCConnectorName
	}
}

func (d *GCPDeployer) getRegistryResources(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) {
	if exists, err := d.services.ArtifactRegistry.RepositoryExists(
		ctx, config.ProjectID, config.Region, constants.ArtifactRegistryRepo,
	); err == nil && exists {
		resources.ArtifactRegistryRepo = constants.ArtifactRegistryRepo
	}
}

func (d *GCPDeployer) getIAMResources(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) {
	orchestratorEmail := buildServiceAccountEmail(
		constants.ServiceAccountOrchestrator, config.ProjectID,
	)
	if exists, err := d.services.IAM.ServiceAccountExists(
		ctx, config.ProjectID, orchestratorEmail,
	); err == nil && exists {
		resources.OrchestratorServiceAccount = orchestratorEmail
	}

	eventProcessorEmail := buildServiceAccountEmail(
		constants.ServiceAccountEventProcessor, config.ProjectID,
	)
	if exists, err := d.services.IAM.ServiceAccountExists(
		ctx, config.ProjectID, eventProcessorEmail,
	); err == nil && exists {
		resources.EventProcessorServiceAccount = eventProcessorEmail
	}

	runnerEmail := buildServiceAccountEmail(constants.ServiceAccountRunner, config.ProjectID)
	if exists, err := d.services.IAM.ServiceAccountExists(
		ctx, config.ProjectID, runnerEmail,
	); err == nil && exists {
		resources.RunnerServiceAccount = runnerEmail
	}
}

func (d *GCPDeployer) getSchedulerResources(
	ctx context.Context,
	config *GCPResourceConfig,
	resources *GCPBackendResources,
) {
	if exists, err := d.services.Scheduler.JobExists(
		ctx, config.ProjectID, config.Region, constants.SchedulerHealthReconcile,
	); err == nil && exists {
		resources.HealthReconcileJobName = constants.SchedulerHealthReconcile
	}
}

// BackendResourcesExist checks if GCP backend resources exist.
func (d *GCPDeployer) BackendResourcesExist(
	ctx context.Context,
	config *GCPResourceConfig,
) (bool, error) {
	if d.services == nil {
		return false, errors.New("service clients not initialized; call SetServiceClients first")
	}

	exists, _, err := d.services.CloudRun.GetService(
		ctx, config.ProjectID, config.Region, constants.ServiceOrchestrator,
	)
	if err != nil {
		return false, fmt.Errorf("failed to check orchestrator service: %w", err)
	}

	return exists, nil
}
