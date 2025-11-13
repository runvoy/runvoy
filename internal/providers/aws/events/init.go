package aws

import (
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/logger"
	appAws "runvoy/internal/providers/aws/app"
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"
	websocketAws "runvoy/internal/providers/aws/websocket"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Initialize constructs an AWS-backed event processor with all required dependencies.
func Initialize(
	cfg *config.Config,
	log *slog.Logger,
) (*Processor, error) {
	logger.RegisterContextExtractor(appAws.NewLambdaContextExtractor())

	awsCfg := *cfg.AWS.SDKConfig
	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	log.Debug("DynamoDB backend configured", "context", map[string]string{
		"executions_table":            cfg.AWS.ExecutionsTable,
		"websocket_connections_table": cfg.AWS.WebSocketConnectionsTable,
		"websocket_tokens_table":      cfg.AWS.WebSocketTokensTable,
	})

	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.AWS.ExecutionsTable, log)
	connectionRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.AWS.WebSocketConnectionsTable, log)
	tokenRepo := dynamoRepo.NewTokenRepository(dynamoClient, cfg.AWS.WebSocketTokensTable, log)

	websocketManager := websocketAws.NewManager(cfg, connectionRepo, tokenRepo, log)

	return NewProcessor(executionRepo, websocketManager, log), nil
}
