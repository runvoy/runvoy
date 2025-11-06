// Package app provides the core application logic for runvoy.
// It initializes and manages the service layer.
package app

import (
	"context"
	"fmt"
	"log/slog"

	appAws "runvoy/internal/app/aws"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	dynamoRepo "runvoy/internal/database/dynamodb"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
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
	logger.Debug(fmt.Sprintf("initializing %s service", constants.ProjectName),
		"provider", provider,
		"version", *constants.GetVersion(),
		"init_timeout_seconds", cfg.InitTimeout.Seconds(),
	)

	var (
		userRepo      database.UserRepository
		executionRepo database.ExecutionRepository
		logRepo       database.LogRepository
		runner        Runner
		err           error
	)

	switch provider {
	case constants.AWS:
		userRepo, executionRepo, logRepo, runner, err = initializeAWSBackend(ctx, cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", provider, constants.AWS)
	}

	logger.Debug(constants.ProjectName + " orchestrator initialized successfully")

	return NewService(userRepo, executionRepo, logRepo, runner, logger, provider, cfg.WebSocketAPIEndpoint), nil
}

// initializeAWSBackend sets up AWS-specific dependencies
func initializeAWSBackend(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger) (database.UserRepository, database.ExecutionRepository, database.LogRepository, Runner, error) {
	if cfg.APIKeysTable == "" {
		return nil, nil, nil, nil, fmt.Errorf("APIKeysTable cannot be empty")
	}

	if cfg.ExecutionsTable == "" {
		return nil, nil, nil, nil, fmt.Errorf("ExecutionsTable cannot be empty")
	}

	if cfg.PendingAPIKeysTable == "" {
		return nil, nil, nil, nil, fmt.Errorf("PendingAPIKeysTable cannot be empty")
	}

	if cfg.ExecutionLogsTable == "" {
		return nil, nil, nil, nil, fmt.Errorf("ExecutionLogsTable cannot be empty")
	}

	if cfg.ECSCluster == "" {
		return nil, nil, nil, nil, fmt.Errorf("ECSCluster cannot be empty")
	}

	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	ecsClientInstance := ecs.NewFromConfig(awsCfg)

	logger.Debug("DynamoDB backend configured", "context", map[string]string{
		"api_keys_table":         cfg.APIKeysTable,
		"executions_table":       cfg.ExecutionsTable,
		"execution_logs_table":   cfg.ExecutionLogsTable,
		"pending_api_keys_table": cfg.PendingAPIKeysTable,
	})

	userRepo := dynamoRepo.NewUserRepository(dynamoClient, cfg.APIKeysTable, cfg.PendingAPIKeysTable, logger)
	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.ExecutionsTable, logger)
	logRepo := dynamoRepo.NewLogRepository(dynamoClient, cfg.ExecutionLogsTable, logger)

	awsExecCfg := &appAws.Config{
		ECSCluster:      cfg.ECSCluster,
		Subnet1:         cfg.Subnet1,
		Subnet2:         cfg.Subnet2,
		SecurityGroup:   cfg.SecurityGroup,
		LogGroup:        cfg.LogGroup,
		TaskExecRoleARN: cfg.TaskExecRoleARN,
		TaskRoleARN:     cfg.TaskRoleARN,
		Region:          awsCfg.Region,
	}
	runner := appAws.NewRunner(ecsClientInstance, awsExecCfg, logger)

	return userRepo, executionRepo, logRepo, runner, nil
}
