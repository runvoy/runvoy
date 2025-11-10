package aws

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/database"
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// Dependencies bundles the AWS-backed implementations required by the app service.
type Dependencies struct {
	UserRepo       database.UserRepository
	ExecutionRepo  database.ExecutionRepository
	ConnectionRepo database.ConnectionRepository
	TokenRepo      database.TokenRepository
	Runner         *Runner
}

// Initialize prepares AWS service dependencies for the app package.
func Initialize(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
) (*Dependencies, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	ecsClient := ecs.NewFromConfig(awsCfg)

	logger.Debug("DynamoDB backend configured", "context", map[string]string{
		"api_keys_table":              cfg.APIKeysTable,
		"executions_table":            cfg.ExecutionsTable,
		"pending_api_keys_table":      cfg.PendingAPIKeysTable,
		"websocket_connections_table": cfg.WebSocketConnectionsTable,
		"websocket_tokens_table":      cfg.WebSocketTokensTable,
	})

	userRepo := dynamoRepo.NewUserRepository(dynamoClient, cfg.APIKeysTable, cfg.PendingAPIKeysTable, logger)
	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.ExecutionsTable, logger)
	connectionRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.WebSocketConnectionsTable, logger)
	tokenRepo := dynamoRepo.NewTokenRepository(dynamoClient, cfg.WebSocketTokensTable, logger)

	runnerCfg := &Config{
		ECSCluster:      cfg.ECSCluster,
		Subnet1:         cfg.Subnet1,
		Subnet2:         cfg.Subnet2,
		SecurityGroup:   cfg.SecurityGroup,
		LogGroup:        cfg.LogGroup,
		TaskExecRoleARN: cfg.TaskExecRoleARN,
		TaskRoleARN:     cfg.TaskRoleARN,
		Region:          awsCfg.Region,
	}
	runner := NewRunner(ecsClient, runnerCfg, logger)

	return &Dependencies{
		UserRepo:       userRepo,
		ExecutionRepo:  executionRepo,
		ConnectionRepo: connectionRepo,
		TokenRepo:      tokenRepo,
		Runner:         runner,
	}, nil
}
