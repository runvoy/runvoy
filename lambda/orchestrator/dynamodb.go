package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Execution represents an execution record in DynamoDB
type Execution struct {
	ExecutionID     string `dynamodbav:"execution_id"`
	UserEmail       string `dynamodbav:"user_email"`
	Command         string `dynamodbav:"command"`
	LockName        string `dynamodbav:"lock_name,omitempty"`
	TaskArn         string `dynamodbav:"task_arn"`
	StartedAt       string `dynamodbav:"started_at"`
	CompletedAt     string `dynamodbav:"completed_at,omitempty"`
	Status          string `dynamodbav:"status"` // starting, running, completed, failed
	ExitCode        *int   `dynamodbav:"exit_code,omitempty"`
	DurationSeconds *int   `dynamodbav:"duration_seconds,omitempty"`
	LogStreamName   string `dynamodbav:"log_stream_name,omitempty"`
}

// Lock represents a lock record in DynamoDB
type Lock struct {
	LockName    string `dynamodbav:"lock_name"`
	ExecutionID string `dynamodbav:"execution_id"`
	UserEmail   string `dynamodbav:"user_email"`
	AcquiredAt  string `dynamodbav:"acquired_at"`
	TTL         int64  `dynamodbav:"ttl"` // Unix timestamp for auto-expiration
}

// tryAcquireLock attempts to acquire a lock
// Returns true if lock was acquired, false if already held
func tryAcquireLock(ctx context.Context, cfg *Config, lockName, executionID, userEmail string, timeoutSeconds int) (bool, *Lock, error) {
	if lockName == "" {
		return true, nil, nil // No lock requested
	}

	now := time.Now().UTC()
	ttl := now.Add(time.Duration(timeoutSeconds) * time.Second).Unix()

	lock := Lock{
		LockName:    lockName,
		ExecutionID: executionID,
		UserEmail:   userEmail,
		AcquiredAt:  now.Format(time.RFC3339),
		TTL:         ttl,
	}

	item, err := attributevalue.MarshalMap(lock)
	if err != nil {
		return false, nil, fmt.Errorf("failed to marshal lock: %w", err)
	}

	// Try to acquire lock with conditional write (fail if lock already exists)
	_, err = cfg.DynamoDBClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(cfg.LocksTable),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(lock_name)"),
	})

	if err != nil {
		// Check if it's a conditional check failure (lock already held)
		var condErr *types.ConditionalCheckFailedException
		if ok := (err != nil); ok {
			_ = condErr
			// Lock is held, get who holds it
			holder, getErr := getLockHolder(ctx, cfg, lockName)
			if getErr != nil {
				return false, nil, fmt.Errorf("lock held but failed to get holder: %w", getErr)
			}
			return false, holder, nil
		}
		return false, nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	return true, &lock, nil
}

// getLockHolder gets the current holder of a lock
func getLockHolder(ctx context.Context, cfg *Config, lockName string) (*Lock, error) {
	result, err := cfg.DynamoDBClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(cfg.LocksTable),
		Key: map[string]types.AttributeValue{
			"lock_name": &types.AttributeValueMemberS{Value: lockName},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get lock: %w", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("lock not found")
	}

	var lock Lock
	if err := attributevalue.UnmarshalMap(result.Item, &lock); err != nil {
		return nil, fmt.Errorf("failed to unmarshal lock: %w", err)
	}

	return &lock, nil
}

// releaseLock releases a lock
func releaseLock(ctx context.Context, cfg *Config, lockName string) error {
	if lockName == "" {
		return nil // No lock to release
	}

	_, err := cfg.DynamoDBClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(cfg.LocksTable),
		Key: map[string]types.AttributeValue{
			"lock_name": &types.AttributeValueMemberS{Value: lockName},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	return nil
}

// recordExecution records an execution in DynamoDB
func recordExecution(ctx context.Context, cfg *Config, exec *Execution) error {
	item, err := attributevalue.MarshalMap(exec)
	if err != nil {
		return fmt.Errorf("failed to marshal execution: %w", err)
	}

	_, err = cfg.DynamoDBClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(cfg.ExecutionsTable),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to record execution: %w", err)
	}

	return nil
}

// getExecution retrieves an execution from DynamoDB
func getExecution(ctx context.Context, cfg *Config, executionID string) (*Execution, error) {
	// Query with just the partition key
	result, err := cfg.DynamoDBClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(cfg.ExecutionsTable),
		KeyConditionExpression: aws.String("execution_id = :id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":id": &types.AttributeValueMemberS{Value: executionID},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("execution not found")
	}

	var exec Execution
	if err := attributevalue.UnmarshalMap(result.Items[0], &exec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal execution: %w", err)
	}

	return &exec, nil
}

// listExecutions lists executions for a user
func listExecutions(ctx context.Context, cfg *Config, userEmail string, limit int) ([]Execution, error) {
	if limit == 0 {
		limit = 20 // Default limit
	}

	result, err := cfg.DynamoDBClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(cfg.ExecutionsTable),
		IndexName:              aws.String("user_email-started_at"),
		KeyConditionExpression: aws.String("user_email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: userEmail},
		},
		Limit:            aws.Int32(int32(limit)),
		ScanIndexForward: aws.Bool(false), // Most recent first
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}

	var executions []Execution
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &executions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal executions: %w", err)
	}

	return executions, nil
}

// listLocks lists all active locks
func listLocks(ctx context.Context, cfg *Config) ([]Lock, error) {
	result, err := cfg.DynamoDBClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(cfg.LocksTable),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list locks: %w", err)
	}

	var locks []Lock
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &locks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal locks: %w", err)
	}

	return locks, nil
}
