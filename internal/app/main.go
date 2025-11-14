package app

import (
	"context"
	"log/slog"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/app/websocket"
	"runvoy/internal/constants"
	"runvoy/internal/database"
)

// Runner abstracts provider-specific command execution (e.g., AWS ECS, GCP, etc.).
type Runner interface {
	// StartTask triggers an execution on the underlying platform and returns
	// a stable executionID and the task creation timestamp.
	// The createdAt timestamp comes from the provider (e.g., ECS CreatedAt) when available.
	StartTask(
		ctx context.Context,
		userEmail string,
		req *api.ExecutionRequest) (executionID string, createdAt *time.Time, err error)
	// KillTask terminates a running task identified by executionID.
	// Returns an error if the task is already terminated or cannot be terminated.
	KillTask(ctx context.Context, executionID string) error
	// RegisterImage registers a Docker image as a task definition in the execution platform.
	// isDefault: if true, explicitly set as default.
	// taskRoleName: optional custom task role name (if nil, uses default from config).
	// taskExecutionRoleName: optional custom task execution role name (if nil, uses default from config).
	RegisterImage(ctx context.Context, image string, isDefault *bool, taskRoleName *string, taskExecutionRoleName *string) error
	// ListImages lists all registered Docker images.
	ListImages(ctx context.Context) ([]api.ImageInfo, error)
	// RemoveImage removes a Docker image and deregisters its task definitions.
	RemoveImage(ctx context.Context, image string) error
	// FetchLogsByExecutionID retrieves logs for a specific execution.
	// Returns empty slice if logs are not available or not supported by the provider.
	FetchLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error)
}

// Service provides the core business logic for command execution and user management.
type Service struct {
	userRepo      database.UserRepository
	executionRepo database.ExecutionRepository
	connRepo      database.ConnectionRepository
	tokenRepo     database.TokenRepository
	runner        Runner
	Logger        *slog.Logger
	Provider      constants.BackendProvider
	wsManager     websocket.Manager          // WebSocket manager for generating URLs and managing connections
	secretsRepo   database.SecretsRepository // Repository for managing secrets
}

// NOTE: provider-specific configuration has been moved to subpackages (e.g., providers/aws/app).

// NewService creates a new service instance.
// If userRepo is nil, user-related operations will not be available.
// This allows the service to work without database dependencies for simple operations.
// If wsManager is nil, WebSocket URL generation will be skipped.
// If secretsRepo is nil, secrets operations will not be available.
func NewService(
	userRepo database.UserRepository,
	executionRepo database.ExecutionRepository,
	connRepo database.ConnectionRepository,
	tokenRepo database.TokenRepository,
	runner Runner,
	log *slog.Logger,
	provider constants.BackendProvider,
	wsManager websocket.Manager,
	secretsRepo database.SecretsRepository) *Service {
	return &Service{
		userRepo:      userRepo,
		executionRepo: executionRepo,
		connRepo:      connRepo,
		tokenRepo:     tokenRepo,
		runner:        runner,
		Logger:        log,
		Provider:      provider,
		wsManager:     wsManager,
		secretsRepo:   secretsRepo,
	}
}
