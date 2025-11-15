package aws

import (
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/logger"
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"
	"runvoy/internal/providers/aws/orchestrator"
	"runvoy/internal/providers/aws/websocket"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Initialize constructs an AWS-backed event processor with all required dependencies.
// Wraps the AWS SDK clients in adapters for improved testability.
func Initialize(
	cfg *config.Config,
	log *slog.Logger,
) (*Processor, error) {
	logger.RegisterContextExtractor(orchestrator.NewLambdaContextExtractor())

	awsCfg := *cfg.AWS.SDKConfig
	dynamoSDKClient := dynamodb.NewFromConfig(awsCfg)

	dynamoClient := dynamoRepo.NewClientAdapter(dynamoSDKClient)

	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.AWS.ExecutionsTable, log)
	connectionRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.AWS.WebSocketConnectionsTable, log)
	tokenRepo := dynamoRepo.NewTokenRepository(dynamoClient, cfg.AWS.WebSocketTokensTable, log)

	websocketManager := websocket.Initialize(cfg, connectionRepo, tokenRepo, log)

	log.Debug(fmt.Sprintf("%s %s event processor initialized successfully",
		constants.ProjectName, cfg.BackendProvider),
		"context", map[string]string{
			"executions_table":            cfg.AWS.ExecutionsTable,
			"websocket_connections_table": cfg.AWS.WebSocketConnectionsTable,
			"websocket_tokens_table":      cfg.AWS.WebSocketTokensTable,
		})

	return NewProcessor(executionRepo, websocketManager, log), nil
}
