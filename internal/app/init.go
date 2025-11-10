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
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
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

	// Get WebSocket endpoint from provider-specific config
	var wsEndpoint string
	switch provider {
	case constants.AWS:
		if cfg.AWS != nil {
			wsEndpoint = cfg.AWS.WebSocketAPIEndpoint
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
		wsEndpoint,
	), nil
}

// initializeAWSBackend sets up AWS-specific dependencies.
func initializeAWSBackend(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
) (*serviceDependencies, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	ecsClientInstance := ecs.NewFromConfig(awsCfg)

	if cfg.AWS == nil {
		return nil, fmt.Errorf("AWS configuration is required but not provided")
	}

	logger.Debug("DynamoDB backend configured", "context", map[string]string{
		"api_keys_table":              cfg.AWS.APIKeysTable,
		"executions_table":            cfg.AWS.ExecutionsTable,
		"pending_api_keys_table":      cfg.AWS.PendingAPIKeysTable,
		"websocket_connections_table": cfg.AWS.WebSocketConnectionsTable,
		"websocket_tokens_table":      cfg.AWS.WebSocketTokensTable,
	})

	userRepo := dynamoRepo.NewUserRepository(dynamoClient, cfg.AWS.APIKeysTable, cfg.AWS.PendingAPIKeysTable, logger)
	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.AWS.ExecutionsTable, logger)
	connRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.AWS.WebSocketConnectionsTable, logger)
	tokenRepo := dynamoRepo.NewTokenRepository(dynamoClient, cfg.AWS.WebSocketTokensTable, logger)

	awsExecCfg := &appAws.Config{
		ECSCluster:      cfg.AWS.ECSCluster,
		Subnet1:         cfg.AWS.Subnet1,
		Subnet2:         cfg.AWS.Subnet2,
		SecurityGroup:   cfg.AWS.SecurityGroup,
		LogGroup:        cfg.AWS.LogGroup,
		TaskExecRoleARN: cfg.AWS.TaskExecRoleARN,
		TaskRoleARN:     cfg.AWS.TaskRoleARN,
		Region:          awsCfg.Region,
	}
	runner := appAws.NewRunner(ecsClientInstance, awsExecCfg, logger)

	return &serviceDependencies{
		userRepo:      userRepo,
		executionRepo: executionRepo,
		connRepo:      connRepo,
		tokenRepo:     tokenRepo,
		runner:        runner,
	}, nil
}
