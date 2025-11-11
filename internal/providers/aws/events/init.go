package aws

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	"runvoy/internal/logger"
	appAws "runvoy/internal/providers/aws/app"
	dynamoRepo "runvoy/internal/providers/aws/database/dynamodb"
	websocketAws "runvoy/internal/providers/aws/websocket"
	"runvoy/internal/websocket"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type backendDependencies struct {
	executionRepo    database.ExecutionRepository
	websocketManager websocket.Manager
}

// Initialize constructs an AWS-backed events backend with all required dependencies.
func Initialize(
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
) (*Backend, error) {
	var (
		deps *backendDependencies
		err  error
	)

	switch cfg.BackendProvider {
	case constants.AWS:
		deps, err = initializeAWSBackend(ctx, cfg, log)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS event backend: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown backend provider: %s (supported: %s)", cfg.BackendProvider, constants.AWS)
	}

	log.Debug("AWS event backend initialized successfully")
	return NewBackend(
		deps.executionRepo,
		deps.websocketManager,
		log,
	), nil
}

func initializeAWSBackend(
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
) (*backendDependencies, error) {
	logger.RegisterContextExtractor(appAws.NewLambdaContextExtractor())

	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	if cfg.AWS == nil {
		return nil, fmt.Errorf("AWS configuration is required")
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	log.Debug("DynamoDB backend configured", "context", map[string]string{
		"executions_table":            cfg.AWS.ExecutionsTable,
		"websocket_connections_table": cfg.AWS.WebSocketConnectionsTable,
		"websocket_tokens_table":      cfg.AWS.WebSocketTokensTable,
	})

	executionRepo := dynamoRepo.NewExecutionRepository(dynamoClient, cfg.AWS.ExecutionsTable, log)
	connectionRepo := dynamoRepo.NewConnectionRepository(dynamoClient, cfg.AWS.WebSocketConnectionsTable, log)
	tokenRepo := dynamoRepo.NewTokenRepository(dynamoClient, cfg.AWS.WebSocketTokensTable, log)

	websocketManager := websocketAws.NewManager(cfg, &awsCfg, connectionRepo, tokenRepo, log)

	return &backendDependencies{
		executionRepo:    executionRepo,
		websocketManager: websocketManager,
	}, nil
}
