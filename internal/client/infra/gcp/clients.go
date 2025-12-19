package gcp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"google.golang.org/api/artifactregistry/v1"
	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/cloudscheduler/v1"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/firestore/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/logging/v2"
	"google.golang.org/api/option"
	"google.golang.org/api/pubsub/v1"
	"google.golang.org/api/run/v2"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/serviceusage/v1"
	"google.golang.org/api/vpcaccess/v1"

	"github.com/runvoy/runvoy/internal/providers/gcp/constants"
)

// newDefaultGCPServiceClients builds concrete service clients backed by Google Cloud APIs.
//
//nolint:funlen,gocyclo // initialization requires wiring many Google Cloud clients
func newDefaultGCPServiceClients(
	ctx context.Context,
	region string,
	projectClient *resourcemanager.ProjectsClient,
) (*GCPServiceClients, error) {
	firestoreSvc, err := firestore.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create firestore service: %w", err)
	}

	runSvc, err := run.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create run service: %w", err)
	}

	pubsubSvc, err := pubsub.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create pubsub service: %w", err)
	}

	kmsSvc, err := cloudkms.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create kms service: %w", err)
	}

	schedulerSvc, err := cloudscheduler.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create scheduler service: %w", err)
	}

	secretManagerSvc, err := secretmanager.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create secret manager service: %w", err)
	}

	computeSvc, err := compute.NewService(ctx, option.WithScopes(compute.CloudPlatformScope))
	if err != nil {
		return nil, fmt.Errorf("create compute service: %w", err)
	}

	iamSvc, err := iam.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create iam service: %w", err)
	}

	loggingSvc, err := logging.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create logging service: %w", err)
	}

	artifactSvc, err := artifactregistry.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create artifact registry service: %w", err)
	}

	vpcAccessSvc, err := vpcaccess.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create vpc access service: %w", err)
	}

	rmSvc, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create resource manager service: %w", err)
	}

	serviceUsageSvc, err := serviceusage.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create service usage service: %w", err)
	}

	if projectClient == nil {
		projectClient, err = resourcemanager.NewProjectsClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("create projects client: %w", err)
		}
	}

	return &GCPServiceClients{
		Projects: &defaultProjectsClient{client: projectClient},
		Firestore: &defaultFirestoreClient{
			service: firestoreSvc,
		},
		CloudRun: &defaultCloudRunClient{
			service: runSvc,
			region:  region,
		},
		PubSub: &defaultPubSubClient{
			service: pubsubSvc,
		},
		KMS: &defaultKMSClient{
			service: kmsSvc,
		},
		Scheduler: &defaultSchedulerClient{
			service: schedulerSvc,
		},
		SecretManager: &defaultSecretManagerClient{
			service: secretManagerSvc,
		},
		Compute: &defaultComputeClient{
			service: computeSvc,
		},
		IAM: &defaultIAMClient{
			iamService:      iamSvc,
			resourceManager: rmSvc,
		},
		Logging: &defaultLoggingClient{
			service: loggingSvc,
		},
		ArtifactRegistry: &defaultArtifactRegistryClient{
			service: artifactSvc,
			region:  region,
		},
		VPCAccess: &defaultVPCAccessClient{
			service: vpcAccessSvc,
		},
		ServiceUsage: &defaultServiceUsageClient{
			service: serviceUsageSvc,
		},
	}, nil
}

type defaultProjectsClient struct {
	client *resourcemanager.ProjectsClient
}

func (c *defaultProjectsClient) GetProject(ctx context.Context, name string) (*resourcemanagerpb.Project, error) {
	project, err := c.client.GetProject(ctx, &resourcemanagerpb.GetProjectRequest{Name: name})
	if err != nil {
		return nil, wrapError("get project", err)
	}
	return project, nil
}

func (c *defaultProjectsClient) CreateProject(ctx context.Context, project *resourcemanagerpb.Project) error {
	req := &resourcemanagerpb.CreateProjectRequest{Project: project}
	_, err := c.client.CreateProject(ctx, req)
	return wrapError("create project", err)
}

func (c *defaultProjectsClient) DeleteProject(ctx context.Context, name string) error {
	_, err := c.client.DeleteProject(ctx, &resourcemanagerpb.DeleteProjectRequest{Name: name})
	return wrapError("delete project", err)
}

type defaultFirestoreClient struct {
	service *firestore.Service
}

func (c *defaultFirestoreClient) CreateDatabase(ctx context.Context, projectID, locationID string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.FirestoreOperationTimeout)
	defer cancel()

	parent := "projects/" + projectID
	db := &firestore.GoogleFirestoreAdminV1Database{
		Type:       "FIRESTORE_NATIVE",
		LocationId: locationID,
	}

	_, err := c.service.Projects.Databases.Create(parent, db).
		DatabaseId(constants.FirestoreDatabaseID).
		Context(ctx).
		Do()
	if isAlreadyExists(err) {
		return nil
	}
	return wrapError("create firestore database", err)
}

func (c *defaultFirestoreClient) GetDatabase(ctx context.Context, projectID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.FirestoreOperationTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/databases/%s", projectID, constants.FirestoreDatabaseID)
	_, err := c.service.Projects.Databases.Get(name).Context(ctx).Do()
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, wrapError("get firestore database", err)
}

type defaultCloudRunClient struct {
	service *run.Service
	region  string
}

