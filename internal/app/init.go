// Package app provides the core application logic for runvoy.
// It initializes and manages the service layer.
package app

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	appAws "runvoy/internal/providers/aws/app"
)

// Initialize creates a new Service configured for the specified backend provider.
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
		"init_timeout", cfg.InitTimeout,
	)

	switch cfg.BackendProvider {
	case constants.AWS:
		awsDeps, err := appAws.Initialize(ctx, cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS dependencies: %w", err)
		}

		logger.Debug(constants.ProjectName + " orchestrator initialized successfully")

		return NewService(
			awsDeps.UserRepo,
			awsDeps.ExecutionRepo,
			awsDeps.ConnectionRepo,
			awsDeps.TokenRepo,
			awsDeps.Runner,
			logger,
			cfg.BackendProvider,
			awsDeps.WebSocketManager,
			awsDeps.SecretsRepo,
		), nil

	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", cfg.BackendProvider, constants.AWS)
	}
}
