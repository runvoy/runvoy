package aws

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/api"
	"runvoy/internal/config"
	"runvoy/internal/database"
	"runvoy/internal/logger"
	awsDatabase "runvoy/internal/providers/aws/database"
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"
	"runvoy/internal/providers/aws/secrets"
	awsWebsocket "runvoy/internal/providers/aws/websocket"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// SecretsManager is an interface for managing secrets.
// This is defined here to avoid circular imports with internal/app.
type SecretsManager interface {
	// CreateSecret creates a new secret.
	// The secret's CreatedBy field must be set by the caller.
	// Returns the created secret with all fields populated (including timestamps).
	CreateSecret(ctx context.Context, secret *api.Secret) (*api.Secret, error)

	// GetSecret retrieves a secret's metadata and value by name.
	GetSecret(ctx context.Context, name string) (*api.Secret, error)

	// ListSecrets retrieves all secrets with their values.
	ListSecrets(ctx context.Context) ([]*api.Secret, error)

	// UpdateSecret updates a secret's value and/or editable properties (description, keyName).
	// The secret's UpdatedBy field must be set by the caller.
	// The implementation always updates the UpdatedAt timestamp.
	// Returns the updated secret with all fields populated.
	UpdateSecret(ctx context.Context, name string, updates *api.UpdateSecretRequest, updatedBy string) (*api.Secret, error)

	// DeleteSecret deletes a secret and its value.
	DeleteSecret(ctx context.Context, name string) error
}

// Dependencies bundles the AWS-backed implementations required by the app service.
type Dependencies struct {
	UserRepo         database.UserRepository
	ExecutionRepo    database.ExecutionRepository
	ConnectionRepo   database.ConnectionRepository
	TokenRepo        database.TokenRepository
	Runner           *Runner
	WebSocketManager *awsWebsocket.Manager
	SecretsManager   SecretsManager
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
	ssmClient := ssm.NewFromConfig(awsCfg)

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
	runner := NewRunner(ecsClient, runnerCfg, log)
	wsManager := awsWebsocket.NewManager(cfg, &awsCfg, connectionRepo, tokenRepo, log)

	// Create an adapter that implements app.SecretsManager using the repository
	secretsManager := NewSecretsManagerAdapter(secretsRepo)

	return &Dependencies{
		UserRepo:         userRepo,
		ExecutionRepo:    executionRepo,
		ConnectionRepo:   connectionRepo,
		TokenRepo:        tokenRepo,
		Runner:           runner,
		WebSocketManager: wsManager,
		SecretsManager:   secretsManager,
	}, nil
}
