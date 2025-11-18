package aws

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/auth/authorization"
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
	enforcer *authorization.Enforcer,
	log *slog.Logger,
) (*Processor, error) {
	logger.RegisterContextExtractor(awsOrchestrator.NewLambdaContextExtractor())

	if err := cfg.AWS.LoadSDKConfig(ctx); err != nil {
		return nil, fmt.Errorf("failed to load AWS SDK config: %w", err)
	}
	accountID, identityErr := identity.GetAccountID(ctx, cfg.AWS.SDKConfig, log)
	if identityErr != nil {
		return nil, fmt.Errorf("failed to get AWS account ID: %w", identityErr)
	}

	awsCfg := *cfg.AWS.SDKConfig
	dynamoSDKClient := dynamodb.NewFromConfig(awsCfg)
	ssmSDKClient := ssm.NewFromConfig(awsCfg)

	dynamoClient := dynamoRepo.NewClientAdapter(dynamoSDKClient)
	ssmClient := secrets.NewClientAdapter(ssmSDKClient)

	repos := awsDatabase.CreateRepositories(dynamoClient, ssmClient, cfg, log)
	websocketManager := websocket.Initialize(cfg, repos.ConnectionRepo, repos.TokenRepo, log)

	if err := enforcer.Hydrate(
		ctx,
		repos.UserRepo,
		repos.ExecutionRepo,
		repos.SecretsRepo,
		repos.ImageTaskDefRepo,
	); err != nil {
		return nil, fmt.Errorf("failed to hydrate enforcer: %w", err)
	}

	healthManager := initializeHealthManager(
		accountID,
		awsClient.NewECSClientAdapter(ecs.NewFromConfig(awsCfg)),
		ssmClient,
		awsClient.NewIAMClientAdapter(iam.NewFromConfig(awsCfg)),
		repos.ImageTaskDefRepo,
		repos.SecretsRepo,
		repos.UserRepo,
		repos.ExecutionRepo,
		enforcer,
		cfg,
		log,
	)

	log.Debug(fmt.Sprintf("%s %s event processor initialized successfully",
		constants.ProjectName, cfg.BackendProvider),
		"context", map[string]string{
			"executions_table":            cfg.AWS.ExecutionsTable,
			"websocket_connections_table": cfg.AWS.WebSocketConnectionsTable,
			"websocket_tokens_table":      cfg.AWS.WebSocketTokensTable,
		})

	return NewProcessor(repos.ExecutionRepo, websocketManager, healthManager, log), nil
}

func initializeHealthManager(
	accountID string,
	ecsClient awsClient.ECSClient,
	ssmClient secrets.Client,
	iamClient awsClient.IAMClient,
	imageTaskDefRepo awsHealth.ImageTaskDefRepository,
	secretsRepo database.SecretsRepository,
	userRepo database.UserRepository,
	executionRepo database.ExecutionRepository,
	enforcer *authorization.Enforcer,
	cfg *config.Config,
	log *slog.Logger,
) health.Manager {
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
		enforcer,
		healthCfg,
		log,
	)
}
