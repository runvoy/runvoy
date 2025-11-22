// Package orchestrator provides the core orchestrator service for runvoy.
// It initializes and manages command execution and API request handling.
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/auth/authorization"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	awsHealth "runvoy/internal/providers/aws/health"
	awsOrchestrator "runvoy/internal/providers/aws/orchestrator"
)

// Initialize creates a new Service configured for the specified backend provider.
// It returns an error if the context is canceled, timed out, or if an unknown provider is specified.
// Callers should handle errors and potentially panic if initialization fails during startup.
// Also initializes the Casbin enforcer for authorization.
//
// Supported cloud providers:
//   - "aws": Uses DynamoDB for storage, Fargate for execution
//   - "gcp": (future) E.g. using Google Cloud Run and Firestore for storage
func Initialize( //nolint:funlen // This is ok, initialization functions are naturally long
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger) (*Service, error) {
	logger.Debug(fmt.Sprintf("initializing %s orchestrator service", constants.ProjectName),
		"provider", cfg.BackendProvider,
		"version", *constants.GetVersion(),
		"init_timeout", cfg.InitTimeout.String(),
	)

	enforcer, err := authorization.NewEnforcer(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize authorization enforcer: %w", err)
	}

	switch cfg.BackendProvider {
	case constants.AWS:
		awsDeps, initErr := awsOrchestrator.Initialize(ctx, cfg, logger)
		if initErr != nil {
			return nil, fmt.Errorf("failed to initialize AWS dependencies: %w", initErr)
		}

		if awsDeps.HealthManager != nil {
			// Type assert to concrete type to access SetCasbinDependencies method
			// This is provider-specific initialization that happens after the enforcer is created
			if awsHealthManager, ok := awsDeps.HealthManager.(*awsHealth.Manager); ok {
				awsHealthManager.SetCasbinDependencies(
					awsDeps.UserRepo,
					awsDeps.ExecutionRepo,
					enforcer,
				)
			}
		}

		repos := database.Repositories{
			User:       awsDeps.UserRepo,
			Execution:  awsDeps.ExecutionRepo,
			Connection: awsDeps.ConnectionRepo,
			Token:      awsDeps.TokenRepo,
			Image:      awsDeps.ImageRepo,
			Secrets:    awsDeps.SecretsRepo,
		}

		svc, svcErr := NewService(
			ctx,
			&repos,
			awsDeps.TaskManager,
			awsDeps.ImageRegistry,
			awsDeps.LogManager,
			awsDeps.ObservabilityManager,
			logger,
			cfg.BackendProvider,
			awsDeps.WebSocketManager,
			awsDeps.HealthManager,
			enforcer,
		)
		if svcErr != nil {
			return nil, fmt.Errorf("failed to initialize service: %w", svcErr)
		}
		return svc, nil

	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", cfg.BackendProvider, constants.AWS)
	}
}
