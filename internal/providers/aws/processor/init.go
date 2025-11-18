package aws

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/backend/health"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	"runvoy/internal/logger"
	awsClient "runvoy/internal/providers/aws/client"
	awsDatabase "runvoy/internal/providers/aws/database"
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"
	awsHealth "runvoy/internal/providers/aws/health"
	"runvoy/internal/providers/aws/identity"
	awsOrchestrator "runvoy/internal/providers/aws/orchestrator"
	"runvoy/internal/providers/aws/secrets"
	"runvoy/internal/providers/aws/websocket"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// Initialize constructs an AWS-backed event processor with all required dependencies.
// Wraps the AWS SDK clients in adapters for improved testability.
func Initialize(
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
) (*Processor, error) {
	logger.RegisterContextExtractor(awsOrchestrator.NewLambdaContextExtractor())

	if err := cfg.AWS.LoadSDKConfig(ctx); err != nil {
		return nil, fmt.Errorf("failed to load AWS SDK config: %w", err)
	}

	awsCfg := *cfg.AWS.SDKConfig
	dynamoSDKClient := dynamodb.NewFromConfig(awsCfg)
	ecsSDKClient := ecs.NewFromConfig(awsCfg)
	ssmSDKClient := ssm.NewFromConfig(awsCfg)
	iamSDKClient := iam.NewFromConfig(awsCfg)

	dynamoClient := dynamoRepo.NewClientAdapter(dynamoSDKClient)
	ecsClient := awsClient.NewECSClientAdapter(ecsSDKClient)
	ssmClient := secrets.NewClientAdapter(ssmSDKClient)
	iamClient := awsClient.NewIAMClientAdapter(iamSDKClient)

	userRepo := dynamoRepo.NewUserRepository(dynamoClient, cfg.AWS.APIKeysTable, cfg.AWS.PendingAPIKeysTable, log)
	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.AWS.ExecutionsTable, log)
	connectionRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.AWS.WebSocketConnectionsTable, log)
	tokenRepo := dynamoRepo.NewTokenRepository(dynamoClient, cfg.AWS.WebSocketTokensTable, log)
	imageTaskDefRepo := dynamoRepo.NewImageTaskDefRepository(dynamoClient, cfg.AWS.ImageTaskDefsTable, log)
	dynamoSecretsRepo := dynamoRepo.NewSecretsRepository(dynamoClient, cfg.AWS.SecretsMetadataTable, log)

	valueStore := secrets.NewParameterStoreManager(ssmClient, cfg.AWS.SecretsPrefix, cfg.AWS.SecretsKMSKeyARN, log)
	secretsRepo := awsDatabase.NewSecretsRepository(dynamoSecretsRepo, valueStore, log)

	websocketManager := websocket.Initialize(cfg, connectionRepo, tokenRepo, log)

	healthManager := initializeHealthManager(
		ctx, &awsCfg, ecsClient, ssmClient, iamClient, imageTaskDefRepo, secretsRepo, userRepo, executionRepo, cfg, log,
	)

	log.Debug(fmt.Sprintf("%s %s event processor initialized successfully",
		constants.ProjectName, cfg.BackendProvider),
		"context", map[string]string{
			"executions_table":            cfg.AWS.ExecutionsTable,
			"websocket_connections_table": cfg.AWS.WebSocketConnectionsTable,
			"websocket_tokens_table":      cfg.AWS.WebSocketTokensTable,
		})

	return NewProcessor(executionRepo, websocketManager, healthManager, log), nil
}

func initializeHealthManager(
	ctx context.Context,
	awsCfg *awsStd.Config,
	ecsClient awsClient.ECSClient,
	ssmClient secrets.Client,
	iamClient awsClient.IAMClient,
	imageTaskDefRepo awsHealth.ImageTaskDefRepository,
	secretsRepo database.SecretsRepository,
	userRepo database.UserRepository,
	executionRepo database.ExecutionRepository,
	cfg *config.Config,
	log *slog.Logger,
) health.Manager {
	accountID, err := identity.GetAccountID(ctx, awsCfg, log)
	if err != nil {
		log.Warn("failed to get AWS account ID, health manager will not be available", "error", err)
		return nil
	}

	healthCfg := &awsHealth.Config{
		Region:                 cfg.AWS.SDKConfig.Region,
		AccountID:              accountID,
		DefaultTaskRoleARN:     cfg.AWS.DefaultTaskRoleARN,
		DefaultTaskExecRoleARN: cfg.AWS.DefaultTaskExecRoleARN,
		LogGroup:               cfg.AWS.LogGroup,
		SecretsPrefix:          cfg.AWS.SecretsPrefix,
	}
	return awsHealth.Initialize(
		ecsClient,
		ssmClient,
		iamClient,
		imageTaskDefRepo,
		secretsRepo,
		userRepo,
		executionRepo,
		nil,
		healthCfg,
		log,
	)
}