func (c *defaultCloudRunClient) serviceName(projectID, service string) string {
	return fmt.Sprintf("projects/%s/locations/%s/services/%s", projectID, c.region, service)
}

func (c *defaultCloudRunClient) parent(projectID string) string {
	return fmt.Sprintf("projects/%s/locations/%s", projectID, c.region)
}

func (c *defaultCloudRunClient) connectorName(projectID, connector string) string {
	return fmt.Sprintf("projects/%s/locations/%s/connectors/%s", projectID, c.region, connector)
}

func (c *defaultCloudRunClient) toRunVPCAccess(
	projectID string,
	vpcAccess *CloudRunVPCAccess,
) *run.GoogleCloudRunV2VpcAccess {
	if vpcAccess == nil {
		return nil
	}

	result := &run.GoogleCloudRunV2VpcAccess{
		Egress: "ALL_TRAFFIC",
	}

	switch {
	case vpcAccess.Connector != "":
		result.Connector = c.connectorName(projectID, vpcAccess.Connector)
	case vpcAccess.Network != "" || vpcAccess.Subnetwork != "":
		result.NetworkInterfaces = []*run.GoogleCloudRunV2NetworkInterface{
			{
				Network:    vpcAccess.Network,
				Subnetwork: vpcAccess.Subnetwork,
			},
		}
	default:
		return nil
	}

	return result
}

func (c *defaultCloudRunClient) CreateService(
	ctx context.Context,
	projectID, _ string, serviceName, image string,
	envVars map[string]string,
	serviceAccount string,
	minInstances, maxInstances int,
	timeoutSeconds int,
	vpcAccess *CloudRunVPCAccess,
) (string, error) {
	maxInst := int64(maxInstances)
	if maxInst == 0 {
		maxInst = int64(constants.DefaultMaxInstances)
	}
	minInst := int64(minInstances)
	timeout := timeoutSeconds
	if timeout == 0 {
		timeout = constants.DefaultTimeoutSeconds
	}

	runService := &run.GoogleCloudRunV2Service{
		Name: c.serviceName(projectID, serviceName),
		Template: &run.GoogleCloudRunV2RevisionTemplate{
			Containers: []*run.GoogleCloudRunV2Container{
				{
					Image: image,
					Env:   toRunEnvVars(envVars),
				},
			},
			ServiceAccount: serviceAccount,
			Scaling: &run.GoogleCloudRunV2RevisionScaling{
				MaxInstanceCount: maxInst,
				MinInstanceCount: minInst,
			},
			Timeout: fmt.Sprintf("%ds", timeout),
		},
	}
	if runVpcAccess := c.toRunVPCAccess(projectID, vpcAccess); runVpcAccess != nil {
		runService.Template.VpcAccess = runVpcAccess
	}

	op, err := c.service.Projects.Locations.Services.Create(
		c.parent(projectID),
		runService,
	).ServiceId(serviceName).Context(ctx).Do()
	if err != nil {
		return "", wrapError("create cloud run service", err)
	}

	if waitErr := c.waitForRunOperation(ctx, op.Name); waitErr != nil {
		return "", wrapError("wait for cloud run creation", waitErr)
	}

	created, err := c.service.Projects.Locations.Services.Get(c.serviceName(projectID, serviceName)).
		Context(ctx).
		Do()
	if err != nil {
		return "", wrapError("get cloud run service uri", err)
	}
	return created.Uri, nil
}

func (c *defaultCloudRunClient) UpdateService(
	ctx context.Context,
	projectID, _ string, serviceName, image string,
	envVars map[string]string,
	minInstances, maxInstances int,
	timeoutSeconds int,
	vpcAccess *CloudRunVPCAccess,
) error {
	servicePath := c.serviceName(projectID, serviceName)

	svc, err := c.service.Projects.Locations.Services.Get(servicePath).Context(ctx).Do()
	if err != nil {
		return wrapError("get cloud run service", err)
	}

	if svc.Template == nil {
		svc.Template = &run.GoogleCloudRunV2RevisionTemplate{}
	}

	maxInst := int64(maxInstances)
	if maxInst == 0 {
		maxInst = int64(constants.DefaultMaxInstances)
	}
	minInst := int64(minInstances)
	timeout := timeoutSeconds
	if timeout == 0 {
		timeout = constants.DefaultTimeoutSeconds
	}

	svc.Template.Containers = []*run.GoogleCloudRunV2Container{
		{
			Image: image,
			Env:   toRunEnvVars(envVars),
		},
	}
	svc.Template.Scaling = &run.GoogleCloudRunV2RevisionScaling{
		MaxInstanceCount: maxInst,
		MinInstanceCount: minInst,
	}
	svc.Template.Timeout = fmt.Sprintf("%ds", timeout)
	if runVpcAccess := c.toRunVPCAccess(projectID, vpcAccess); runVpcAccess != nil {
		svc.Template.VpcAccess = runVpcAccess
	}

	op, err := c.service.Projects.Locations.Services.Patch(servicePath, svc).
		UpdateMask("template").
		Context(ctx).
		Do()
	if err != nil {
		return wrapError("update cloud run service", err)
	}

	return c.waitForRunOperation(ctx, op.Name)
}

