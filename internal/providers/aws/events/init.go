package aws

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"
	websocketAws "runvoy/internal/providers/aws/websocket"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Initialize constructs an AWS-backed events backend with all required dependencies.
func Initialize(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
) (*Backend, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	logger.Debug("DynamoDB backend configured", "context", map[string]string{
		"executions_table":            cfg.ExecutionsTable,
		"websocket_connections_table": cfg.WebSocketConnectionsTable,
		"websocket_tokens_table":      cfg.WebSocketTokensTable,
	})

	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.ExecutionsTable, logger)
	connectionRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.WebSocketConnectionsTable, logger)
	tokenRepo := dynamoRepo.NewTokenRepository(dynamoClient, cfg.WebSocketTokensTable, logger)

	websocketManager := websocketAws.NewManager(cfg, &awsCfg, connectionRepo, tokenRepo, logger)

	return NewBackend(executionRepo, websocketManager, logger), nil
}
