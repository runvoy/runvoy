package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/database"
	appErrors "github.com/runvoy/runvoy/internal/errors"
	"github.com/runvoy/runvoy/internal/logger"
	awsconstants "github.com/runvoy/runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ConnectionRepository implements the database.ConnectionRepository interface using DynamoDB.
type ConnectionRepository struct {
	client    Client
	tableName string
	logger    *slog.Logger
}

// NewConnectionRepository creates a new DynamoDB-backed connection repository.
func NewConnectionRepository(
	client Client,
	tableName string,
	log *slog.Logger,
) database.ConnectionRepository {
	return &ConnectionRepository{
		client:    client,
		tableName: tableName,
		logger:    log,
	}
}

// connectionItem represents the structure stored in DynamoDB.
// This keeps the database schema separate from the API types.
type connectionItem struct {
	ConnectionID         string `dynamodbav:"connection_id"`
	ExecutionID          string `dynamodbav:"execution_id"`
	Functionality        string `dynamodbav:"functionality"`
	ExpiresAt            int64  `dynamodbav:"expires_at"`
	LastEventID          string `dynamodbav:"last_event_id,omitempty"`
	ClientIP             string `dynamodbav:"client_ip,omitempty"`
	Token                string `dynamodbav:"token,omitempty"`
	UserEmail            string `dynamodbav:"user_email,omitempty"`
	TokenRequestClientIP string `dynamodbav:"token_request_client_ip,omitempty"`
}

// toConnectionItem converts an api.WebSocketConnection to a connectionItem.
func toConnectionItem(conn *api.WebSocketConnection) *connectionItem {
	return &connectionItem{
		ConnectionID:         conn.ConnectionID,
		ExecutionID:          conn.ExecutionID,
		Functionality:        conn.Functionality,
		ExpiresAt:            conn.ExpiresAt,
		LastEventID:          conn.LastEventID,
		ClientIP:             conn.ClientIP,
		Token:                conn.Token,
		UserEmail:            conn.UserEmail,
		TokenRequestClientIP: conn.TokenRequestClientIP,
	}
}

// CreateConnection stores a new WebSocket connection record in DynamoDB.
func (r *ConnectionRepository) CreateConnection(
	ctx context.Context,
	connection *api.WebSocketConnection,
) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	item := toConnectionItem(connection)

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return appErrors.ErrDatabaseError("failed to marshal connection item", err)
	}

	logArgs := []any{
		"operation", "DynamoDB.PutItem",
		"table", r.tableName,
		"connection_id", connection.ConnectionID,
		"execution_id", connection.ExecutionID,
		"functionality", connection.Functionality,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	})
	if err != nil {
		return appErrors.ErrDatabaseError("failed to store connection", err)
	}

	reqLogger.Debug("connection stored successfully", "context", map[string]string{
		"connection_id": connection.ConnectionID,
		"execution_id":  connection.ExecutionID,
		"functionality": connection.Functionality,
	})
	return nil
}