func (c *defaultCloudRunClient) DeleteService(ctx context.Context, projectID, _, serviceName string) error {
	op, err := c.service.Projects.Locations.Services.Delete(c.serviceName(projectID, serviceName)).
		Context(ctx).
		Do()
	if isNotFound(err) {
		return nil
	}
	if err != nil {
		return wrapError("delete cloud run service", err)
	}
	return wrapError("wait for cloud run deletion", c.waitForRunOperation(ctx, op.Name))
}

func (c *defaultCloudRunClient) GetService(
	ctx context.Context,
	projectID, _ string, serviceName string,
) (exists bool, url string, err error) {
	svc, err := c.service.Projects.Locations.Services.Get(c.serviceName(projectID, serviceName)).
		Context(ctx).
		Do()
	if isNotFound(err) {
		return false, "", nil
	}
	if err != nil {
		return false, "", wrapError("get cloud run service", err)
	}
	return true, svc.Uri, nil
}

func (c *defaultCloudRunClient) SetIAMPolicy(
	ctx context.Context,
	projectID, _ string, serviceName string,
	allowUnauthenticated bool,
) error {
	resource := c.serviceName(projectID, serviceName)
	policy, err := c.service.Projects.Locations.Services.GetIamPolicy(resource).
		Context(ctx).
		Do()
	if err != nil {
		return wrapError("get cloud run iam policy", err)
	}

	const invokerRole = "roles/run.invoker"
	member := "allUsers"

	if allowUnauthenticated {
		if !bindingExists(policy.Bindings, invokerRole, member) {
			policy.Bindings = append(policy.Bindings, &run.GoogleIamV1Binding{
				Role:    invokerRole,
				Members: []string{member},
			})
		}
	} else {
		policy.Bindings = removeBinding(policy.Bindings, invokerRole, member)
	}

	_, err = c.service.Projects.Locations.Services.SetIamPolicy(
		resource,
		&run.GoogleIamV1SetIamPolicyRequest{Policy: policy},
	).Context(ctx).Do()
	return wrapError("set cloud run iam policy", err)
}

func (c *defaultCloudRunClient) waitForRunOperation(ctx context.Context, name string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.CloudRunOperationTimeout)
	defer cancel()

	for {
		op, err := c.service.Projects.Locations.Operations.Get(name).Context(ctx).Do()
		if err != nil {
			return wrapError("poll cloud run operation", err)
		}
		if op.Done {
			if op.Error != nil {
				return fmt.Errorf("operation error: %s", op.Error.Message)
			}
			return nil
		}
		time.Sleep(constants.ResourcePollInterval)
	}
}

type defaultPubSubClient struct {
	service *pubsub.Service
}

func (c *defaultPubSubClient) CreateTopic(ctx context.Context, projectID, topicID string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.PubSubOperationTimeout)
	defer cancel()

	name := c.topicName(projectID, topicID)
	_, err := c.service.Projects.Topics.Create(name, &pubsub.Topic{}).Context(ctx).Do()
	if isAlreadyExists(err) {
		return nil
	}
	return wrapError("create pubsub topic", err)
}

func (c *defaultPubSubClient) DeleteTopic(ctx context.Context, projectID, topicID string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.PubSubOperationTimeout)
	defer cancel()

	name := c.topicName(projectID, topicID)
	_, err := c.service.Projects.Topics.Delete(name).Context(ctx).Do()
	if isNotFound(err) {
		return nil
	}
	return wrapError("delete pubsub topic", err)
}

func (c *defaultPubSubClient) TopicExists(ctx context.Context, projectID, topicID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.PubSubOperationTimeout)
	defer cancel()

	_, err := c.service.Projects.Topics.Get(c.topicName(projectID, topicID)).Context(ctx).Do()
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, wrapError("get pubsub topic", err)
}

func (c *defaultPubSubClient) CreateSubscription(
	ctx context.Context,
	projectID, subscriptionID, topicID, pushEndpoint string,
) error {
	ctx, cancel := context.WithTimeout(ctx, constants.PubSubOperationTimeout)
	defer cancel()

	parent := "projects/" + projectID
	_, err := c.service.Projects.Subscriptions.Create(
		fmt.Sprintf("%s/subscriptions/%s", parent, subscriptionID),
		&pubsub.Subscription{
			Topic: c.topicName(projectID, topicID),
			PushConfig: &pubsub.PushConfig{
				PushEndpoint: pushEndpoint,
			},
		},
	).Context(ctx).Do()
	if isAlreadyExists(err) {
		return nil
	}
	return wrapError("create pubsub subscription", err)
}

func (c *defaultPubSubClient) DeleteSubscription(ctx context.Context, projectID, subscriptionID string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.PubSubOperationTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/subscriptions/%s", projectID, subscriptionID)
	_, err := c.service.Projects.Subscriptions.Delete(name).Context(ctx).Do()
	if isNotFound(err) {
		return nil
	}
	return wrapError("delete pubsub subscription", err)
}

func (c *defaultPubSubClient) topicName(projectID, topicID string) string {
	return fmt.Sprintf("projects/%s/topics/%s", projectID, topicID)
}

type defaultKMSClient struct {
	service *cloudkms.Service
}

func (c *defaultKMSClient) CreateKeyRing(ctx context.Context, projectID, locationID, keyRingID string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.KMSOperationTimeout)
	defer cancel()

	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, locationID)
	_, err := c.service.Projects.Locations.KeyRings.Create(parent, &cloudkms.KeyRing{}).
		KeyRingId(keyRingID).
		Context(ctx).
		Do()
	if isAlreadyExists(err) {
		return nil
	}
	return wrapError("create kms key ring", err)
}

