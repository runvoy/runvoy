// Package dynamodb implements DynamoDB-based storage for runvoy.
// It provides persistence for execution locks using AWS DynamoDB.
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// LockRepository implements the database.LockRepository interface using DynamoDB.
type LockRepository struct {
	client    *dynamodb.Client
	tableName string
	logger    *slog.Logger
}

// NewLockRepository creates a new DynamoDB-backed lock repository.
func NewLockRepository(client *dynamodb.Client, tableName string, log *slog.Logger) *LockRepository {
	return &LockRepository{
		client:    client,
		tableName: tableName,
		logger:    log,
	}
}

// lockItem represents the structure stored in DynamoDB.
// This keeps the database schema separate from the API types.
type lockItem struct {
	LockName    string    `dynamodbav:"lock_name"`        // Partition key
	LockID      string    `dynamodbav:"lock_id"`          // Unique identifier for lock instance
	ExecutionID string    `dynamodbav:"execution_id"`     // Who holds the lock
	UserEmail   string    `dynamodbav:"user_email"`       // Email of lock holder
	AcquiredAt  time.Time `dynamodbav:"acquired_at"`      // When acquired
	ExpiresAt   time.Time `dynamodbav:"expires_at"`       // When it expires (TTL attribute)
	Status      string    `dynamodbav:"status"`           // 'active', 'releasing', 'released'
}

// toLockItem converts an api.Lock to a lockItem.
func toLockItem(l *api.Lock) *lockItem {
	return &lockItem{
		LockName:    l.LockName,
		LockID:      l.LockID,
		ExecutionID: l.ExecutionID,
		UserEmail:   l.UserEmail,
		AcquiredAt:  l.AcquiredAt,
		ExpiresAt:   l.ExpiresAt,
		Status:      l.Status,
	}
}

// toAPILock converts a lockItem to an api.Lock.
func (li *lockItem) toAPILock() *api.Lock {
	return &api.Lock{
		LockName:    li.LockName,
		LockID:      li.LockID,
		ExecutionID: li.ExecutionID,
		UserEmail:   li.UserEmail,
		AcquiredAt:  li.AcquiredAt,
		ExpiresAt:   li.ExpiresAt,
		Status:      li.Status,
	}
}

// AcquireLock atomically acquires a lock for an execution.
// Returns the lock if acquired, or an error if the lock is already held.
// The lock will automatically expire after ttl seconds.
func (r *LockRepository) AcquireLock(ctx context.Context, lockName, executionID, userEmail string, ttl int64) (*api.Lock, error) {
	if lockName == "" || executionID == "" || userEmail == "" || ttl <= 0 {
		return nil, apperrors.ErrInvalidInput("lockName, executionID, userEmail, and ttl must be non-empty and ttl > 0")
	}

	lockID := uuid.New().String()
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(ttl) * time.Second)

	item := &lockItem{
		LockName:    lockName,
		LockID:      lockID,
		ExecutionID: executionID,
		UserEmail:   userEmail,
		AcquiredAt:  now,
		ExpiresAt:   expiresAt,
		Status:      "active",
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		r.logger.Error("failed to marshal lock item", "lock_name", lockName, "error", err)
		return nil, apperrors.ErrDatabaseError("failed to marshal lock", err)
	}

	// Use conditional write to ensure atomicity:
	// Only succeed if the lock_name doesn't exist (new lock) or the existing lock has expired
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
		// Condition: lock doesn't exist OR lock exists but is expired
		// We check expires_at < now for expiration
		ConditionExpression: aws.String(
			"attribute_not_exists(lock_name) OR #expAt < :now",
		),
		ExpressionAttributeNames: map[string]string{
			"#expAt": "expires_at",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now": &types.AttributeValueMemberN{
				Value: fmt.Sprintf("%d", now.Unix()),
			},
		},
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			// Lock is already held by another execution
			existingLock, getErr := r.GetLock(ctx, lockName)
			if getErr != nil {
				r.logger.Warn("failed to get existing lock after conflict", "lock_name", lockName, "error", getErr)
				// Return a generic conflict error
				return nil, apperrors.ErrLockConflict(fmt.Sprintf("lock '%s' is already held", lockName))
			}
			if existingLock != nil {
				return nil, apperrors.ErrLockConflict(fmt.Sprintf(
					"lock '%s' held by execution %s until %s",
					lockName, existingLock.ExecutionID, existingLock.ExpiresAt.Format(time.RFC3339),
				))
			}
			return nil, apperrors.ErrLockConflict(fmt.Sprintf("lock '%s' is already held", lockName))
		}
		r.logger.Error("failed to acquire lock", "lock_name", lockName, "error", err)
		return nil, apperrors.ErrDatabaseError("failed to acquire lock", err)
	}

	r.logger.Debug("lock acquired", "lock_name", lockName, "execution_id", executionID, "expires_at", expiresAt)
	return item.toAPILock(), nil
}

