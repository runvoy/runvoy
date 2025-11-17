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
	enforcer      *authorization.Enforcer    // Enforcer for authorization
}

// NOTE: provider-specific configuration has been moved to sub packages (e.g., providers/aws/app).

// NewService creates a new service instance and initializes the enforcer with user roles from the database.
// Returns an error if the enforcer is configured but user roles cannot be loaded (critical initialization failure).
// If userRepo is nil, user-related operations will not be available.
// This allows the service to work without database dependencies for simple operations.
// If wsManager is nil, WebSocket URL generation will be skipped.
// If secretsRepo is nil, secrets operations will not be available.
// If healthManager is nil, health reconciliation will not be available.
// enforcer must be non-nil; use a permissive test enforcer in tests if needed.
func NewService(
	ctx context.Context,
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
	if enforcer == nil {
		return nil, fmt.Errorf("enforcer is required and cannot be nil")
	}

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

	if userRepo != nil {
		if err := svc.loadUserRoles(ctx); err != nil {
			return nil, err
		}
	}
	if err := svc.loadResourceOwnerships(ctx); err != nil {
		return nil, err
	}

	log.Debug("casbin authorization enforcer initialized successfully")
	log.Debug(fmt.Sprintf("%s %s orchestrator initialized successfully",
		constants.ProjectName, svc.Provider))
	return svc, nil
}

// loadUserRoles populates the Casbin enforcer with all user roles from the database.
// Returns an error if user loading fails or if any enforcer operation fails.
// This function should only be called when userRepo is non-nil.
func (s *Service) loadUserRoles(ctx context.Context) error {
	if s.userRepo == nil {
		return nil
	}
	users, err := s.userRepo.ListUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to load users for enforcer initialization: %w", err)
	}

	g, _ := errgroup.WithContext(ctx)

	for _, user := range users {
		g.Go(func() error {
			role, roleErr := authorization.NewRole(user.Role)
			if roleErr != nil {
				return fmt.Errorf("user %s has invalid role %q: %w", user.Email, user.Role, roleErr)
			}

			if addErr := s.enforcer.AddRoleForUser(user.Email, role); addErr != nil {
				return fmt.Errorf("failed to add role %q for user %s to enforcer: %w", user.Role, user.Email, addErr)
			}

			return nil
		})
	}

	if waitErr := g.Wait(); waitErr != nil {
		return errors.New("failed to load user roles into enforcer")
	}

	return nil
}

// loadResourceOwnerships hydrates the enforcer with resource ownership mappings for secrets and executions.
// Returns an error if any ownership loading fails.
func (s *Service) loadResourceOwnerships(ctx context.Context) error {
	if err := s.hydrateSecretOwnerships(ctx); err != nil {
		return fmt.Errorf("failed to load secret ownerships: %w", err)
	}
	if err := s.hydrateExecutionOwnerships(ctx); err != nil {
		return fmt.Errorf("failed to load execution ownerships: %w", err)
	}

	return nil
}

func (s *Service) hydrateSecretOwnerships(ctx context.Context) error {
	if s.secretsRepo == nil {
		return nil
	}

	secrets, err := s.secretsRepo.ListSecrets(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to load secrets for enforcer initialization: %w", err)
	}

	for _, secret := range secrets {
		if secret == nil || secret.Name == "" || secret.CreatedBy == "" {
			return errors.New("secret is nil or missing required fields")
		}

		resourceID := fmt.Sprintf("secret:%s", secret.Name)
		if addErr := s.enforcer.AddOwnershipForResource(resourceID, secret.CreatedBy); addErr != nil {
			return fmt.Errorf("failed to add ownership for secret %s: %w", secret.Name, addErr)
		}
	}

	return nil
}

func (s *Service) hydrateExecutionOwnerships(ctx context.Context) error {
	if s.executionRepo == nil {
		return nil
	}

	executions, err := s.executionRepo.ListExecutions(ctx, 0, nil)
	if err != nil {
		return fmt.Errorf("failed to load executions for enforcer initialization: %w", err)
	}

	g, _ := errgroup.WithContext(ctx)

	for _, execution := range executions {
		if execution == nil || execution.ExecutionID == "" || execution.UserEmail == "" {
			return errors.New("execution is nil or missing required fields")
		}

		g.Go(func() error {
			resourceID := fmt.Sprintf("execution:%s", execution.ExecutionID)
			if addErr := s.enforcer.AddOwnershipForResource(resourceID, execution.UserEmail); addErr != nil {
				return fmt.Errorf("failed to add ownership for execution %s: %w", execution.ExecutionID, addErr)
			}

			return nil
		})
	}

	if waitErr := g.Wait(); waitErr != nil {
		return fmt.Errorf("failed to load execution ownerships into enforcer: %w", waitErr)
	}

	return nil
}

// GetEnforcer returns the Casbin enforcer for authorization checks.
func (s *Service) GetEnforcer() *authorization.Enforcer {
	return s.enforcer
}
