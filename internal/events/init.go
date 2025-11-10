// Package events provides event processing functionality for cloud provider events.
package events

import (
	"context"
	"fmt"
	"log/slog"

	eventsAws "runvoy/internal/events/aws"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	dynamoRepo "runvoy/internal/database/dynamodb"
	"runvoy/internal/websocket"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type processorDependencies struct {
	backend Backend
}

// Initialize creates a new Processor configured for the specified backend provider.
// It returns an error if the context is canceled, timed out, or if an unknown provider is specified.
//
// Supported cloud providers:
//   - "aws": Uses CloudWatch events for ECS task state changes and API Gateway for WebSocket events
func Initialize(
	ctx context.Context,
	provider constants.BackendProvider,
	cfg *config.Config,
	logger *slog.Logger,
) (*Processor, error) {
	logger.Debug("initializing event processor",
		"provider", provider,
		"version", *constants.GetVersion(),
		"init_timeout_seconds", cfg.InitTimeout.Seconds(),
	)

	var (
		backend Backend
		err     error
	)

	switch provider {
	case constants.AWS:
		var deps *processorDependencies
		deps, err = initializeAWSBackend(ctx, cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS backend: %w", err)
		}
		backend = deps.backend
	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", provider, constants.AWS)
	}

	logger.Debug("event processor initialized successfully")

	return NewProcessor(backend, logger), nil
}

// initializeAWSBackend sets up AWS-specific event processing dependencies.
func initializeAWSBackend(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
) (*processorDependencies, error) {
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

	websocketManager := websocket.NewWebSocketManager(cfg, &awsCfg, connectionRepo, tokenRepo, logger)

	backend := eventsAws.NewBackend(executionRepo, websocketManager, logger)

	return &processorDependencies{
		backend: backend,
	}, nil
}

// NewProcessorForAWS creates a new event processor with AWS backend.
// This is a convenience function for backward compatibility and testing.
// Deprecated: Use Initialize() instead for better multi-cloud support.
func NewProcessorForAWS(
	executionRepo database.ExecutionRepository,
	webSocketManager websocket.Manager,
	logger *slog.Logger,
) *Processor {
	backend := eventsAws.NewBackend(executionRepo, webSocketManager, logger)
	return NewProcessor(backend, logger)
}
