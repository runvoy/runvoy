// Package main implements the AWS Lambda log forwarder for runvoy WebSocket API.
// It processes CloudWatch Logs subscription events and forwards them to connected WebSocket clients.
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type logForwarder struct {
	tableName     string
	apiEndpoint   string
	dynamoClient  *dynamodb.Client
	apigwClient   *apigatewaymanagementapi.Client
	logger        *slog.Logger
	executionIDRe *regexp.Regexp
}

type cloudWatchLogsEvent struct {
	MessageType         string     `json:"messageType"`
	Owner               string     `json:"owner"`
	LogGroup            string     `json:"logGroup"`
	LogStream           string     `json:"logStream"`
	SubscriptionFilters []string   `json:"subscriptionFilters"`
	LogEvents           []logEvent `json:"logEvents"`
}

type logEvent struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
}

type connectionRecord struct {
	ConnectionID string `dynamodbav:"connection_id"`
	ExecutionID  string `dynamodbav:"execution_id"`
	CreatedAt    int64  `dynamodbav:"created_at"`
	ExpiresAt    int64  `dynamodbav:"expires_at"`
	LastLogIndex int64  `dynamodbav:"last_log_index"`
}

type websocketMessage struct {
	ExecutionID string `json:"execution_id"`
	Timestamp   int64  `json:"timestamp"`
	Message     string `json:"message"`
	EventID     string `json:"event_id"`
	Index       int64  `json:"index"`
}

func main() {
	cfg := config.MustLoadEventProcessor()
	log := logger.Initialize(constants.Production, cfg.GetLogLevel())

	tableName := os.Getenv("RUNVOY_WEBSOCKET_CONNECTIONS_TABLE")
	if tableName == "" {
		log.Error("RUNVOY_WEBSOCKET_CONNECTIONS_TABLE environment variable is required")
		os.Exit(1)
	}

	apiEndpoint := os.Getenv("RUNVOY_WEBSOCKET_API_ENDPOINT")
	if apiEndpoint == "" {
		log.Error("RUNVOY_WEBSOCKET_API_ENDPOINT environment variable is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)
	defer cancel()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Error("Failed to load AWS configuration", "error", err)
		os.Exit(1)
	}

	executionIDRe := regexp.MustCompile(`^task/runner/(.+)$`)

	forwarder := &logForwarder{
		tableName:    tableName,
		apiEndpoint:  apiEndpoint,
		dynamoClient: dynamodb.NewFromConfig(awsCfg),
		apigwClient: apigatewaymanagementapi.NewFromConfig(awsCfg, func(o *apigatewaymanagementapi.Options) {
			o.BaseEndpoint = aws.String("https://" + apiEndpoint)
		}),
		logger:        log,
		executionIDRe: executionIDRe,
	}

	log.Debug("starting WebSocket log forwarder Lambda handler")
	lambda.Start(forwarder.handleRequest)
}

func (f *logForwarder) handleRequest(ctx context.Context, event map[string]interface{}) error {
	reqLogger := logger.DeriveRequestLogger(ctx, f.logger)

	awsLogsData, ok := event["awslogs"].(map[string]interface{})
	if !ok {
		reqLogger.Warn("missing awslogs data in event")
		return nil
	}

	dataStr, ok := awsLogsData["data"].(string)
	if !ok {
		reqLogger.Warn("missing data field in awslogs")
		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		reqLogger.Error("failed to decode base64 data", "error", err)
		return err
	}

	reader, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		reqLogger.Error("failed to create gzip reader", "error", err)
		return err
	}
	defer reader.Close()

	var logEvent cloudWatchLogsEvent
	if err := json.NewDecoder(reader).Decode(&logEvent); err != nil {
		reqLogger.Error("failed to decode log event", "error", err)
		return err
	}

	executionID, err := f.extractExecutionID(logEvent.LogStream)
	if err != nil {
		reqLogger.Debug("failed to extract execution_id from log stream", "logStream", logEvent.LogStream, "error", err)
		return nil
	}

	reqLogger.Debug("processing log events",
		"executionID", executionID,
		"logGroup", logEvent.LogGroup,
		"logStream", logEvent.LogStream,
		"eventCount", len(logEvent.LogEvents),
	)

	connections, err := f.getConnectionsByExecutionID(ctx, executionID)
	if err != nil {
		reqLogger.Error("failed to get connections", "error", err, "executionID", executionID)
		return err
	}

	if len(connections) == 0 {
		reqLogger.Debug("no active connections for execution", "executionID", executionID)
		return nil
	}

	for _, logEvt := range logEvent.LogEvents {
		if err := f.forwardLogEvent(ctx, executionID, logEvt, connections, reqLogger); err != nil {
			reqLogger.Error("failed to forward log event", "error", err, "eventID", logEvt.ID)
		}
	}

	return nil
}

