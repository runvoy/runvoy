// Package app provides the core application logic for runvoy.
// It initializes and manages the service layer.
package app

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/app/websocket"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	appAws "runvoy/internal/providers/aws/app"
)

type serviceDependencies struct {
	userRepo      database.UserRepository
	executionRepo database.ExecutionRepository
	connRepo      database.ConnectionRepository
	tokenRepo     database.TokenRepository
	runner        Runner
	secretsRepo   database.SecretsRepository
}

// Initialize creates a new Service configured for the backend provider specified in cfg.
// It returns an error if the context is canceled, timed out, or if an unknown provider is specified.
// Callers should handle errors and potentially panic if initialization fails during startup.
//
// Supported cloud providers:
//   - "aws": Uses DynamoDB for storage, Fargate for execution
//   - "gcp": (future) E.g. using Google Cloud Run and Firestore for storage
func Initialize(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger) (*Service, error) {
	logger.Debug(fmt.Sprintf("initializing %s orchestrator service", constants.ProjectName),
		"provider", cfg.BackendProvider,
		"version", *constants.GetVersion(),
		"init_timeout_seconds", cfg.InitTimeout.Seconds(),
	)

	var (
		deps      *serviceDependencies
		wsManager websocket.Manager
	)

	switch cfg.BackendProvider {
	case constants.AWS:
		awsDeps, err := appAws.Initialize(ctx, cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS dependencies: %w", err)
		}
		deps = &serviceDependencies{
			userRepo:      awsDeps.UserRepo,
			executionRepo: awsDeps.ExecutionRepo,
			connRepo:      awsDeps.ConnectionRepo,
			tokenRepo:     awsDeps.TokenRepo,
			runner:        awsDeps.Runner,
			secretsRepo:   awsDeps.SecretsRepo,
		}
		if awsDeps.WebSocketManager != nil {
			wsManager = awsDeps.WebSocketManager
		}

	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", cfg.BackendProvider, constants.AWS)
	}

	logger.Debug(constants.ProjectName + " orchestrator initialized successfully")

	return NewService(
		deps.userRepo,
		deps.executionRepo,
		deps.connRepo,
		deps.tokenRepo,
		deps.runner,
		logger,
		cfg.BackendProvider,
		wsManager,
		deps.secretsRepo,
	), nil
}