func (c *defaultKMSClient) KeyRingExists(ctx context.Context, projectID, locationID, keyRingID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.KMSOperationTimeout)
	defer cancel()

	_, err := c.service.Projects.Locations.KeyRings.Get(
		fmt.Sprintf("projects/%s/locations/%s/keyRings/%s", projectID, locationID, keyRingID),
	).Context(ctx).Do()
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, wrapError("get kms key ring", err)
}

func (c *defaultKMSClient) CreateCryptoKey(
	ctx context.Context,
	projectID, locationID, keyRingID, cryptoKeyID string,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.KMSOperationTimeout)
	defer cancel()

	parent := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s", projectID, locationID, keyRingID)
	cryptoKey := &cloudkms.CryptoKey{
		Purpose: "ENCRYPT_DECRYPT",
	}

	created, err := c.service.Projects.Locations.KeyRings.CryptoKeys.Create(parent, cryptoKey).
		CryptoKeyId(cryptoKeyID).
		Context(ctx).
		Do()
	if isAlreadyExists(err) {
		return c.GetCryptoKeyID(ctx, projectID, locationID, keyRingID, cryptoKeyID)
	}
	if err != nil {
		return "", wrapError("create kms crypto key", err)
	}
	return created.Name, nil
}

func (c *defaultKMSClient) CryptoKeyExists(
	ctx context.Context,
	projectID, locationID, keyRingID, cryptoKeyID string,
) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.KMSOperationTimeout)
	defer cancel()

	_, err := c.service.Projects.Locations.KeyRings.CryptoKeys.Get(
		fmt.Sprintf(
			"projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
			projectID, locationID, keyRingID, cryptoKeyID,
		),
	).Context(ctx).Do()
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, wrapError("get kms crypto key", err)
}

func (c *defaultKMSClient) GetCryptoKeyID(
	ctx context.Context,
	projectID, locationID, keyRingID, cryptoKeyID string,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.KMSOperationTimeout)
	defer cancel()

	key, err := c.service.Projects.Locations.KeyRings.CryptoKeys.Get(
		fmt.Sprintf(
			"projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
			projectID, locationID, keyRingID, cryptoKeyID,
		),
	).Context(ctx).Do()
	if err != nil {
		return "", wrapError("get kms crypto key id", err)
	}
	return key.Name, nil
}

type defaultSchedulerClient struct {
	service *cloudscheduler.Service
}

func (c *defaultSchedulerClient) CreateJob(
	ctx context.Context,
	projectID, region, jobID, schedule, targetURL, httpMethod string,
) error {
	ctx, cancel := context.WithTimeout(ctx, constants.SchedulerOperationTimeout)
	defer cancel()

	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, region)
	job := &cloudscheduler.Job{
		Name:     fmt.Sprintf("%s/jobs/%s", parent, jobID),
		Schedule: schedule,
		HttpTarget: &cloudscheduler.HttpTarget{
			HttpMethod: strings.ToUpper(httpMethod),
			Uri:        targetURL,
		},
	}

	_, err := c.service.Projects.Locations.Jobs.Create(parent, job).Context(ctx).Do()
	if isAlreadyExists(err) {
		return nil
	}
	return wrapError("create scheduler job", err)
}

func (c *defaultSchedulerClient) DeleteJob(ctx context.Context, projectID, region, jobID string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.SchedulerOperationTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/locations/%s/jobs/%s", projectID, region, jobID)
	_, err := c.service.Projects.Locations.Jobs.Delete(name).Context(ctx).Do()
	if isNotFound(err) {
		return nil
	}
	return wrapError("delete scheduler job", err)
}

func (c *defaultSchedulerClient) JobExists(ctx context.Context, projectID, region, jobID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.SchedulerOperationTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/locations/%s/jobs/%s", projectID, region, jobID)
	_, err := c.service.Projects.Locations.Jobs.Get(name).Context(ctx).Do()
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, wrapError("get scheduler job", err)
}

type defaultSecretManagerClient struct {
	service *secretmanager.Service
}

func (c *defaultSecretManagerClient) CreateSecret(ctx context.Context, projectID, secretID string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.SecretManagerTimeout)
	defer cancel()

	parent := "projects/" + projectID
	secret := &secretmanager.Secret{
		Replication: &secretmanager.Replication{
			Automatic: &secretmanager.Automatic{},
		},
	}
	_, err := c.service.Projects.Secrets.Create(parent, secret).
		SecretId(secretID).
		Context(ctx).
		Do()
	if isAlreadyExists(err) {
		return nil
	}
	return wrapError("create secret", err)
}

func (c *defaultSecretManagerClient) AddSecretVersion(
	ctx context.Context,
	projectID, secretID string,
	payload []byte,
) error {
	ctx, cancel := context.WithTimeout(ctx, constants.SecretManagerTimeout)
	defer cancel()

	parent := fmt.Sprintf("projects/%s/secrets/%s", projectID, secretID)
	encoded := base64.StdEncoding.EncodeToString(payload)
	req := &secretmanager.AddSecretVersionRequest{
		Payload: &secretmanager.SecretPayload{
			Data: encoded,
		},
	}
	_, err := c.service.Projects.Secrets.AddVersion(parent, req).Context(ctx).Do()
	return wrapError("add secret version", err)
}

