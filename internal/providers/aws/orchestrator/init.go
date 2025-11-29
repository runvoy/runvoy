package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/runvoy/runvoy/internal/auth/authorization"
	"github.com/runvoy/runvoy/internal/backend/contract"
	"github.com/runvoy/runvoy/internal/config"
	awsconfig "github.com/runvoy/runvoy/internal/config/aws"
	"github.com/runvoy/runvoy/internal/database"
	"github.com/runvoy/runvoy/internal/logger"
	awsClient "github.com/runvoy/runvoy/internal/providers/aws/client"
	awsDatabase "github.com/runvoy/runvoy/internal/providers/aws/database"
	dynamoRepo "github.com/runvoy/runvoy/internal/providers/aws/database/dynamodb"
	awsHealth "github.com/runvoy/runvoy/internal/providers/aws/health"
	"github.com/runvoy/runvoy/internal/providers/aws/identity"
	"github.com/runvoy/runvoy/internal/providers/aws/secrets"
	awsWebsocket "github.com/runvoy/runvoy/internal/providers/aws/websocket"

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
func Initialize(
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
	enforcer *authorization.Enforcer,
) (*Dependencies, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	logger.RegisterContextExtractor(NewLambdaContextExtractor())

	clients, err := buildAWSClients(ctx, cfg, log)
	if err != nil {
		return nil, err
	}

	repos := awsDatabase.CreateRepositories(clients.dynamo, clients.ssm, cfg, log)
	providerCfg := buildProviderConfig(cfg, clients.accountID)

	managers := buildManagers(clients, repos, providerCfg, enforcer, log, cfg)

	return &Dependencies{
		UserRepo:             repos.UserRepo,
		ExecutionRepo:        repos.ExecutionRepo,
		ConnectionRepo:       repos.ConnectionRepo,
		TokenRepo:            repos.TokenRepo,
		ImageRepo:            repos.ImageTaskDefRepo,
		TaskManager:          managers.taskManager,
		ImageRegistry:        managers.imageRegistry,
		LogManager:           managers.logManager,
		ObservabilityManager: managers.observabilityManager,
		WebSocketManager:     managers.wsManager,
		SecretsRepo:          repos.SecretsRepo,
		HealthManager:        managers.healthManager,
	}, nil
}

type clientFactory struct {
	cfg *config.Config
	log *slog.Logger
}

type awsClients struct {
	dynamo    dynamoRepo.Client
	ecs       awsClient.ECSClient
	ssm       secrets.Client
	cwl       awsClient.CloudWatchLogsClient
	iam       awsClient.IAMClient
	accountID string
}

type managerSet struct {
	taskManager          contract.TaskManager
	imageRegistry        contract.ImageRegistry
	logManager           contract.LogManager
	observabilityManager contract.ObservabilityManager
	wsManager            contract.WebSocketManager
	healthManager        contract.HealthManager
}

func validateConfig(cfg *config.Config) error {
	if cfg == nil {
		return errors.New("config is required")
	}
	if cfg.AWS == nil {
		return errors.New("AWS config is required when backend_provider is AWS")
	}
	if err := awsconfig.ValidateOrchestrator(cfg.AWS); err != nil {
		return fmt.Errorf("invalid AWS orchestrator config: %w", err)
	}
	return nil
}

func buildAWSClients(ctx context.Context, cfg *config.Config, log *slog.Logger) (*awsClients, error) {
	factory := clientFactory{cfg: cfg, log: log}

	if err := factory.loadSDKConfig(ctx); err != nil {
		return nil, err
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

	return &awsClients{
		dynamo:    dynamoRepo.NewClientAdapter(dynamoSDKClient),
		ecs:       awsClient.NewECSClientAdapter(ecsSDKClient),
		ssm:       secrets.NewClientAdapter(ssmSDKClient),
		cwl:       awsClient.NewCloudWatchLogsClientAdapter(cwlSDKClient),
		iam:       awsClient.NewIAMClientAdapter(iamSDKClient),
		accountID: accountID,
	}, nil
}

func (f *clientFactory) loadSDKConfig(ctx context.Context) error {
	if f.cfg.AWS.SDKConfig != nil {
		return nil
	}

	if err := f.cfg.AWS.LoadSDKConfig(ctx); err != nil {
		return fmt.Errorf("failed to load AWS SDK config: %w", err)
	}
	return nil
}

func buildProviderConfig(cfg *config.Config, accountID string) *Config {
	return &Config{
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
}

func buildManagers(
	clients *awsClients,
	repos *awsDatabase.Repositories,
	providerCfg *Config,
	enforcer *authorization.Enforcer,
	log *slog.Logger,
	cfg *config.Config,
) *managerSet {
	taskManager := NewTaskManager(clients.ecs, repos.ImageTaskDefRepo, providerCfg, log)
	imageRegistry := NewImageRegistry(clients.ecs, clients.iam, repos.ImageTaskDefRepo, providerCfg, log)
	logManager := NewLogManager(clients.cwl, providerCfg, log)
	observabilityManager := NewObservabilityManager(clients.cwl, log)
	wsManager := awsWebsocket.Initialize(cfg, repos.ConnectionRepo, repos.TokenRepo, repos.LogEventRepo, log)

	healthCfg := &awsHealth.Config{
		Region:                 cfg.AWS.SDKConfig.Region,
		AccountID:              clients.accountID,
		DefaultTaskRoleARN:     cfg.AWS.DefaultTaskRoleARN,
		DefaultTaskExecRoleARN: cfg.AWS.DefaultTaskExecRoleARN,
		LogGroup:               cfg.AWS.LogGroup,
		SecretsPrefix:          cfg.AWS.SecretsPrefix,
	}
	healthManager := awsHealth.Initialize(
		clients.ecs,
		clients.ssm,
		clients.iam,
		repos.ImageTaskDefRepo,
		repos.SecretsRepo,
		repos.UserRepo,
		repos.ExecutionRepo,
		enforcer,
		healthCfg,
		log,
	)

	return &managerSet{
		taskManager:          taskManager,
		imageRegistry:        imageRegistry,
		logManager:           logManager,
		observabilityManager: observabilityManager,
		wsManager:            wsManager,
		healthManager:        healthManager,
	}
}
