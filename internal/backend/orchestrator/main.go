package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/runvoy/runvoy/internal/auth/authorization"
	"github.com/runvoy/runvoy/internal/backend/contract"
	"github.com/runvoy/runvoy/internal/constants"
	"github.com/runvoy/runvoy/internal/database"
)

// Service provides the core business logic for command execution and user management.
type Service struct {
	Region               string
	repos                database.Repositories
	taskManager          contract.TaskManager
	imageRegistry        contract.ImageRegistry
	logManager           contract.LogManager
	observabilityManager contract.ObservabilityManager
	Logger               *slog.Logger
	Provider             constants.BackendProvider
	wsManager            contract.WebSocketManager // WebSocket manager for generating URLs and managing connections
	healthManager        contract.HealthManager    // Health manager for resource reconciliation
	enforcer             *authorization.Enforcer   // Enforcer for authorization
}

// NOTE: provider-specific configuration has been moved to sub packages (e.g., providers/aws/app).

// NewService creates a new service instance and initializes the enforcer with user roles from the database.
// Returns an error if the enforcer is configured but user roles cannot be loaded (critical initialization failure).
// Core repositories (repos.User, repos.Execution) and enforcer are required for initialization and must be non-nil.
// WebSocket manager is required for log streaming token generation.
// If repos.Secrets is nil, secrets operations will not be available.
// If repos.Image is nil, image-by-request-ID queries will not be available.
// healthManager is required; initialization fails if it is nil.
func NewService(
	ctx context.Context,
	region string,
	repos *database.Repositories,
	taskManager contract.TaskManager,
	imageRegistry contract.ImageRegistry,
	logManager contract.LogManager,
	observabilityManager contract.ObservabilityManager,
	log *slog.Logger,
	provider constants.BackendProvider,
	wsManager contract.WebSocketManager,
	healthManager contract.HealthManager,
	enforcer *authorization.Enforcer) (*Service, error) {
	if enforcer == nil {
		return nil, fmt.Errorf("enforcer is required")
	}
	if repos == nil || repos.User == nil || repos.Execution == nil {
		return nil, fmt.Errorf("user and execution repositories are required")
	}
	if healthManager == nil {
		return nil, fmt.Errorf("healthManager is required")
	}
	if wsManager == nil {
		return nil, fmt.Errorf("wsManager is required")
	}

	svc := &Service{
		Region:               region,
		repos:                *repos,
		taskManager:          taskManager,
		imageRegistry:        imageRegistry,
		logManager:           logManager,
		observabilityManager: observabilityManager,
		Logger:               log,
		Provider:             provider,
		wsManager:            wsManager,
		healthManager:        healthManager,
		enforcer:             enforcer,
	}

	if err := enforcer.Hydrate(
		ctx,
		repos.User,
		repos.Execution,
		repos.Secrets,
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
func (s *Service) TaskManager() contract.TaskManager {
	return s.taskManager
}

// ImageRegistry returns the image management interface.
func (s *Service) ImageRegistry() contract.ImageRegistry {
	return s.imageRegistry
}

// LogManager returns the log management interface.
func (s *Service) LogManager() contract.LogManager {
	return s.logManager
}

// ObservabilityManager returns the observability interface.
func (s *Service) ObservabilityManager() contract.ObservabilityManager {
	return s.observabilityManager
}
