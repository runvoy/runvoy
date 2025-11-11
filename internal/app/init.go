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
	provider constants.BackendProvider,
	cfg *config.Config,
	logger *slog.Logger) (*Service, error) {
	logger.Debug(fmt.Sprintf("initializing %s orchestrator service", constants.ProjectName),
		"provider", provider,
		"version", *constants.GetVersion(),
		"init_timeout", cfg.InitTimeout,
	)

	var service *Service

	switch provider {
	case constants.AWS:
		awsDeps, err := appAws.Initialize(ctx, cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS dependencies: %w", err)
		}
		service = &Service{
			userRepo:      awsDeps.UserRepo,
			executionRepo: awsDeps.ExecutionRepo,
			connRepo:      awsDeps.ConnectionRepo,
			tokenRepo:     awsDeps.TokenRepo,
			runner:        awsDeps.Runner,
			wsManager:     awsDeps.WebSocketManager,
		}

	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", provider, constants.AWS)
	}

	logger.Debug(constants.ProjectName + " orchestrator initialized successfully")

	return service, nil
}
