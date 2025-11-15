// Package orchestrator provides the core orchestrator service for runvoy.
// It initializes and manages command execution and API request handling.
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	awsOrchestrator "runvoy/internal/providers/aws/orchestrator"
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
		"init_timeout", cfg.InitTimeout.String(),
	)

	switch cfg.BackendProvider {
	case constants.AWS:
		awsDeps, err := awsOrchestrator.Initialize(ctx, cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS dependencies: %w", err)
		}

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
			awsDeps.HealthManager,
		), nil

	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", cfg.BackendProvider, constants.AWS)
	}
}
