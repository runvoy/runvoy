package services

import (
	"context"
	"runvoy/internal/api"
)

// Service interfaces for dependency injection and testing

// AuthService handles authentication and authorization
type AuthService interface {
	ValidateAPIKey(ctx context.Context, apiKey string) (*api.User, error)
	GenerateAPIKey(ctx context.Context, email string) (string, error)
	RevokeAPIKey(ctx context.Context, email string) error
}

// ExecutionService handles execution management
type ExecutionService interface {
	StartExecution(ctx context.Context, req *api.ExecutionRequest, user *api.User) (*api.ExecutionResponse, error)
	GetExecution(ctx context.Context, executionID string) (*api.Execution, error)
	ListExecutions(ctx context.Context, userEmail string, limit int) ([]*api.Execution, error)
	UpdateExecutionStatus(ctx context.Context, executionID string, status string, exitCode int) error
}

// LockService handles lock management
type LockService interface {
	AcquireLock(ctx context.Context, lockName string, userEmail string, executionID string) error
	ReleaseLock(ctx context.Context, lockName string) error
	GetLockHolder(ctx context.Context, lockName string) (*api.Lock, error)
}

// StorageService handles data persistence
type StorageService interface {
	// User operations
	GetUserByAPIKey(ctx context.Context, apiKeyHash string) (*api.User, error)
	CreateUser(ctx context.Context, user *api.User) error
	UpdateUser(ctx context.Context, user *api.User) error
	DeleteUser(ctx context.Context, email string) error

	// Execution operations
	CreateExecution(ctx context.Context, execution *api.Execution) error
	GetExecution(ctx context.Context, executionID string) (*api.Execution, error)
	UpdateExecution(ctx context.Context, execution *api.Execution) error
	ListExecutions(ctx context.Context, userEmail string, limit int) ([]*api.Execution, error)

	// Lock operations
	CreateLock(ctx context.Context, lock *api.Lock) error
	GetLock(ctx context.Context, lockName string) (*api.Lock, error)
	DeleteLock(ctx context.Context, lockName string) error
}

// ECSService handles ECS task operations
type ECSService interface {
	StartTask(ctx context.Context, req *api.ExecutionRequest, executionID string, userEmail string) (string, error)
	GetTaskStatus(ctx context.Context, taskARN string) (string, error)
	StopTask(ctx context.Context, taskARN string) error
}

// LogService handles log operations
type LogService interface {
	GetLogs(ctx context.Context, executionID string, since time.Time) (string, error)
	GenerateLogURL(ctx context.Context, executionID string) (string, error)
}