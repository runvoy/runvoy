package services

import (
	"context"
	"fmt"
	"time"

	"runvoy/internal/api"
)

// ExecutionService implements the ExecutionService interface
type ExecutionService struct {
	storage StorageService
	ecs     ECSService
	lock    LockService
	log     LogService
}

// NewExecutionService creates a new execution service
func NewExecutionService(storage StorageService, ecs ECSService, lock LockService, log LogService) *ExecutionService {
	return &ExecutionService{
		storage: storage,
		ecs:     ecs,
		lock:    lock,
		log:     log,
	}
}

// StartExecution starts a new execution
func (s *ExecutionService) StartExecution(ctx context.Context, req *api.ExecutionRequest, user *api.User) (*api.ExecutionResponse, error) {
	// Generate execution ID
	executionID := generateExecutionID()

	// Acquire lock if requested
	if req.Lock != "" {
		if err := s.lock.AcquireLock(ctx, req.Lock, user.Email, executionID); err != nil {
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}
	}

	// Start ECS task
	taskARN, err := s.ecs.StartTask(ctx, req, executionID, user.Email)
	if err != nil {
		// Release lock if we acquired it
		if req.Lock != "" {
			s.lock.ReleaseLock(ctx, req.Lock)
		}
		return nil, fmt.Errorf("failed to start ECS task: %w", err)
	}

	// Create execution record
	execution := &api.Execution{
		ExecutionID:   executionID,
		UserEmail:     user.Email,
		Command:       req.Command,
		LockName:      req.Lock,
		TaskARN:       taskARN,
		StartedAt:     time.Now(),
		Status:        "starting",
		LogStreamName: fmt.Sprintf("exec/%s", executionID),
	}

	if err := s.storage.CreateExecution(ctx, execution); err != nil {
		// Clean up: stop task and release lock
		s.ecs.StopTask(ctx, taskARN)
		if req.Lock != "" {
			s.lock.ReleaseLock(ctx, req.Lock)
		}
		return nil, fmt.Errorf("failed to create execution record: %w", err)
	}

	// Generate log URL
	logURL, err := s.log.GenerateLogURL(ctx, executionID)
	if err != nil {
		// Non-critical error, continue
		logURL = ""
	}

	return &api.ExecutionResponse{
		ExecutionID: executionID,
		TaskARN:     taskARN,
		LogURL:      logURL,
		Status:      "starting",
	}, nil
}

// GetExecution retrieves an execution by ID
func (s *ExecutionService) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	return s.storage.GetExecution(ctx, executionID)
}

// ListExecutions lists executions for a user
func (s *ExecutionService) ListExecutions(ctx context.Context, userEmail string, limit int) ([]*api.Execution, error) {
	return s.storage.ListExecutions(ctx, userEmail, limit)
}

// UpdateExecutionStatus updates the status of an execution
func (s *ExecutionService) UpdateExecutionStatus(ctx context.Context, executionID string, status string, exitCode int) error {
	execution, err := s.storage.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get execution: %w", err)
	}

	execution.Status = status
	execution.ExitCode = exitCode

	if status == "completed" || status == "failed" {
		now := time.Now()
		execution.CompletedAt = &now
		execution.DurationSeconds = int(now.Sub(execution.StartedAt).Seconds())
	}

	if err := s.storage.UpdateExecution(ctx, execution); err != nil {
		return fmt.Errorf("failed to update execution: %w", err)
	}

	// Release lock if execution is complete
	if execution.LockName != "" && (status == "completed" || status == "failed") {
		if err := s.lock.ReleaseLock(ctx, execution.LockName); err != nil {
			// Log error but don't fail the update
			// TODO: Add proper logging
		}
	}

	return nil
}

// generateExecutionID generates a unique execution ID
func generateExecutionID() string {
	// Simple implementation - in production, use a more robust ID generator
	return fmt.Sprintf("exec_%d_%s", time.Now().Unix(), randomString(6))
}

// randomString generates a random string of specified length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}