func (c *defaultSecretManagerClient) DeleteSecret(ctx context.Context, projectID, secretID string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.SecretManagerTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/secrets/%s", projectID, secretID)
	_, err := c.service.Projects.Secrets.Delete(name).Context(ctx).Do()
	if isNotFound(err) {
		return nil
	}
	return wrapError("delete secret", err)
}

func (c *defaultSecretManagerClient) SecretExists(ctx context.Context, projectID, secretID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.SecretManagerTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/secrets/%s", projectID, secretID)
	_, err := c.service.Projects.Secrets.Get(name).Context(ctx).Do()
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, wrapError("get secret", err)
}

func (c *defaultSecretManagerClient) AccessSecretVersion(
	ctx context.Context,
	projectID, secretID, version string,
) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.SecretManagerTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", projectID, secretID, version)
	resp, err := c.service.Projects.Secrets.Versions.Access(name).Context(ctx).Do()
	if err != nil {
		return nil, wrapError("access secret version", err)
	}
	data, decodeErr := base64.StdEncoding.DecodeString(resp.Payload.Data)
	if decodeErr != nil {
		return nil, wrapError("decode secret version", decodeErr)
	}
	return data, nil
}

type defaultComputeClient struct {
	service *compute.Service
}

const computeOperationDone = "DONE"

func (c *defaultComputeClient) CreateVPC(ctx context.Context, projectID, vpcName string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.VPCOperationTimeout)
	defer cancel()

	// Create a custom subnet mode network (not legacy mode)
	// AutoCreateSubnetworks: false means custom subnet mode
	// RoutingConfig specifies regional routing mode
	network := &compute.Network{
		Name:                  vpcName,
		AutoCreateSubnetworks: false,
		RoutingConfig: &compute.NetworkRoutingConfig{
			RoutingMode: "REGIONAL",
		},
		// ForceSendFields ensures the API sees AutoCreateSubnetworks=false so the
		// network is created in subnet mode rather than legacy mode.
		ForceSendFields: []string{"AutoCreateSubnetworks"},
	}

	op, err := c.service.Networks.Insert(projectID, network).Context(ctx).Do()
	if isAlreadyExists(err) {
		return nil
	}
	if err != nil {
		return wrapError("create vpc network", err)
	}
	return wrapError("wait for vpc creation", c.waitForGlobalOperation(ctx, projectID, op.Name))
}

func (c *defaultComputeClient) DeleteVPC(ctx context.Context, projectID, vpcName string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.VPCOperationTimeout)
	defer cancel()

	op, err := c.service.Networks.Delete(projectID, vpcName).Context(ctx).Do()
	if isNotFound(err) {
		return nil
	}
	if err != nil {
		return wrapError("delete vpc network", err)
	}
	return wrapError("wait for vpc deletion", c.waitForGlobalOperation(ctx, projectID, op.Name))
}

func (c *defaultComputeClient) VPCExists(ctx context.Context, projectID, vpcName string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.VPCOperationTimeout)
	defer cancel()

	_, err := c.service.Networks.Get(projectID, vpcName).Context(ctx).Do()
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, wrapError("get vpc network", err)
}

func (c *defaultComputeClient) CreateSubnet(
	ctx context.Context,
	projectID, region, subnetName, vpcName, cidrRange string,
) error {
	ctx, cancel := context.WithTimeout(ctx, constants.VPCOperationTimeout)
	defer cancel()

	op, err := c.service.Subnetworks.Insert(projectID, region, &compute.Subnetwork{
		Name:        subnetName,
		IpCidrRange: cidrRange,
		Network:     fmt.Sprintf("projects/%s/global/networks/%s", projectID, vpcName),
	}).Context(ctx).Do()
	if isAlreadyExists(err) {
		return nil
	}
	if err != nil {
		return wrapError("create subnet", err)
	}
	return wrapError("wait for subnet creation", c.waitForRegionalOperation(ctx, projectID, region, op.Name))
}

func (c *defaultComputeClient) DeleteSubnet(ctx context.Context, projectID, region, subnetName string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.VPCOperationTimeout)
	defer cancel()

	op, err := c.service.Subnetworks.Delete(projectID, region, subnetName).Context(ctx).Do()
	if isNotFound(err) {
		return nil
	}
	if err != nil {
		return wrapError("delete subnet", err)
	}
	return wrapError("wait for subnet deletion", c.waitForRegionalOperation(ctx, projectID, region, op.Name))
}

func (c *defaultComputeClient) CreateFirewallRule(
	ctx context.Context,
	projectID, ruleName, vpcName, direction string,
	allowed []string,
) error {
	ctx, cancel := context.WithTimeout(ctx, constants.FirewallOperationTimeout)
	defer cancel()

	allowRules := make([]*compute.FirewallAllowed, 0, len(allowed))
	for _, proto := range allowed {
		allowRules = append(allowRules, &compute.FirewallAllowed{
			IPProtocol: proto,
		})
	}

	rule := &compute.Firewall{
		Name:         ruleName,
		Network:      fmt.Sprintf("projects/%s/global/networks/%s", projectID, vpcName),
		Direction:    direction,
		Allowed:      allowRules,
		SourceRanges: []string{"0.0.0.0/0"},
	}
	if direction == "EGRESS" {
		rule.SourceRanges = nil
		rule.DestinationRanges = []string{"0.0.0.0/0"}
	}

	op, err := c.service.Firewalls.Insert(projectID, rule).Context(ctx).Do()
	if isAlreadyExists(err) {
		return nil
	}
	if err != nil {
		return wrapError("create firewall rule", err)
	}
	return wrapError("wait for firewall creation", c.waitForGlobalOperation(ctx, projectID, op.Name))
}

