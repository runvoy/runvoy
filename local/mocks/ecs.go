package mocks

import (
	"context"
	"fmt"
	"time"

	"runvoy/internal/api"
)

// MockECSService implements ECSService for local testing
type MockECSService struct {
	tasks map[string]string // executionID -> taskARN
}

// NewMockECSService creates a new mock ECS service
func NewMockECSService() *MockECSService {
	return &MockECSService{
		tasks: make(map[string]string),
	}
}

// StartTask simulates starting an ECS task
func (m *MockECSService) StartTask(ctx context.Context, req *api.ExecutionRequest, executionID string, userEmail string) (string, error) {
	// Simulate task ARN
	taskARN := fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:task/runvoy-cluster/%s", executionID)
	
	// Store the task
	m.tasks[executionID] = taskARN

	// Simulate async task execution
	go m.simulateTaskExecution(executionID, req.Command)

	return taskARN, nil
}

// GetTaskStatus simulates getting task status
func (m *MockECSService) GetTaskStatus(ctx context.Context, taskARN string) (string, error) {
	// Simple simulation - in real implementation, this would query ECS
	return "RUNNING", nil
}

// StopTask simulates stopping a task
func (m *MockECSService) StopTask(ctx context.Context, taskARN string) error {
	// In real implementation, this would stop the ECS task
	return nil
}

// simulateTaskExecution simulates a task running
func (m *MockECSService) simulateTaskExecution(executionID, command string) {
	// Simulate task running for a few seconds
	time.Sleep(2 * time.Second)
	
	// In a real implementation, this would update the execution status
	// via a callback or by polling the task status
	fmt.Printf("Mock task %s completed command: %s\n", executionID, command)
}