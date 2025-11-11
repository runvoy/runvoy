package aws

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/database"
	"runvoy/internal/logger"
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"
	awsWebsocket "runvoy/internal/providers/aws/websocket"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// Dependencies bundles the AWS-backed implementations required by the app service.
type Dependencies struct {
	UserRepo         database.UserRepository
	ExecutionRepo    database.ExecutionRepository
	ConnectionRepo   database.ConnectionRepository
	TokenRepo        database.TokenRepository
	Runner           *Runner
	WebSocketManager *awsWebsocket.Manager
}

// Initialize prepares AWS service dependencies for the app package.
func Initialize(
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
) (*Dependencies, error) {
	logger.RegisterContextExtractor(NewLambdaContextExtractor())

	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	ecsClient := ecs.NewFromConfig(awsCfg)

	if cfg.AWS == nil {
		return nil, fmt.Errorf("AWS configuration is required")
	}

	log.Debug("DynamoDB backend configured", "context", map[string]string{
		"api_keys_table":              cfg.AWS.APIKeysTable,
		"executions_table":            cfg.AWS.ExecutionsTable,
		"pending_api_keys_table":      cfg.AWS.PendingAPIKeysTable,
		"websocket_connections_table": cfg.AWS.WebSocketConnectionsTable,
		"websocket_tokens_table":      cfg.AWS.WebSocketTokensTable,
	})

	userRepo := dynamoRepo.NewUserRepository(dynamoClient, cfg.AWS.APIKeysTable, cfg.AWS.PendingAPIKeysTable, log)
	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.AWS.ExecutionsTable, log)
	connectionRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.AWS.WebSocketConnectionsTable, log)
	tokenRepo := dynamoRepo.NewTokenRepository(dynamoClient, cfg.AWS.WebSocketTokensTable, log)

	runnerCfg := &Config{
		ECSCluster:      cfg.AWS.ECSCluster,
		Subnet1:         cfg.AWS.Subnet1,
		Subnet2:         cfg.AWS.Subnet2,
		SecurityGroup:   cfg.AWS.SecurityGroup,
		LogGroup:        cfg.AWS.LogGroup,
		TaskExecRoleARN: cfg.AWS.TaskExecRoleARN,
		TaskRoleARN:     cfg.AWS.TaskRoleARN,
		Region:          awsCfg.Region,
	}
	runner := NewRunner(ecsClient, runnerCfg, log)
	wsManager := awsWebsocket.NewManager(cfg, &awsCfg, connectionRepo, tokenRepo, log)

	return &Dependencies{
		UserRepo:         userRepo,
		ExecutionRepo:    executionRepo,
		ConnectionRepo:   connectionRepo,
		TokenRepo:        tokenRepo,
		Runner:           runner,
		WebSocketManager: wsManager,
	}, nil
}