func (c *defaultComputeClient) DeleteFirewallRule(ctx context.Context, projectID, ruleName string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.FirewallOperationTimeout)
	defer cancel()

	op, err := c.service.Firewalls.Delete(projectID, ruleName).Context(ctx).Do()
	if isNotFound(err) {
		return nil
	}
	if err != nil {
		return wrapError("delete firewall rule", err)
	}
	return wrapError("wait for firewall deletion", c.waitForGlobalOperation(ctx, projectID, op.Name))
}

func (c *defaultComputeClient) waitForGlobalOperation(ctx context.Context, projectID, opName string) error {
	for {
		op, err := c.service.GlobalOperations.Get(projectID, opName).Context(ctx).Do()
		if err != nil {
			return wrapError("poll compute global operation", err)
		}
		if op.Status == computeOperationDone {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				return fmt.Errorf("operation error: %s", op.Error.Errors[0].Message)
			}
			return nil
		}
		time.Sleep(constants.ResourcePollInterval)
	}
}

func (c *defaultComputeClient) waitForRegionalOperation(ctx context.Context, projectID, region, opName string) error {
	for {
		op, err := c.service.RegionOperations.Get(projectID, region, opName).Context(ctx).Do()
		if err != nil {
			return wrapError("poll compute regional operation", err)
		}
		if op.Status == computeOperationDone {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				return fmt.Errorf("operation error: %s", op.Error.Errors[0].Message)
			}
			return nil
		}
		time.Sleep(constants.ResourcePollInterval)
	}
}

type defaultIAMClient struct {
	iamService      *iam.Service
	resourceManager *cloudresourcemanager.Service
}

func (c *defaultIAMClient) CreateServiceAccount(
	ctx context.Context,
	projectID, accountID, displayName string,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.ServiceAccountTimeout)
	defer cancel()

	req := &iam.CreateServiceAccountRequest{
		AccountId: accountID,
		ServiceAccount: &iam.ServiceAccount{
			DisplayName: displayName,
		},
	}

	sa, err := c.iamService.Projects.ServiceAccounts.Create("projects/"+projectID, req).
		Context(ctx).
		Do()
	if err != nil {
		return "", wrapError("create service account", err)
	}
	return sa.Email, nil
}

func (c *defaultIAMClient) DeleteServiceAccount(ctx context.Context, projectID, accountEmail string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.ServiceAccountTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/serviceAccounts/%s", projectID, accountEmail)
	_, err := c.iamService.Projects.ServiceAccounts.Delete(name).Context(ctx).Do()
	if isNotFound(err) {
		return nil
	}
	return wrapError("delete service account", err)
}

func (c *defaultIAMClient) ServiceAccountExists(ctx context.Context, projectID, accountEmail string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.ServiceAccountTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/serviceAccounts/%s", projectID, accountEmail)
	_, err := c.iamService.Projects.ServiceAccounts.Get(name).Context(ctx).Do()
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, wrapError("get service account", err)
}

func (c *defaultIAMClient) AddIAMBinding(ctx context.Context, projectID, member, role string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.IAMBindingTimeout)
	defer cancel()

	resource := "projects/" + projectID
	policy, err := c.resourceManager.Projects.GetIamPolicy(resource, &cloudresourcemanager.GetIamPolicyRequest{}).
		Context(ctx).
		Do()
	if err != nil {
		return wrapError("get project iam policy", err)
	}

	if !crmBindingExists(policy.Bindings, role, member) {
		policy.Bindings = append(policy.Bindings, &cloudresourcemanager.Binding{
			Role:    role,
			Members: []string{member},
		})
	}

	_, err = c.resourceManager.Projects.SetIamPolicy(
		resource,
		&cloudresourcemanager.SetIamPolicyRequest{Policy: policy},
	).Context(ctx).Do()
	return wrapError("set project iam policy", err)
}

func (c *defaultIAMClient) RemoveIAMBinding(ctx context.Context, projectID, member, role string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.IAMBindingTimeout)
	defer cancel()

	resource := "projects/" + projectID
	policy, err := c.resourceManager.Projects.GetIamPolicy(resource, &cloudresourcemanager.GetIamPolicyRequest{}).
		Context(ctx).
		Do()
	if err != nil {
		return wrapError("get project iam policy", err)
	}

	policy.Bindings = removeCRMBinding(policy.Bindings, role, member)

	_, err = c.resourceManager.Projects.SetIamPolicy(
		resource,
		&cloudresourcemanager.SetIamPolicyRequest{Policy: policy},
	).Context(ctx).Do()
	return wrapError("set project iam policy", err)
}

