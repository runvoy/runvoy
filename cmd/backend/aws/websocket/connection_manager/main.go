// Package main implements the AWS Lambda connection manager for runvoy WebSocket API.
// It handles $connect and $disconnect events, storing/removing connections in DynamoDB.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type connectionManager struct {
	tableName string
	client    *dynamodb.Client
	logger    *slog.Logger
}

type connectionRecord struct {
	ConnectionID string `dynamodbav:"connection_id"`
	ExecutionID  string `dynamodbav:"execution_id"`
	CreatedAt    int64  `dynamodbav:"created_at"`
	ExpiresAt    int64  `dynamodbav:"expires_at"`
	LastLogIndex int64  `dynamodbav:"last_log_index"`
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}
	log := logger.Initialize(constants.Production, cfg.GetLogLevel())

	tableName := os.Getenv("RUNVOY_WEBSOCKET_CONNECTIONS_TABLE")
	if tableName == "" {
		log.Error("RUNVOY_WEBSOCKET_CONNECTIONS_TABLE environment variable is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)
	defer cancel()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Error("Failed to load AWS configuration", "error", err)
		os.Exit(1)
	}

	client := dynamodb.NewFromConfig(awsCfg)

	manager := &connectionManager{
		tableName: tableName,
		client:    client,
		logger:    log,
	}

	log.Debug("starting WebSocket connection manager Lambda handler")
	lambda.Start(manager.handleRequest)
}

func (m *connectionManager) handleRequest(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, m.logger)
	routeKey := event.RequestContext.RouteKey
	connectionID := event.RequestContext.ConnectionID

	reqLogger.Debug("received WebSocket event",
		"routeKey", routeKey,
		"connectionID", connectionID,
	)

	switch routeKey {
	case "$connect":
		return m.handleConnect(ctx, event, reqLogger)
	case "$disconnect":
		return m.handleDisconnect(ctx, event, reqLogger)
	default:
		reqLogger.Info("unhandled route key", "routeKey", routeKey)
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       "OK",
		}, nil
	}
}

func (m *connectionManager) handleConnect(ctx context.Context, event events.APIGatewayWebsocketProxyRequest, log *slog.Logger) (events.APIGatewayProxyResponse, error) {
	connectionID := event.RequestContext.ConnectionID
	executionID := event.QueryStringParameters["execution_id"]

	if executionID == "" {
		log.Warn("connection attempt without execution_id", "connectionID", connectionID)
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       "execution_id query parameter is required",
		}, nil
	}

	now := time.Now().Unix()
	expiresAt := now + 3600

	record := connectionRecord{
		ConnectionID: connectionID,
		ExecutionID:  executionID,
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
		LastLogIndex: 0,
	}

	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		log.Error("failed to marshal connection record", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Internal server error",
		}, nil
	}

	_, err = m.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &m.tableName,
		Item:      item,
	})
	if err != nil {
		log.Error("failed to store connection", "error", err, "connectionID", connectionID)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Internal server error",
		}, nil
	}

	log.Info("connection established",
		"connectionID", connectionID,
		"executionID", executionID,
	)

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "Connected",
	}, nil
}

func (m *connectionManager) handleDisconnect(ctx context.Context, event events.APIGatewayWebsocketProxyRequest, log *slog.Logger) (events.APIGatewayProxyResponse, error) {
	connectionID := event.RequestContext.ConnectionID

	_, err := m.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &m.tableName,
		Key: map[string]types.AttributeValue{
			"connection_id": &types.AttributeValueMemberS{
				Value: connectionID,
			},
		},
	})
	if err != nil {
		log.Error("failed to remove connection", "error", err, "connectionID", connectionID)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Internal server error",
		}, nil
	}

	log.Info("connection disconnected", "connectionID", connectionID)

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "Disconnected",
	}, nil
}

