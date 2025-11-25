package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/auth/authorization"
	"runvoy/internal/backend/contract"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	"runvoy/internal/logger"
	awsOrchestrator "runvoy/internal/providers/aws/orchestrator"
)

// ProviderDependencies groups the repositories and provider-specific managers required to build a Service.
// This enables injecting fake or prebuilt dependencies without touching cloud SDKs.
type ProviderDependencies struct {
	Region               string
	Repositories         database.Repositories
	TaskManager          contract.TaskManager
	ImageRegistry        contract.ImageRegistry
	LogManager           contract.LogManager
	ObservabilityManager contract.ObservabilityManager
	WebSocketManager     contract.WebSocketManager
	HealthManager        contract.HealthManager
}

// ProviderInitializer constructs provider dependencies given configuration and an enforcer instance.
type ProviderInitializer func(
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
	enforcer *authorization.Enforcer,
) (*ProviderDependencies, error)

type initializeOptions struct {
	providerInitializer ProviderInitializer
}

// InitializeOption configures initialization behavior.
type InitializeOption func(*initializeOptions)

// WithProviderInitializer injects a custom provider initializer, enabling in-memory tests
// or alternate provider wiring without invoking cloud SDKs.
func WithProviderInitializer(initializer ProviderInitializer) InitializeOption {
	return func(opts *initializeOptions) {
		opts.providerInitializer = initializer
	}
}

// Initialize creates a new Service configured for the specified backend provider.
// It returns an error if the context is canceled, timed out, or if an unknown provider is specified.
// Callers should handle errors and potentially panic if initialization fails during startup.
// Also initializes the Casbin enforcer for authorization.
//
// Supported cloud providers:
//   - "aws": Uses DynamoDB for storage, Fargate for execution
//   - "gcp": (future) E.g. using Google Cloud Run and Firestore for storage
func Initialize(
	ctx context.Context,
	cfg *config.Config,
	baseLogger *slog.Logger,
	opts ...InitializeOption,
) (*Service, error) {
	options := initializeOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, baseLogger)
	reqLogger.Debug(fmt.Sprintf("initializing %s orchestrator service", constants.ProjectName),
		"provider", cfg.BackendProvider,
		"version", *constants.GetVersion(),
		"init_timeout", cfg.InitTimeout.String(),
	)

	enforcer, err := authorization.NewEnforcer(baseLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize authorization enforcer: %w", err)
	}

	initializer, err := selectProviderInitializer(cfg.BackendProvider, options.providerInitializer)
	if err != nil {
		return nil, err
	}

	deps, initErr := initializer(ctx, cfg, baseLogger, enforcer)
	if initErr != nil {
		return nil, fmt.Errorf("failed to initialize %s dependencies: %w", cfg.BackendProvider, initErr)
	}

	svc, svcErr := NewService(
		ctx,
		deps.Region,
		&deps.Repositories,
		deps.TaskManager,
		deps.ImageRegistry,
		deps.LogManager,
		deps.ObservabilityManager,
		baseLogger,
		cfg.BackendProvider,
		deps.WebSocketManager,
		deps.HealthManager,
		enforcer,
	)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to initialize service: %w", svcErr)
	}
	return svc, nil
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
	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", provider, constants.AWS)
	}
}

func awsProviderInitializer(
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
	enforcer *authorization.Enforcer,
) (*ProviderDependencies, error) {
	awsDeps, err := awsOrchestrator.Initialize(ctx, cfg, log, enforcer)
	if err != nil {
		return nil, err
	}

	repos := database.Repositories{
		User:       awsDeps.UserRepo,
		Execution:  awsDeps.ExecutionRepo,
		Connection: awsDeps.ConnectionRepo,
		Token:      awsDeps.TokenRepo,
		Image:      awsDeps.ImageRepo,
		Secrets:    awsDeps.SecretsRepo,
	}

	return &ProviderDependencies{
		Region:               cfg.AWS.SDKConfig.Region,
		Repositories:         repos,
		TaskManager:          awsDeps.TaskManager,
		ImageRegistry:        awsDeps.ImageRegistry,
		LogManager:           awsDeps.LogManager,
		ObservabilityManager: awsDeps.ObservabilityManager,
		WebSocketManager:     awsDeps.WebSocketManager,
		HealthManager:        awsDeps.HealthManager,
	}, nil
}
