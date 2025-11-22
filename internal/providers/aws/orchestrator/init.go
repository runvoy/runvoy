package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/backend/contract"
	"runvoy/internal/config"
	"runvoy/internal/database"
	"runvoy/internal/logger"
	awsClient "runvoy/internal/providers/aws/client"
	awsDatabase "runvoy/internal/providers/aws/database"
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"
	awsHealth "runvoy/internal/providers/aws/health"
	"runvoy/internal/providers/aws/identity"
	"runvoy/internal/providers/aws/secrets"
	awsWebsocket "runvoy/internal/providers/aws/websocket"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// Dependencies bundles the AWS-backed implementations required by the app service.
type Dependencies struct {
	UserRepo             database.UserRepository
	ExecutionRepo        database.ExecutionRepository
	ConnectionRepo       database.ConnectionRepository
	TokenRepo            database.TokenRepository
	ImageRepo            database.ImageRepository
	TaskManager          contract.TaskManager
	ImageRegistry        contract.ImageRegistry
	LogManager           contract.LogManager
	ObservabilityManager contract.ObservabilityManager
	WebSocketManager     contract.WebSocketManager
	SecretsRepo          database.SecretsRepository
	HealthManager        contract.HealthManager
}

// Initialize prepares AWS service dependencies for the app package.
// Wraps the AWS SDK clients in adapters for improved testability.
func Initialize( //nolint:funlen // This is ok, lots of initializations required
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
) (*Dependencies, error) {
	logger.RegisterContextExtractor(NewLambdaContextExtractor())

	if err := cfg.AWS.LoadSDKConfig(ctx); err != nil {
		return nil, fmt.Errorf("failed to load AWS SDK config: %w", err)
	}

	accountID, err := identity.GetAccountID(ctx, cfg.AWS.SDKConfig, log)
	if err != nil {
		return nil, fmt.Errorf("failed to get AWS account ID: %w", err)
	}

	dynamoSDKClient := dynamodb.NewFromConfig(*cfg.AWS.SDKConfig)
	ecsSDKClient := ecs.NewFromConfig(*cfg.AWS.SDKConfig)
	ssmSDKClient := ssm.NewFromConfig(*cfg.AWS.SDKConfig)
	cwlSDKClient := cloudwatchlogs.NewFromConfig(*cfg.AWS.SDKConfig)
	iamSDKClient := iam.NewFromConfig(*cfg.AWS.SDKConfig)

	dynamoClient := dynamoRepo.NewClientAdapter(dynamoSDKClient)
	ecsClient := awsClient.NewECSClientAdapter(ecsSDKClient)
	ssmClient := secrets.NewClientAdapter(ssmSDKClient)
	cwlClient := awsClient.NewCloudWatchLogsClientAdapter(cwlSDKClient)
	iamClient := awsClient.NewIAMClientAdapter(iamSDKClient)

	repos := awsDatabase.CreateRepositories(dynamoClient, ssmClient, cfg, log)
	providerCfg := &Config{
		ECSCluster:             cfg.AWS.ECSCluster,
		Subnet1:                cfg.AWS.Subnet1,
		Subnet2:                cfg.AWS.Subnet2,
		SecurityGroup:          cfg.AWS.SecurityGroup,
		LogGroup:               cfg.AWS.LogGroup,
		DefaultTaskExecRoleARN: cfg.AWS.DefaultTaskExecRoleARN,
		DefaultTaskRoleARN:     cfg.AWS.DefaultTaskRoleARN,
		Region:                 cfg.AWS.SDKConfig.Region,
		AccountID:              accountID,
		SDKConfig:              cfg.AWS.SDKConfig,
	}
	// Provider implements all 4 orchestrator interfaces (TaskManager, ImageRegistry, LogManager, ObservabilityManager).
	// We assign the same instance to each interface field to maintain explicit interface types in Dependencies.
	provider := NewProvider(ecsClient, cwlClient, iamClient, repos.ImageTaskDefRepo, providerCfg, log)
	wsManager := awsWebsocket.Initialize(cfg, repos.ConnectionRepo, repos.TokenRepo, log)

	healthCfg := &awsHealth.Config{
		Region:                 cfg.AWS.SDKConfig.Region,
		AccountID:              accountID,
		DefaultTaskRoleARN:     cfg.AWS.DefaultTaskRoleARN,
		DefaultTaskExecRoleARN: cfg.AWS.DefaultTaskExecRoleARN,
		LogGroup:               cfg.AWS.LogGroup,
		SecretsPrefix:          cfg.AWS.SecretsPrefix,
	}
	healthManager := awsHealth.Initialize(
		ecsClient,
		ssmClient,
		iamClient,
		repos.ImageTaskDefRepo,
		repos.SecretsRepo,
		repos.UserRepo,
		repos.ExecutionRepo,
		nil,
		healthCfg,
		log,
	)

	return &Dependencies{
		UserRepo:             repos.UserRepo,
		ExecutionRepo:        repos.ExecutionRepo,
		ConnectionRepo:       repos.ConnectionRepo,
		TokenRepo:            repos.TokenRepo,
		ImageRepo:            repos.ImageTaskDefRepo,
		TaskManager:          provider,
		ImageRegistry:        provider,
		LogManager:           provider,
		ObservabilityManager: provider,
		WebSocketManager:     wsManager,
		SecretsRepo:          repos.SecretsRepo,
		HealthManager:        healthManager,
	}, nil
}
