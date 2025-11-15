package orchestrator

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

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
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
// Wraps the AWS SDK clients in adapters for improved testability.
func Initialize(
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
) (*Dependencies, error) {
	logger.RegisterContextExtractor(NewLambdaContextExtractor())

	if err := cfg.AWS.LoadSDKConfig(ctx); err != nil {
		return nil, fmt.Errorf("failed to load AWS SDK config: %w", err)
	}

	accountID, err := getAccountID(ctx, cfg.AWS.SDKConfig, log)
	if err != nil {
		return nil, fmt.Errorf("failed to get AWS account ID: %w", err)
	}

	dynamoSDKClient := dynamodb.NewFromConfig(*cfg.AWS.SDKConfig)
	ecsSDKClient := ecs.NewFromConfig(*cfg.AWS.SDKConfig)
	ssmSDKClient := ssm.NewFromConfig(*cfg.AWS.SDKConfig)
	cwlSDKClient := cloudwatchlogs.NewFromConfig(*cfg.AWS.SDKConfig)
	iamSDKClient := iam.NewFromConfig(*cfg.AWS.SDKConfig)

	dynamoClient := dynamoRepo.NewClientAdapter(dynamoSDKClient)
	ecsClient := NewClientAdapter(ecsSDKClient)
	ssmClient := secrets.NewClientAdapter(ssmSDKClient)
	cwlClient := NewCloudWatchLogsClientAdapter(cwlSDKClient)
	iamClient := NewIAMClientAdapter(iamSDKClient)

	repos := createRepositories(dynamoClient, ssmClient, cfg, log)
	runnerCfg := &Config{
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
	runner := NewRunner(ecsClient, cwlClient, iamClient, repos.imageTaskDefRepo, runnerCfg, log)
	wsManager := awsWebsocket.NewManager(cfg, repos.connectionRepo, repos.tokenRepo, log)

	return &Dependencies{
		UserRepo:         repos.userRepo,
		ExecutionRepo:    repos.executionRepo,
		ConnectionRepo:   repos.connectionRepo,
		TokenRepo:        repos.tokenRepo,
		Runner:           runner,
		WebSocketManager: wsManager,
		SecretsRepo:      repos.secretsRepo,
	}, nil
}

type repositories struct {
	userRepo         database.UserRepository
	executionRepo    database.ExecutionRepository
	connectionRepo   database.ConnectionRepository
	tokenRepo        database.TokenRepository
	imageTaskDefRepo ImageTaskDefRepository
	secretsRepo      database.SecretsRepository
}

func createRepositories(
	dynamoClient dynamoRepo.Client,
	ssmClient secrets.Client,
	cfg *config.Config,
	log *slog.Logger,
) *repositories {
	userRepo := dynamoRepo.NewUserRepository(dynamoClient, cfg.AWS.APIKeysTable, cfg.AWS.PendingAPIKeysTable, log)
	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.AWS.ExecutionsTable, log)
	connectionRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.AWS.WebSocketConnectionsTable, log)
	tokenRepo := dynamoRepo.NewTokenRepository(dynamoClient, cfg.AWS.WebSocketTokensTable, log)
	imageTaskDefRepo := dynamoRepo.NewImageTaskDefRepository(dynamoClient, cfg.AWS.ImageTaskDefsTable, log)
	dynamoSecretsRepo := dynamoRepo.NewSecretsRepository(dynamoClient, cfg.AWS.SecretsMetadataTable, log)

	valueStore := secrets.NewParameterStoreManager(ssmClient, cfg.AWS.SecretsPrefix, cfg.AWS.SecretsKMSKeyARN, log)
	secretsRepo := awsDatabase.NewSecretsRepository(dynamoSecretsRepo, valueStore, log)

	return &repositories{
		userRepo:         userRepo,
		executionRepo:    executionRepo,
		connectionRepo:   connectionRepo,
		tokenRepo:        tokenRepo,
		imageTaskDefRepo: imageTaskDefRepo,
		secretsRepo:      secretsRepo,
	}
}

// getAccountID retrieves the AWS account ID using STS GetCallerIdentity.
func getAccountID(ctx context.Context, awsCfg *awsStd.Config, log *slog.Logger) (string, error) {
	stsClient := sts.NewFromConfig(*awsCfg)

	log.Debug("calling external service", "context", map[string]string{
		"operation": "STS.GetCallerIdentity",
	})

	output, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("STS GetCallerIdentity failed: %w", err)
	}

	if output.Account == nil || *output.Account == "" {
		return "", fmt.Errorf("STS returned empty account ID")
	}

	accountID := *output.Account

	return accountID, nil
}
