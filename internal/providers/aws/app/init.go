package aws

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/database"
	"runvoy/internal/logger"
	awsDatabase "runvoy/internal/providers/aws/database"
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"
	"runvoy/internal/providers/aws/secrets"
	awsWebsocket "runvoy/internal/providers/aws/websocket"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// Dependencies bundles the AWS-backed implementations required by the app service.
type Dependencies struct {
	UserRepo         database.UserRepository
	ExecutionRepo    database.ExecutionRepository
	ConnectionRepo   database.ConnectionRepository
	TokenRepo        database.TokenRepository
	Runner           *Runner
	WebSocketManager *awsWebsocket.Manager
	SecretsRepo      database.SecretsRepository
}

// Initialize prepares AWS service dependencies for the app package.
func Initialize(
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
) (*Dependencies, error) {
	logger.RegisterContextExtractor(NewLambdaContextExtractor())

	if cfg.AWS == nil {
		return nil, fmt.Errorf("AWS configuration is required")
	}

	if cfg.AWS.SDKConfig == nil {
		return nil, fmt.Errorf("AWS SDK configuration not loaded; call LoadSDKConfig first")
	}

	awsCfg := *cfg.AWS.SDKConfig
	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	ecsClient := ecs.NewFromConfig(awsCfg)
	ssmClient := ssm.NewFromConfig(awsCfg)

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

	// Create secrets repository with DynamoDB metadata and Parameter Store values
	dynamoSecretsRepo := dynamoRepo.NewSecretsRepository(dynamoClient, cfg.AWS.SecretsMetadataTable, log)
	valueStore := secrets.NewParameterStoreManager(ssmClient, cfg.AWS.SecretsPrefix, cfg.AWS.SecretsKMSKeyARN, log)
	secretsRepo := awsDatabase.NewSecretsRepository(dynamoSecretsRepo, valueStore, log)

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
	runner := NewRunner(ecsClient, runnerCfg, &awsCfg, log)
	wsManager := awsWebsocket.NewManager(cfg, &awsCfg, connectionRepo, tokenRepo, log)

	return &Dependencies{
		UserRepo:         userRepo,
		ExecutionRepo:    executionRepo,
		ConnectionRepo:   connectionRepo,
		TokenRepo:        tokenRepo,
		Runner:           runner,
		WebSocketManager: wsManager,
		SecretsRepo:      secretsRepo,
	}, nil
}