// ReleaseLock atomically releases a lock.
// Returns an error if the lock doesn't exist or is held by a different execution.
func (r *LockRepository) ReleaseLock(ctx context.Context, lockName, executionID string) error {
	if lockName == "" || executionID == "" {
		return apperrors.ErrInvalidInput("lockName and executionID must be non-empty")
	}

	// Use conditional update to ensure atomicity:
	// Only succeed if the lock exists AND is held by the specified execution
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"lock_name": &types.AttributeValueMemberS{Value: lockName},
		},
		UpdateExpression: aws.String("SET #status = :released, #expAt = :now"),
		ConditionExpression: aws.String(
			"attribute_exists(lock_name) AND #execID = :execID",
		),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
			"#expAt":  "expires_at",
			"#execID": "execution_id",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":released": &types.AttributeValueMemberS{Value: "released"},
			":execID":   &types.AttributeValueMemberS{Value: executionID},
			":now":      &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().UTC().Unix())},
		},
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			// Lock doesn't exist or is held by a different execution
			return apperrors.ErrLockNotHeld(fmt.Sprintf("lock '%s' not held by execution %s", lockName, executionID))
		}
		r.logger.Error("failed to release lock", "lock_name", lockName, "execution_id", executionID, "error", err)
		return apperrors.ErrDatabaseError("failed to release lock", err)
	}

	r.logger.Debug("lock released", "lock_name", lockName, "execution_id", executionID)
	return nil
}

// GetLock retrieves the current lock holder for a lock name.
// Returns nil if the lock doesn't exist or has expired.
func (r *LockRepository) GetLock(ctx context.Context, lockName string) (*api.Lock, error) {
	if lockName == "" {
		return nil, apperrors.ErrInvalidInput("lockName must be non-empty")
	}

	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"lock_name": &types.AttributeValueMemberS{Value: lockName},
		},
	})

	if err != nil {
		r.logger.Error("failed to get lock", "lock_name", lockName, "error", err)
		return nil, apperrors.ErrDatabaseError("failed to get lock", err)
	}

	// No item found
	if result.Item == nil {
		return nil, nil
	}

	var item lockItem
	if err = attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		r.logger.Error("failed to unmarshal lock item", "lock_name", lockName, "error", err)
		return nil, apperrors.ErrDatabaseError("failed to unmarshal lock", err)
	}

	// Check if lock has expired
	if time.Now().UTC().After(item.ExpiresAt) {
		// Lock has expired, treat as not held
		return nil, nil
	}

	// Check if lock is in 'released' status
	if item.Status == "released" {
		return nil, nil
	}

	return item.toAPILock(), nil
}

// RenewLock extends the TTL of an existing lock.
// Returns an error if the lock is not held by the specified execution.
func (r *LockRepository) RenewLock(ctx context.Context, lockName, executionID string, newTTL int64) (*api.Lock, error) {
	if lockName == "" || executionID == "" || newTTL <= 0 {
		return nil, apperrors.ErrInvalidInput("lockName, executionID must be non-empty and newTTL > 0")
	}

	now := time.Now().UTC()
	newExpiresAt := now.Add(time.Duration(newTTL) * time.Second)

	// Use conditional update to ensure atomicity:
	// Only succeed if the lock exists, is held by the specified execution, and is active
	result, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"lock_name": &types.AttributeValueMemberS{Value: lockName},
		},
		UpdateExpression: aws.String("SET #expAt = :newExp, #acqAt = :now"),
		ConditionExpression: aws.String(
			"attribute_exists(lock_name) AND #execID = :execID AND #status = :active",
		),
		ExpressionAttributeNames: map[string]string{
			"#expAt":  "expires_at",
			"#acqAt":  "acquired_at",
			"#execID": "execution_id",
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":newExp": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", newExpiresAt.Unix())},
			":now":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", now.Unix())},
			":execID": &types.AttributeValueMemberS{Value: executionID},
			":active": &types.AttributeValueMemberS{Value: "active"},
		},
		ReturnValues: "ALL_NEW",
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return nil, apperrors.ErrLockNotHeld(fmt.Sprintf("lock '%s' not held by execution %s", lockName, executionID))
		}
		r.logger.Error("failed to renew lock", "lock_name", lockName, "execution_id", executionID, "error", err)
		return nil, apperrors.ErrDatabaseError("failed to renew lock", err)
	}

	// Parse the returned lock item
	var item lockItem
	if err = attributevalue.UnmarshalMap(result.Attributes, &item); err != nil {
		r.logger.Error("failed to unmarshal renewed lock item", "lock_name", lockName, "error", err)
		return nil, apperrors.ErrDatabaseError("failed to unmarshal lock", err)
	}

	r.logger.Debug("lock renewed", "lock_name", lockName, "execution_id", executionID, "new_expires_at", newExpiresAt)
	return item.toAPILock(), nil
}
