// Package events provides event processing functionality for cloud provider events.
package events

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	eventsAws "runvoy/internal/providers/aws/events"
)

// Initialize creates a new Processor configured for the backend provider specified in cfg.
// It returns an error if the context is canceled, timed out, or if an unknown provider is specified.
// Callers should handle errors and potentially panic if initialization fails during startup.
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

	switch cfg.BackendProvider {
	case constants.AWS:
		if err := cfg.AWS.LoadSDKConfig(ctx); err != nil {
			return nil, fmt.Errorf("failed to load AWS SDK config: %w", err)
		}

		processor, err := eventsAws.Initialize(ctx, cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS backend: %w", err)
		}
		logger.Debug(constants.ProjectName + " event processor initialized successfully")
		return processor, nil
	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", cfg.BackendProvider, constants.AWS)
	}
}
