package aws

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/logger"
	appAws "runvoy/internal/providers/aws/app"
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"
	websocketAws "runvoy/internal/providers/aws/websocket"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Initialize constructs an AWS-backed events backend with all required dependencies.
func Initialize(
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
) (*Backend, error) {
	// Register AWS Lambda context extractor
	logger.RegisterContextExtractor(appAws.NewLambdaContextExtractor())

	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	if cfg.AWS == nil {
		return nil, fmt.Errorf("AWS configuration is required")
	}

	log.Debug("DynamoDB backend configured", "context", map[string]string{
		"executions_table":            cfg.AWS.ExecutionsTable,
		"websocket_connections_table": cfg.AWS.WebSocketConnectionsTable,
		"websocket_tokens_table":      cfg.AWS.WebSocketTokensTable,
	})

	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.AWS.ExecutionsTable, log)
	connectionRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.AWS.WebSocketConnectionsTable, log)
	tokenRepo := dynamoRepo.NewTokenRepository(dynamoClient, cfg.AWS.WebSocketTokensTable, log)

	websocketManager := websocketAws.NewManager(cfg, &awsCfg, connectionRepo, tokenRepo, log)

	return NewBackend(executionRepo, websocketManager, log), nil
}