// DeleteConnections removes WebSocket connections from DynamoDB by connection IDs using batch delete.
func (r *ConnectionRepository) DeleteConnections(ctx context.Context, connectionIDs []string) (int, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	if len(connectionIDs) == 0 {
		reqLogger.Debug("no connections to delete")
		return 0, nil
	}

	logArgs := []any{
		"operation", "DynamoDB.BatchWriteItem",
		"table", r.tableName,
		"connection_count", len(connectionIDs),
		"connection_ids", connectionIDs,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	deleteRequests, buildErr := r.buildDeleteRequests(connectionIDs)
	if buildErr != nil {
		return 0, buildErr
	}

	deletedCount, batchErr := r.executeBatchDeletes(ctx, deleteRequests)
	if batchErr != nil {
		return deletedCount, batchErr
	}

	reqLogger.Debug("connections deleted successfully", "context", map[string]any{
		"connections_count": deletedCount,
		"connection_ids":    connectionIDs,
	})

	return deletedCount, nil
}

// GetConnectionsByExecutionID retrieves all active WebSocket connection records for a given execution ID
// using the execution_id-index GSI.
func (r *ConnectionRepository) GetConnectionsByExecutionID(
	ctx context.Context,
	executionID string,
) ([]*api.WebSocketConnection, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	logArgs := []any{
		"operation", "DynamoDB.Query",
		"table", r.tableName,
		"index", "execution_id-index",
		"execution_id", executionID,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("execution_id-index"),
		KeyConditionExpression: aws.String("execution_id = :execution_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":execution_id": &types.AttributeValueMemberS{Value: executionID},
		},
	})
	if err != nil {
		return nil, appErrors.ErrDatabaseError("failed to query connections by execution ID", err)
	}

	if len(result.Items) == 0 {
		return []*api.WebSocketConnection{}, nil
	}

	connections := make([]*api.WebSocketConnection, 0, len(result.Items))
	connIDs := make([]string, 0, len(result.Items))

	for _, item := range result.Items {
		var connItem connectionItem
		if unmarshalErr := attributevalue.UnmarshalMap(item, &connItem); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to unmarshal connection item: %w", unmarshalErr)
		}
		connIDs = append(connIDs, connItem.ConnectionID)
		connections = append(connections, &api.WebSocketConnection{
			ConnectionID:         connItem.ConnectionID,
			ExecutionID:          connItem.ExecutionID,
			Functionality:        connItem.Functionality,
			ExpiresAt:            connItem.ExpiresAt,
			LastEventID:          connItem.LastEventID,
			ClientIP:             connItem.ClientIP,
			Token:                connItem.Token,
			UserEmail:            connItem.UserEmail,
			TokenRequestClientIP: connItem.TokenRequestClientIP,
		})
	}

	reqLogger.Debug("connections retrieved successfully", "context", map[string]any{
		"execution_id":      executionID,
		"connection_ids":    connIDs,
		"connections_count": len(connections),
	})

	return connections, nil
}

// UpdateLastEventID persists the last delivered event ID for a connection.
func (r *ConnectionRepository) UpdateLastEventID(ctx context.Context, connectionID, lastEventID string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, r.logger)

	if connectionID == "" {
		return errors.New("connection ID is required")
	}

	keyAV, err := attributevalue.MarshalMap(map[string]string{"connection_id": connectionID})
	if err != nil {
		return appErrors.ErrDatabaseError("failed to marshal connection key", err)
	}

	logArgs := []any{
		"operation", "DynamoDB.UpdateItem",
		"table", r.tableName,
		"connection_id", connectionID,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("calling external service", "context", logger.SliceToMap(logArgs))

	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(r.tableName),
		Key:              keyAV,
		UpdateExpression: aws.String("SET last_event_id = :last_event_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":last_event_id": &types.AttributeValueMemberS{Value: lastEventID},
		},
	})
	if err != nil {
		return appErrors.ErrDatabaseError("failed to update last event ID", err)
	}

	reqLogger.Debug("last event ID updated", "context", map[string]any{
		"connection_id": connectionID,
		"last_event_id": lastEventID,
	})

	return nil
}

// buildDeleteRequests creates WriteRequest objects for all connection IDs.
func (r *ConnectionRepository) buildDeleteRequests(connectionIDs []string) ([]types.WriteRequest, error) {
	deleteRequests := make([]types.WriteRequest, 0, len(connectionIDs))

	for _, connID := range connectionIDs {
		key := map[string]string{
			"connection_id": connID,
		}
		keyAV, err := attributevalue.MarshalMap(key)
		if err != nil {
			return nil, appErrors.ErrDatabaseError("failed to marshal connection key", err)
		}

		deleteRequests = append(deleteRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: keyAV,
			},
		})
	}

	return deleteRequests, nil
}

// executeBatchDeletes processes delete requests in batches respecting DynamoDB's BatchWriteItem limit.
func (r *ConnectionRepository) executeBatchDeletes(
	ctx context.Context,
	deleteRequests []types.WriteRequest,
) (int, error) {
	deletedCount := 0

	for i := 0; i < len(deleteRequests); i += awsconstants.DynamoDBBatchWriteLimit {
		end := min(i+awsconstants.DynamoDBBatchWriteLimit, len(deleteRequests))

		batchRequests := deleteRequests[i:end]

		_, err := r.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				r.tableName: batchRequests,
			},
		})
		if err != nil {
			return deletedCount, appErrors.ErrDatabaseError("failed to delete connections batch", err)
		}

		deletedCount += len(batchRequests)
	}

	return deletedCount, nil
}
