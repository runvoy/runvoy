package processor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/runvoy/runvoy/internal/auth/authorization"
	"github.com/runvoy/runvoy/internal/config"
	"github.com/runvoy/runvoy/internal/constants"
	processorAws "github.com/runvoy/runvoy/internal/providers/aws/processor"
)

// ProviderInitializer constructs a processor for the configured backend.
type ProviderInitializer func(
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
	enforcer *authorization.Enforcer,
) (Processor, error)

type initializeOptions struct {
	providerInitializer ProviderInitializer
}

// InitializeOption configures processor initialization.
type InitializeOption func(*initializeOptions)

// WithProviderInitializer injects a custom provider initializer, enabling in-memory tests
// or alternate provider wiring without invoking cloud SDKs.
func WithProviderInitializer(initializer ProviderInitializer) InitializeOption {
	return func(opts *initializeOptions) {
		opts.providerInitializer = initializer
	}
}

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
	opts ...InitializeOption,
) (Processor, error) {
	options := initializeOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	logger.Debug(fmt.Sprintf("initializing %s event processor", constants.ProjectName),
		"provider", cfg.BackendProvider,
		"version", *constants.GetVersion(),
		"init_timeout", cfg.InitTimeout.String(),
	)

	enforcer, err := authorization.NewEnforcer(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize authorization enforcer: %w", err)
	}

	initializer, err := selectProviderInitializer(cfg.BackendProvider, options.providerInitializer)
	if err != nil {
		return nil, err
	}

	processor, initErr := initializer(ctx, cfg, logger, enforcer)
	if initErr != nil {
		return nil, fmt.Errorf("failed to initialize %s event processor: %w", cfg.BackendProvider, initErr)
	}
	return processor, nil
}

func selectProviderInitializer(
	provider constants.BackendProvider,
	override ProviderInitializer,
) (ProviderInitializer, error) {
	if override != nil {
		return override, nil
	}

	switch provider {
	case constants.AWS:
		return awsProviderInitializer, nil
	case constants.GCP:
		return nil, errors.New("GCP processor initializer not yet implemented")
	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", provider, constants.ProvidersString())
	}
}

func awsProviderInitializer(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
	enforcer *authorization.Enforcer,
) (Processor, error) {
	if sdkErr := cfg.AWS.LoadSDKConfig(ctx); sdkErr != nil {
		return nil, fmt.Errorf("failed to load AWS SDK config: %w", sdkErr)
	}

	processor, initErr := processorAws.Initialize(ctx, cfg, enforcer, logger)
	if initErr != nil {
		return nil, fmt.Errorf("failed to initialize event processor AWS backend: %w", initErr)
	}

	return processor, nil
}
