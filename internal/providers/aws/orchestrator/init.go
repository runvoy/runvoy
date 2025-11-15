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
	"runvoy/internal/providers/aws/websocket"

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
	WebSocketManager *websocket.Manager
	SecretsRepo      database.SecretsRepository
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
	wsManager := websocket.Initialize(cfg, repos.connectionRepo, repos.tokenRepo, log)

	log.Debug("AWS orchestrator initialized successfully", "context", map[string]string{
		"ecs_cluster":                cfg.AWS.ECSCluster,
		"subnet1":                    cfg.AWS.Subnet1,
		"subnet2":                    cfg.AWS.Subnet2,
		"security_group":             cfg.AWS.SecurityGroup,
		"log_group":                  cfg.AWS.LogGroup,
		"default_task_exec_role_arn": cfg.AWS.DefaultTaskExecRoleARN,
		"default_task_role_arn":      cfg.AWS.DefaultTaskRoleARN,
	})

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

	log.Debug("DynamoDB backend configured", "context", map[string]string{
		"api_keys_table":              cfg.AWS.APIKeysTable,
		"executions_table":            cfg.AWS.ExecutionsTable,
		"websocket_connections_table": cfg.AWS.WebSocketConnectionsTable,
		"websocket_tokens_table":      cfg.AWS.WebSocketTokensTable,
		"image_taskdefs_table":        cfg.AWS.ImageTaskDefsTable,
		"secrets_metadata_table":      cfg.AWS.SecretsMetadataTable,
	})

	log.Debug("SSM Parameter Store secrets backend configured", "context", map[string]string{
		"secrets_prefix":      cfg.AWS.SecretsPrefix,
		"secrets_kms_key_arn": cfg.AWS.SecretsKMSKeyARN,
	})

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
