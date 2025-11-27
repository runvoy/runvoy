package database

import (
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/database"
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"
	"runvoy/internal/providers/aws/secrets"
)

// Repositories bundles all AWS-backed database repositories.
type Repositories struct {
	UserRepo         database.UserRepository
	ExecutionRepo    database.ExecutionRepository
	ConnectionRepo   database.ConnectionRepository
	LogEventRepo     database.LogEventRepository
	TokenRepo        database.TokenRepository
	ImageTaskDefRepo *dynamoRepo.ImageTaskDefRepository
	SecretsRepo      database.SecretsRepository
}

// CreateRepositories creates all AWS-backed database repositories from the provided clients and configuration.
func CreateRepositories(
	dynamoClient dynamoRepo.Client,
	ssmClient secrets.Client,
	cfg *config.Config,
	log *slog.Logger,
) *Repositories {
	userRepo := dynamoRepo.NewUserRepository(dynamoClient, cfg.AWS.APIKeysTable, cfg.AWS.PendingAPIKeysTable, log)
	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.AWS.ExecutionsTable, log)
	connectionRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.AWS.WebSocketConnectionsTable, log)
	logEventRepo := dynamoRepo.NewLogEventRepository(dynamoClient, cfg.AWS.ExecutionLogsTable, log)
	tokenRepo := dynamoRepo.NewTokenRepository(dynamoClient, cfg.AWS.WebSocketTokensTable, log)
	imageTaskDefRepo := dynamoRepo.NewImageTaskDefRepository(dynamoClient, cfg.AWS.ImageTaskDefsTable, log)
	dynamoSecretsRepo := dynamoRepo.NewSecretsRepository(dynamoClient, cfg.AWS.SecretsMetadataTable, log)

	valueStore := secrets.NewParameterStoreManager(ssmClient, cfg.AWS.SecretsPrefix, cfg.AWS.SecretsKMSKeyARN, log)
	secretsRepo := NewSecretsRepository(dynamoSecretsRepo, valueStore, log)

	log.Debug("DynamoDB backend configured", "context", map[string]string{
		"api_keys_table":              cfg.AWS.APIKeysTable,
		"executions_table":            cfg.AWS.ExecutionsTable,
		"execution_logs_table":        cfg.AWS.ExecutionLogsTable,
		"websocket_connections_table": cfg.AWS.WebSocketConnectionsTable,
		"websocket_tokens_table":      cfg.AWS.WebSocketTokensTable,
		"image_taskdefs_table":        cfg.AWS.ImageTaskDefsTable,
		"secrets_metadata_table":      cfg.AWS.SecretsMetadataTable,
	})

	log.Debug("SSM Parameter Store secrets backend configured", "context", map[string]string{
		"secrets_prefix":      cfg.AWS.SecretsPrefix,
		"secrets_kms_key_arn": cfg.AWS.SecretsKMSKeyARN,
	})

	return &Repositories{
		UserRepo:         userRepo,
		ExecutionRepo:    executionRepo,
		ConnectionRepo:   connectionRepo,
		LogEventRepo:     logEventRepo,
		TokenRepo:        tokenRepo,
		ImageTaskDefRepo: imageTaskDefRepo,
		SecretsRepo:      secretsRepo,
	}
}
