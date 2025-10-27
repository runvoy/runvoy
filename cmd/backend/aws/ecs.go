package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"runvoy/internal/api"
	"runvoy/internal/services"
)

// ECSService implements ECSService using AWS ECS
type ECSService struct {
	client *ecs.Client
}

// NewECSService creates a new ECS service
func NewECSService() *ECSService {
	// TODO: Initialize ECS client with proper configuration
	return &ECSService{
		// client: ecs.NewFromConfig(cfg),
	}
}

// StartTask starts an ECS task
func (e *ECSService) StartTask(ctx context.Context, req *api.ExecutionRequest, executionID string, userEmail string) (string, error) {
	// TODO: Implement ECS task start
	// This would use the ECS client to start a Fargate task
	// with the appropriate task definition, environment variables, etc.
	return "", fmt.Errorf("not implemented")
}

// GetTaskStatus gets the status of an ECS task
func (e *ECSService) GetTaskStatus(ctx context.Context, taskARN string) (string, error) {
	// TODO: Implement ECS task status check
	return "", fmt.Errorf("not implemented")
}

// StopTask stops an ECS task
func (e *ECSService) StopTask(ctx context.Context, taskARN string) error {
	// TODO: Implement ECS task stop
	return fmt.Errorf("not implemented")
}