func (c *defaultIAMClient) AddServiceAccountIAMBinding(
	ctx context.Context,
	projectID, serviceAccountEmail, member, role string,
) error {
	ctx, cancel := context.WithTimeout(ctx, constants.IAMBindingTimeout)
	defer cancel()

	resource := fmt.Sprintf("projects/%s/serviceAccounts/%s", projectID, serviceAccountEmail)
	policy, err := c.iamService.Projects.ServiceAccounts.GetIamPolicy(resource).
		Context(ctx).
		Do()
	if err != nil {
		return wrapError("get service account iam policy", err)
	}

	if !saBindingExists(policy.Bindings, role, member) {
		policy.Bindings = append(policy.Bindings, &iam.Binding{
			Role:    role,
			Members: []string{member},
		})
	}

	_, err = c.iamService.Projects.ServiceAccounts.SetIamPolicy(
		resource,
		&iam.SetIamPolicyRequest{Policy: policy},
	).Context(ctx).Do()
	return wrapError("set service account iam policy", err)
}

type defaultLoggingClient struct {
	service *logging.Service
}

func (c *defaultLoggingClient) CreateSink(ctx context.Context, projectID, sinkName, filter, destination string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.LoggingSinkTimeout)
	defer cancel()

	parent := "projects/" + projectID
	sink := &logging.LogSink{
		Name:        sinkName,
		Filter:      filter,
		Destination: destination,
	}

	_, err := c.service.Projects.Sinks.Create(parent, sink).
		UniqueWriterIdentity(true).
		Context(ctx).
		Do()
	if isAlreadyExists(err) {
		return nil
	}
	return wrapError("create log sink", err)
}

func (c *defaultLoggingClient) DeleteSink(ctx context.Context, projectID, sinkName string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.LoggingSinkTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/sinks/%s", projectID, sinkName)
	_, err := c.service.Projects.Sinks.Delete(name).Context(ctx).Do()
	if isNotFound(err) {
		return nil
	}
	return wrapError("delete log sink", err)
}

func (c *defaultLoggingClient) SinkExists(ctx context.Context, projectID, sinkName string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.LoggingSinkTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/sinks/%s", projectID, sinkName)
	_, err := c.service.Projects.Sinks.Get(name).Context(ctx).Do()
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, wrapError("get log sink", err)
}

func (c *defaultLoggingClient) CreateLogBucket(
	ctx context.Context,
	projectID, bucketID, location string,
	retentionDays int,
) error {
	ctx, cancel := context.WithTimeout(ctx, constants.LoggingSinkTimeout)
	defer cancel()

	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, location)
	bucket := &logging.LogBucket{
		RetentionDays: int64(retentionDays),
	}

	_, err := c.service.Projects.Locations.Buckets.Create(parent, bucket).
		BucketId(bucketID).
		Context(ctx).
		Do()
	if isAlreadyExists(err) {
		return nil
	}
	return wrapError("create log bucket", err)
}

type defaultArtifactRegistryClient struct {
	service *artifactregistry.Service
	region  string
}

func (c *defaultArtifactRegistryClient) CreateRepository(
	ctx context.Context,
	projectID, location, repoID string,
) error {
	ctx, cancel := context.WithTimeout(ctx, constants.ArtifactRegistryTimeout)
	defer cancel()

	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, location)
	repo := &artifactregistry.Repository{
		Format: "DOCKER",
	}

	op, err := c.service.Projects.Locations.Repositories.Create(parent, repo).
		RepositoryId(repoID).
		Context(ctx).
		Do()
	if isAlreadyExists(err) {
		return nil
	}
	if err != nil {
		return wrapError("create artifact registry repository", err)
	}
	return c.waitForOperation(ctx, op.Name)
}

func (c *defaultArtifactRegistryClient) DeleteRepository(
	ctx context.Context,
	projectID, location, repoID string,
) error {
	ctx, cancel := context.WithTimeout(ctx, constants.ArtifactRegistryTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/locations/%s/repositories/%s", projectID, location, repoID)
	op, err := c.service.Projects.Locations.Repositories.Delete(name).Context(ctx).Do()
	if isNotFound(err) {
		return nil
	}
	if err != nil {
		return wrapError("delete artifact registry repository", err)
	}
	return c.waitForOperation(ctx, op.Name)
}

func (c *defaultArtifactRegistryClient) RepositoryExists(
	ctx context.Context,
	projectID, location, repoID string,
) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.ArtifactRegistryTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/locations/%s/repositories/%s", projectID, location, repoID)
	_, err := c.service.Projects.Locations.Repositories.Get(name).Context(ctx).Do()
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, wrapError("get artifact registry repository", err)
}

func (c *defaultArtifactRegistryClient) waitForOperation(ctx context.Context, name string) error {
	for {
		op, err := c.service.Projects.Locations.Operations.Get(name).Context(ctx).Do()
		if err != nil {
			return wrapError("poll artifact registry operation", err)
		}
		if op.Done {
			if op.Error != nil {
				return fmt.Errorf("operation error: %s", op.Error.Message)
			}
			return nil
		}
		time.Sleep(constants.ResourcePollInterval)
	}
}

type defaultVPCAccessClient struct {
	service *vpcaccess.Service
}

type defaultServiceUsageClient struct {
	service *serviceusage.Service
}

func (c *defaultServiceUsageClient) EnableServices(ctx context.Context, projectID string, services []string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.ServiceUsageOperationTimeout)
	defer cancel()

	parent := "projects/" + projectID
	req := &serviceusage.BatchEnableServicesRequest{
		ServiceIds: services,
	}

	op, err := c.service.Services.BatchEnable(parent, req).Context(ctx).Do()
	if err != nil {
		return wrapError("batch enable services", err)
	}

	if op.Done {
		if op.Error != nil {
			return fmt.Errorf("batch enable services: %s", op.Error.Message)
		}
		return nil
	}

	return wrapError("wait for service enablement", c.waitForOperation(ctx, op.Name))
}

