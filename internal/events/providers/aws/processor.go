package aws

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	dynamorepo "runvoy/internal/database/dynamodb"
	"runvoy/internal/events"
	"runvoy/internal/websocket"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// NewProcessor builds an AWS-backed event processor with DynamoDB repositories and API Gateway WebSocket manager.
func NewProcessor(ctx context.Context, cfg *config.Config, log *slog.Logger) (*events.Processor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}
	if log == nil {
		return nil, fmt.Errorf("logger is required")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	dynamoClient := awsdynamodb.NewFromConfig(awsCfg)
	executionRepo := dynamorepo.NewExecutionRepository(dynamoClient, cfg.ExecutionsTable, log)
	connectionRepo := dynamorepo.NewConnectionRepository(dynamoClient, cfg.WebSocketConnectionsTable, log)
	tokenRepo := dynamorepo.NewTokenRepository(dynamoClient, cfg.WebSocketTokensTable, log)
	websocketManager := websocket.NewWebSocketManager(cfg, &awsCfg, connectionRepo, tokenRepo, log)

	log.Debug("event processor initialized",
		"context", map[string]string{
			"executions_table":             cfg.ExecutionsTable,
			"web_socket_connections_table": cfg.WebSocketConnectionsTable,
			"web_socket_tokens_table":      cfg.WebSocketTokensTable,
			"websocket_api_endpoint":       cfg.WebSocketAPIEndpoint,
		},
	)

	return events.NewProcessor(events.ProcessorDependencies{
		ExecutionRepo:    executionRepo,
		WebSocketManager: websocketManager,
	}, log)
}
