package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"runvoy/internal/api"
	"runvoy/internal/services"
	"runvoy/internal/testing"
)

func TestExecutionService_StartExecution(t *testing.T) {
	// Create mocks
	mockStorage := testing.NewMockStorageService(t)
	mockECS := testing.NewMockECSService(t)
	mockLock := testing.NewMockLockService(t)
	mockLog := testing.NewMockLogService(t)

	// Create service under test
	service := services.NewExecutionService(mockStorage, mockECS, mockLock, mockLog)

	// Setup test data
	user := &api.User{
		Email: "test@example.com",
		APIKey: "test-key",
		CreatedAt: time.Now(),
		Revoked: false,
	}

	req := &api.ExecutionRequest{
		Command: "echo hello world",
		Lock: "test-lock",
	}

	// Setup mock expectations
	mockLock.On("AcquireLock", mock.Anything, "test-lock", "test@example.com", mock.AnythingOfType("string")).Return(nil)
	mockECS.On("StartTask", mock.Anything, req, mock.AnythingOfType("string"), "test@example.com").Return("arn:aws:ecs:test", nil)
	mockStorage.On("CreateExecution", mock.Anything, mock.AnythingOfType("*api.Execution")).Return(nil)
	mockLog.On("GenerateLogURL", mock.Anything, mock.AnythingOfType("string")).Return("http://test.com/logs", nil)

	// Execute
	ctx := context.Background()
	resp, err := service.StartExecution(ctx, req, user)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.ExecutionID)
	assert.Equal(t, "arn:aws:ecs:test", resp.TaskARN)
	assert.Equal(t, "starting", resp.Status)
	assert.Equal(t, "http://test.com/logs", resp.LogURL)

	// Verify all mocks were called
	mockLock.AssertExpectations(t)
	mockECS.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockLog.AssertExpectations(t)
}

func TestExecutionService_StartExecution_LockConflict(t *testing.T) {
	// Create mocks
	mockStorage := testing.NewMockStorageService(t)
	mockECS := testing.NewMockECSService(t)
	mockLock := testing.NewMockLockService(t)
	mockLog := testing.NewMockLogService(t)

	// Create service under test
	service := services.NewExecutionService(mockStorage, mockECS, mockLock, mockLog)

	// Setup test data
	user := &api.User{
		Email: "test@example.com",
		APIKey: "test-key",
		CreatedAt: time.Now(),
		Revoked: false,
	}

	req := &api.ExecutionRequest{
		Command: "echo hello world",
		Lock: "test-lock",
	}

	// Setup mock to return lock conflict error
	mockLock.On("AcquireLock", mock.Anything, "test-lock", "test@example.com", mock.AnythingOfType("string")).Return(assert.AnError)

	// Execute
	ctx := context.Background()
	resp, err := service.StartExecution(ctx, req, user)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to acquire lock")

	// Verify only lock was called, not ECS or storage
	mockLock.AssertExpectations(t)
	mockECS.AssertNotCalled(t, "StartTask")
	mockStorage.AssertNotCalled(t, "CreateExecution")
}

func TestExecutionService_UpdateExecutionStatus(t *testing.T) {
	// Create mocks
	mockStorage := testing.NewMockStorageService(t)
	mockECS := testing.NewMockECSService(t)
	mockLock := testing.NewMockLockService(t)
	mockLog := testing.NewMockLogService(t)

	// Create service under test
	service := services.NewExecutionService(mockStorage, mockECS, mockLock, mockLog)

	// Setup test data
	executionID := "exec_123"
	execution := &api.Execution{
		ExecutionID: executionID,
		UserEmail: "test@example.com",
		Command: "echo hello world",
		LockName: "test-lock",
		StartedAt: time.Now(),
		Status: "running",
	}

	// Setup mock expectations
	mockStorage.On("GetExecution", mock.Anything, executionID).Return(execution, nil)
	mockStorage.On("UpdateExecution", mock.Anything, mock.AnythingOfType("*api.Execution")).Return(nil)
	mockLock.On("ReleaseLock", mock.Anything, "test-lock").Return(nil)

	// Execute
	ctx := context.Background()
	err := service.UpdateExecutionStatus(ctx, executionID, "completed", 0)

	// Assert
	assert.NoError(t, err)

	// Verify all mocks were called
	mockStorage.AssertExpectations(t)
	mockLock.AssertExpectations(t)
}