func (c *defaultServiceUsageClient) waitForOperation(ctx context.Context, name string) error {
	for {
		op, err := c.service.Operations.Get(name).Context(ctx).Do()
		if err != nil {
			return wrapError("poll service usage operation", err)
		}
		if op.Done {
			if op.Error != nil {
				return fmt.Errorf("operation error: %s", op.Error.Message)
			}
			return nil
		}
		time.Sleep(constants.ResourcePollInterval)
	}
}

func (c *defaultVPCAccessClient) CreateConnector(
	ctx context.Context,
	projectID, region, connectorName, network, ipRange string,
	minInstances, maxInstances int,
) error {
	ctx, cancel := context.WithTimeout(ctx, constants.VPCConnectorTimeout)
	defer cancel()

	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, region)
	connector := &vpcaccess.Connector{
		Name:         fmt.Sprintf("%s/connectors/%s", parent, connectorName),
		Network:      fmt.Sprintf("projects/%s/global/networks/%s", projectID, network),
		IpCidrRange:  ipRange,
		MinInstances: int64(minInstances),
		MaxInstances: int64(maxInstances),
		MachineType:  constants.VPCConnectorMachineType,
	}

	op, err := c.service.Projects.Locations.Connectors.Create(parent, connector).
		ConnectorId(connectorName).
		Context(ctx).
		Do()
	if isAlreadyExists(err) {
		return nil
	}
	if err != nil {
		return wrapError("create vpc connector", err)
	}

	return c.waitForOperation(ctx, op.Name)
}

func (c *defaultVPCAccessClient) DeleteConnector(ctx context.Context, projectID, region, connectorName string) error {
	ctx, cancel := context.WithTimeout(ctx, constants.VPCConnectorTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/locations/%s/connectors/%s", projectID, region, connectorName)
	op, err := c.service.Projects.Locations.Connectors.Delete(name).Context(ctx).Do()
	if isNotFound(err) {
		return nil
	}
	if err != nil {
		return wrapError("delete vpc connector", err)
	}
	return c.waitForOperation(ctx, op.Name)
}

func (c *defaultVPCAccessClient) ConnectorExists(
	ctx context.Context,
	projectID, region, connectorName string,
) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, constants.VPCConnectorTimeout)
	defer cancel()

	name := fmt.Sprintf("projects/%s/locations/%s/connectors/%s", projectID, region, connectorName)
	_, err := c.service.Projects.Locations.Connectors.Get(name).Context(ctx).Do()
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, wrapError("get vpc connector", err)
}

func (c *defaultVPCAccessClient) waitForOperation(ctx context.Context, name string) error {
	for {
		op, err := c.service.Projects.Locations.Operations.Get(name).Context(ctx).Do()
		if err != nil {
			return wrapError("poll vpc access operation", err)
		}
		if op.Done {
			if op.Error != nil {
				return fmt.Errorf("operation error: %s", op.Error.Message)
			}
			return nil
		}
		time.Sleep(constants.ResourcePollInterval)
	}
}

func toRunEnvVars(envVars map[string]string) []*run.GoogleCloudRunV2EnvVar {
	result := make([]*run.GoogleCloudRunV2EnvVar, 0, len(envVars))
	for k, v := range envVars {
		result = append(result, &run.GoogleCloudRunV2EnvVar{
			Name:  k,
			Value: v,
		})
	}
	return result
}

func wrapError(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", action, err)
}

func bindingExists(bindings []*run.GoogleIamV1Binding, role, member string) bool {
	for _, b := range bindings {
		if b.Role == role && slices.Contains(b.Members, member) {
			return true
		}
	}
	return false
}

func removeBinding(bindings []*run.GoogleIamV1Binding, role, member string) []*run.GoogleIamV1Binding {
	var result []*run.GoogleIamV1Binding
	for _, b := range bindings {
		if b.Role != role {
			result = append(result, b)
			continue
		}
		var members []string
		for _, m := range b.Members {
			if m != member {
				members = append(members, m)
			}
		}
		if len(members) > 0 {
			b.Members = members
			result = append(result, b)
		}
	}
	return result
}

func removeCRMBinding(bindings []*cloudresourcemanager.Binding, role, member string) []*cloudresourcemanager.Binding {
	var result []*cloudresourcemanager.Binding
	for _, b := range bindings {
		if b.Role != role {
			result = append(result, b)
			continue
		}
		var members []string
		for _, m := range b.Members {
			if m != member {
				members = append(members, m)
			}
		}
		if len(members) > 0 {
			b.Members = members
			result = append(result, b)
		}
	}
	return result
}

func crmBindingExists(bindings []*cloudresourcemanager.Binding, role, member string) bool {
	for _, b := range bindings {
		if b.Role == role && slices.Contains(b.Members, member) {
			return true
		}
	}
	return false
}

func saBindingExists(bindings []*iam.Binding, role, member string) bool {
	for _, b := range bindings {
		if b.Role == role && slices.Contains(b.Members, member) {
			return true
		}
	}
	return false
}

func isNotFound(err error) bool {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == http.StatusNotFound
	}
	return false
}

func isAlreadyExists(err error) bool {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == http.StatusConflict
	}
	return false
}