func (f *logForwarder) extractExecutionID(logStream string) (string, error) {
	matches := f.executionIDRe.FindStringSubmatch(logStream)
	if len(matches) < 2 {
		return "", fmt.Errorf("log stream does not match expected pattern: %s", logStream)
	}
	return matches[1], nil
}

func (f *logForwarder) getConnectionsByExecutionID(ctx context.Context, executionID string) ([]connectionRecord, error) {
	result, err := f.dynamoClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &f.tableName,
		IndexName:              aws.String("execution_id-index"),
		KeyConditionExpression: aws.String("execution_id = :execution_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":execution_id": &types.AttributeValueMemberS{
				Value: executionID,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query connections: %w", err)
	}

	var connections []connectionRecord
	for _, item := range result.Items {
		var conn connectionRecord
		if err := attributevalue.UnmarshalMap(item, &conn); err != nil {
			f.logger.Warn("failed to unmarshal connection record", "error", err)
			continue
		}
		connections = append(connections, conn)
	}

	return connections, nil
}

func (f *logForwarder) forwardLogEvent(ctx context.Context, executionID string, logEvt logEvent, connections []connectionRecord, reqLogger *slog.Logger) error {
	for _, conn := range connections {
		index := conn.LastLogIndex + 1

		msg := websocketMessage{
			ExecutionID: executionID,
			Timestamp:   logEvt.Timestamp,
			Message:     logEvt.Message,
			EventID:     logEvt.ID,
			Index:       index,
		}

		msgJSON, err := json.Marshal(msg)
		if err != nil {
			reqLogger.Error("failed to marshal message", "error", err)
			continue
		}

		_, err = f.apigwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: &conn.ConnectionID,
			Data:         msgJSON,
		})

		if err != nil {
			var goneErr *apigwtypes.GoneException
			if errors.As(err, &goneErr) {
				reqLogger.Debug("connection gone, removing from DynamoDB", "connectionID", conn.ConnectionID)
				if delErr := f.removeConnection(ctx, conn.ConnectionID); delErr != nil {
					reqLogger.Error("failed to remove gone connection", "error", delErr, "connectionID", conn.ConnectionID)
				}
				continue
			}

			reqLogger.Error("failed to post to connection", "error", err, "connectionID", conn.ConnectionID)
			continue
		}

		if err := f.updateLastLogIndex(ctx, conn.ConnectionID, index); err != nil {
			reqLogger.Error("failed to update last_log_index", "error", err, "connectionID", conn.ConnectionID)
		}
	}

	return nil
}

func (f *logForwarder) removeConnection(ctx context.Context, connectionID string) error {
	_, err := f.dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &f.tableName,
		Key: map[string]types.AttributeValue{
			"connection_id": &types.AttributeValueMemberS{
				Value: connectionID,
			},
		},
	})
	return err
}

func (f *logForwarder) updateLastLogIndex(ctx context.Context, connectionID string, index int64) error {
	_, err := f.dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &f.tableName,
		Key: map[string]types.AttributeValue{
			"connection_id": &types.AttributeValueMemberS{
				Value: connectionID,
			},
		},
		UpdateExpression: aws.String("SET last_log_index = :index"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":index": &types.AttributeValueMemberN{
				Value: strconv.FormatInt(index, 10),
			},
		},
	})
	return err
}
