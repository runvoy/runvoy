package gcp

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/deploymentmanager/v2"
	"google.golang.org/api/serviceusage/v1"

	"github.com/runvoy/runvoy/internal/providers/gcp/constants"
)

func newDefaultServiceClients(ctx context.Context) (*ServiceClients, error) {
	deploymentService, err := deploymentmanager.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create deployment manager service: %w", err)
	}

	serviceUsageSvc, err := serviceusage.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("create service usage service: %w", err)
	}

	return &ServiceClients{
		ServiceUsage: &defaultServiceUsageClient{
			service: serviceUsageSvc,
		},
		DeploymentManager: &defaultDeploymentManagerClient{
			service: deploymentService,
		},
	}, nil
}

type defaultDeploymentManagerClient struct {
	service *deploymentmanager.Service
}

func (c *defaultDeploymentManagerClient) GetDeployment(
	ctx context.Context,
	projectID, deploymentName string,
) (*deploymentmanager.Deployment, error) {
	deployment, err := c.service.Deployments.Get(projectID, deploymentName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get deployment: %w", err)
	}
	return deployment, nil
}

func (c *defaultDeploymentManagerClient) CreateDeployment(
	ctx context.Context,
	projectID string,
	deployment *deploymentmanager.Deployment,
) (*deploymentmanager.Operation, error) {
	op, err := c.service.Deployments.Insert(projectID, deployment).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("create deployment: %w", err)
	}
	return op, nil
}

func (c *defaultDeploymentManagerClient) UpdateDeployment(
	ctx context.Context,
	projectID, deploymentName string,
	deployment *deploymentmanager.Deployment,
) (*deploymentmanager.Operation, error) {
	op, err := c.service.Deployments.Update(projectID, deploymentName, deployment).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("update deployment: %w", err)
	}
	return op, nil
}

func (c *defaultDeploymentManagerClient) DeleteDeployment(
	ctx context.Context,
	projectID, deploymentName string,
) (*deploymentmanager.Operation, error) {
	op, err := c.service.Deployments.Delete(projectID, deploymentName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("delete deployment: %w", err)
	}
	return op, nil
}

func (c *defaultDeploymentManagerClient) GetOperation(
	ctx context.Context,
	projectID, operationName string,
) (*deploymentmanager.Operation, error) {
	op, err := c.service.Operations.Get(projectID, operationName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get deployment operation: %w", err)
	}
	return op, nil
}

func (c *defaultDeploymentManagerClient) GetManifest(
	ctx context.Context,
	projectID, deploymentName, manifestName string,
) (*deploymentmanager.Manifest, error) {
	manifest, err := c.service.Manifests.Get(projectID, deploymentName, manifestName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get deployment manifest: %w", err)
	}
	return manifest, nil
}

type defaultServiceUsageClient struct {
	service *serviceusage.Service
}

func (c *defaultServiceUsageClient) EnableServices(
	ctx context.Context,
	projectID string,
	services []string,
) error {
	ctx, cancel := context.WithTimeout(ctx, constants.ServiceUsageOperationTimeout)
	defer cancel()

	parent := "projects/" + projectID
	req := &serviceusage.BatchEnableServicesRequest{
		ServiceIds: services,
	}

	op, err := c.service.Services.BatchEnable(parent, req).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("batch enable services: %w", err)
	}

	if op.Done {
		if op.Error != nil {
			return fmt.Errorf("batch enable services: %s", op.Error.Message)
		}
		return nil
	}

	return c.waitForOperation(ctx, op.Name)
}

func (c *defaultServiceUsageClient) waitForOperation(ctx context.Context, name string) error {
	for {
		op, err := c.service.Operations.Get(name).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("poll service usage operation: %w", err)
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
