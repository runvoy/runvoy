package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
	"runvoy/internal/backend/health"
	"runvoy/internal/backend/websocket"
	"runvoy/internal/constants"
	"runvoy/internal/database"

	"golang.org/x/sync/errgroup"
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
	// cpu: optional CPU value (e.g., 256, 1024). Defaults to 256 if nil.
	// memory: optional Memory value in MB (e.g., 512, 2048). Defaults to 512 if nil.
	// runtimePlatform: optional runtime platform (e.g., "Linux/ARM64", "Linux/X86_64"). Defaults to "Linux/ARM64" if nil.
	RegisterImage(
		ctx context.Context,
		image string,
		isDefault *bool,
		taskRoleName, taskExecutionRoleName *string,
		cpu, memory *int,
		runtimePlatform *string,
	) error
	// ListImages lists all registered Docker images.
	ListImages(ctx context.Context) ([]api.ImageInfo, error)
	// GetImage retrieves a single Docker image by ID or name.
	// Accepts either an ImageID (e.g., "alpine:latest-a1b2c3d4") or an image name (e.g., "alpine:latest").
	GetImage(ctx context.Context, image string) (*api.ImageInfo, error)
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
	healthManager health.Manager             // Health manager for resource reconciliation
	enforcer      *authorization.Enforcer    // Casbin enforcer for authorization
}

// NOTE: provider-specific configuration has been moved to sub packages (e.g., providers/aws/app).

// NewService creates a new service instance and initializes the enforcer with user roles from the database.
// Returns an error if the enforcer is configured but user roles cannot be loaded (critical initialization failure).
// If userRepo is nil, user-related operations will not be available.
// This allows the service to work without database dependencies for simple operations.
// If wsManager is nil, WebSocket URL generation will be skipped.
// If secretsRepo is nil, secrets operations will not be available.
// If healthManager is nil, health reconciliation will not be available.
// If enforcer is nil, authorization will not be available.
func NewService(
	userRepo database.UserRepository,
	executionRepo database.ExecutionRepository,
	connRepo database.ConnectionRepository,
	tokenRepo database.TokenRepository,
	runner Runner,
	log *slog.Logger,
	provider constants.BackendProvider,
	wsManager websocket.Manager,
	secretsRepo database.SecretsRepository,
	healthManager health.Manager,
	enforcer *authorization.Enforcer) (*Service, error) {
	svc := &Service{
		userRepo:      userRepo,
		executionRepo: executionRepo,
		connRepo:      connRepo,
		tokenRepo:     tokenRepo,
		runner:        runner,
		Logger:        log,
		Provider:      provider,
		wsManager:     wsManager,
		secretsRepo:   secretsRepo,
		healthManager: healthManager,
		enforcer:      enforcer,
	}

	if enforcer != nil && userRepo != nil {
		if err := svc.loadUserRoles(context.Background()); err != nil {
			return nil, err
		}
	}

	log.Debug("casbin authorization enforcer initialized successfully")
	log.Debug(fmt.Sprintf("%s %s orchestrator initialized successfully",
		constants.ProjectName, svc.Provider))
	return svc, nil
}

// loadUserRoles populates the Casbin enforcer with all user roles from the database.
// Returns an error if any role is invalid or fails to load (critical initialization failure).
func (s *Service) loadUserRoles(ctx context.Context) error {
	users, err := s.userRepo.ListUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to load users for enforcer initialization: %w", err)
	}

	g, _ := errgroup.WithContext(ctx)

	for _, user := range users {
		g.Go(func() error {
			role, roleErr := authorization.NewRole(user.Role)
			if roleErr != nil {
				s.Logger.Error("user has invalid role", "user", user.Email, "role", user.Role, "error", roleErr)
				return fmt.Errorf("user %s has invalid role: %w", user.Email, roleErr)
			}

			if addErr := s.enforcer.AddRoleForUser(user.Email, role); addErr != nil {
				s.Logger.Error("failed to add role for user to enforcer", "user", user.Email, "role", user.Role, "error", addErr)
				return fmt.Errorf("failed to add role for user %s to enforcer: %w", user.Email, addErr)
			}

			return nil
		})
	}

	if waitErr := g.Wait(); waitErr != nil {
		return errors.New("failed to load user roles into enforcer")
	}

	return nil
}

// GetEnforcer returns the Casbin enforcer for authorization checks.
// May be nil if authorization is not configured.
func (s *Service) GetEnforcer() *authorization.Enforcer {
	return s.enforcer
}
