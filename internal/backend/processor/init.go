// Package processor provides event processing functionality for cloud provider events.
// It handles asynchronous events from AWS services like EventBridge, CloudWatch Logs, and WebSocket lifecycle events.
package processor

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/auth/authorization"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	processorAws "runvoy/internal/providers/aws/processor"
)

// Initialize creates a new Processor configured for the backend provider specified in cfg.
// It returns an error if the context is canceled, timed out, or if an unknown provider is specified.
// Callers should handle errors and potentially panic if initialization fails during startup.
// Also initializes the Casbin enforcer for authorization.
//
// Supported cloud providers:
//   - "aws": Uses CloudWatch events for ECS task state changes and API Gateway for WebSocket events
//   - "gcp": (future) Google Cloud Pub/Sub and Cloud Tasks for event processing
func Initialize(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
) (Processor, error) {
	logger.Debug(fmt.Sprintf("initializing %s event processor", constants.ProjectName),
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
		if sdkErr := cfg.AWS.LoadSDKConfig(ctx); sdkErr != nil {
			return nil, fmt.Errorf("failed to load AWS SDK config: %w", sdkErr)
		}

		processor, initErr := processorAws.Initialize(ctx, cfg, enforcer, logger)
		if initErr != nil {
			return nil, fmt.Errorf("failed to initialize event processor AWS backend: %w", initErr)
		}

		return processor, nil

	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", cfg.BackendProvider, constants.AWS)
	}
}
