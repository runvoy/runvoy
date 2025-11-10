// Package events provides event processing functionality for cloud provider events.
package events

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"
	eventsAws "runvoy/internal/providers/aws/events"
	websocketAws "runvoy/internal/providers/aws/websocket"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type processorDependencies struct {
	backend Backend
}

// Initialize creates a new Processor configured for the backend provider specified in cfg.
// It returns an error if the context is canceled, timed out, or if an unknown provider is specified.
//
// Supported cloud providers:
//   - "aws": Uses CloudWatch events for ECS task state changes and API Gateway for WebSocket events
func Initialize(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
) (*Processor, error) {
	logger.Debug(fmt.Sprintf("initializing %s event processor", constants.ProjectName),
		"context", map[string]any{
			"provider":             cfg.BackendProvider,
			"version":              *constants.GetVersion(),
			"init_timeout_seconds": cfg.InitTimeout.Seconds(),
		},
	)

	var (
		backend Backend
		err     error
	)

	switch cfg.BackendProvider {
	case constants.AWS:
		var deps *processorDependencies
		deps, err = initializeAWSBackend(ctx, cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS backend: %w", err)
		}
		backend = deps.backend
	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", cfg.BackendProvider, constants.AWS)
	}

	logger.Debug(constants.ProjectName + " event processor initialized successfully")

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

	if cfg.AWS == nil {
		return nil, fmt.Errorf("AWS configuration is required but not provided")
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	logger.Debug("DynamoDB backend configured", "context", map[string]string{
		"executions_table":            cfg.AWS.ExecutionsTable,
		"websocket_connections_table": cfg.AWS.WebSocketConnectionsTable,
		"websocket_tokens_table":      cfg.AWS.WebSocketTokensTable,
	})

	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.AWS.ExecutionsTable, logger)
	connectionRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.AWS.WebSocketConnectionsTable, logger)
	tokenRepo := dynamoRepo.NewTokenRepository(dynamoClient, cfg.AWS.WebSocketTokensTable, logger)

	websocketManager := websocketAws.NewManager(cfg, &awsCfg, connectionRepo, tokenRepo, logger)

	backend := eventsAws.NewBackend(executionRepo, websocketManager, logger)

	return &processorDependencies{
		backend: backend,
	}, nil
}
