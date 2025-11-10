// Package app provides the core application logic for runvoy.
// It initializes and manages the service layer.
package app

import (
	"context"
	"fmt"
	"log/slog"

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
}

// Initialize creates a new Service configured for the specified backend provider.
// It returns an error if the context is canceled, timed out, or if an unknown provider is specified.
// Callers should handle errors and potentially panic if initialization fails during startup.
//
// Supported cloud providers:
//   - "aws": Uses DynamoDB for storage, Fargate for execution
//   - "gcp": (future) E.g. using Google Cloud Run and Firestore for storage
func Initialize(
	ctx context.Context,
	provider constants.BackendProvider,
	cfg *config.Config,
	logger *slog.Logger) (*Service, error) {
	logger.Debug(fmt.Sprintf("initializing %s orchestrator service", constants.ProjectName),
		"provider", provider,
		"version", *constants.GetVersion(),
		"init_timeout_seconds", cfg.InitTimeout.Seconds(),
	)

	var (
		userRepo      database.UserRepository
		executionRepo database.ExecutionRepository
		connRepo      database.ConnectionRepository
		tokenRepo     database.TokenRepository
		runner        Runner
		err           error
	)

	switch provider {
	case constants.AWS:
		var deps *serviceDependencies
		deps, err = initializeAWSBackend(ctx, cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS: %w", err)
		}

		userRepo = deps.userRepo
		executionRepo = deps.executionRepo
		connRepo = deps.connRepo
		tokenRepo = deps.tokenRepo
		runner = deps.runner
	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", provider, constants.AWS)
	}

	logger.Debug(constants.ProjectName + " orchestrator initialized successfully")

	// Get the WebSocket API endpoint based on provider
	var websocketEndpoint string
	switch provider {
	case constants.AWS:
		if cfg.AWS != nil {
			websocketEndpoint = cfg.AWS.WebSocketAPIEndpoint
		}
	}

	return NewService(
		userRepo,
		executionRepo,
		connRepo,
		tokenRepo,
		runner,
		logger,
		provider,
		websocketEndpoint,
	), nil
}

// initializeAWSBackend sets up AWS-specific dependencies.
func initializeAWSBackend(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
) (*serviceDependencies, error) {
	deps, err := appAws.Initialize(ctx, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AWS dependencies: %w", err)
	}

	return &serviceDependencies{
		userRepo:      deps.UserRepo,
		executionRepo: deps.ExecutionRepo,
		connRepo:      deps.ConnectionRepo,
		tokenRepo:     deps.TokenRepo,
		runner:        deps.Runner,
	}, nil
}
