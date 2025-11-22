package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/auth/authorization"
	"runvoy/internal/backend/health"
	"runvoy/internal/backend/orchestrator/interfaces"
	"runvoy/internal/backend/websocket"
	"runvoy/internal/constants"
	"runvoy/internal/database"
)

// Service provides the core business logic for command execution and user management.
type Service struct {
	userRepo             database.UserRepository
	executionRepo        database.ExecutionRepository
	connRepo             database.ConnectionRepository
	tokenRepo            database.TokenRepository
	imageRepo            database.ImageRepository
	taskManager          interfaces.TaskManager
	imageRegistry        interfaces.ImageRegistry
	logManager           interfaces.LogManager
	observabilityManager interfaces.ObservabilityManager
	Logger               *slog.Logger
	Provider             constants.BackendProvider
	wsManager            websocket.Manager          // WebSocket manager for generating URLs and managing connections
	secretsRepo          database.SecretsRepository // Repository for managing secrets
	healthManager        health.Manager             // Health manager for resource reconciliation
	enforcer             *authorization.Enforcer    // Enforcer for authorization
}

// NOTE: provider-specific configuration has been moved to sub packages (e.g., providers/aws/app).

// NewService creates a new service instance and initializes the enforcer with user roles from the database.
// Returns an error if the enforcer is configured but user roles cannot be loaded (critical initialization failure).
// Core repositories (userRepo, executionRepo) and enforcer are required for initialization and must be non-nil.
// If wsManager is nil, WebSocket URL generation will be skipped.
// If secretsRepo is nil, secrets operations will not be available.
// If imageRepo is nil, image-by-request-ID queries will not be available.
// If healthManager is nil, health reconciliation will not be available.
func NewService(
	ctx context.Context,
	userRepo database.UserRepository,
	executionRepo database.ExecutionRepository,
	connRepo database.ConnectionRepository,
	tokenRepo database.TokenRepository,
	imageRepo database.ImageRepository,
	taskManager interfaces.TaskManager,
	imageRegistry interfaces.ImageRegistry,
	logManager interfaces.LogManager,
	observabilityManager interfaces.ObservabilityManager,
	log *slog.Logger,
	provider constants.BackendProvider,
	wsManager websocket.Manager,
	secretsRepo database.SecretsRepository,
	healthManager health.Manager,
	enforcer *authorization.Enforcer) (*Service, error) {
	svc := &Service{
		userRepo:             userRepo,
		executionRepo:        executionRepo,
		connRepo:             connRepo,
		tokenRepo:            tokenRepo,
		imageRepo:            imageRepo,
		taskManager:          taskManager,
		imageRegistry:        imageRegistry,
		logManager:           logManager,
		observabilityManager: observabilityManager,
		Logger:               log,
		Provider:             provider,
		wsManager:            wsManager,
		secretsRepo:          secretsRepo,
		healthManager:        healthManager,
		enforcer:             enforcer,
	}

	if err := enforcer.Hydrate(
		ctx,
		userRepo,
		executionRepo,
		secretsRepo,
		imageRegistry,
	); err != nil {
		return nil, fmt.Errorf("failed to hydrate enforcer: %w", err)
	}

	log.Debug("casbin authorization enforcer initialized successfully")
	log.Debug(fmt.Sprintf("%s %s orchestrator initialized successfully",
		constants.ProjectName, svc.Provider))
	return svc, nil
}

// GetEnforcer returns the Casbin enforcer for authorization checks.
func (s *Service) GetEnforcer() *authorization.Enforcer {
	return s.enforcer
}

// TaskManager returns the task execution interface.
func (s *Service) TaskManager() interfaces.TaskManager {
	return s.taskManager
}

// ImageRegistry returns the image management interface.
func (s *Service) ImageRegistry() interfaces.ImageRegistry {
	return s.imageRegistry
}

// LogManager returns the log management interface.
func (s *Service) LogManager() interfaces.LogManager {
	return s.logManager
}

// ObservabilityManager returns the observability interface.
func (s *Service) ObservabilityManager() interfaces.ObservabilityManager {
	return s.observabilityManager
}
