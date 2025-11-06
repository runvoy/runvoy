// Package database defines repository interfaces for data persistence.
// It provides abstractions for user and execution storage.
package database

import (
	"context"
	"time"

	"runvoy/internal/api"
)

// UserRepository defines the interface for user-related database operations.
// This abstraction allows for different implementations (DynamoDB, PostgreSQL, etc.)
// without changing the business logic layer.
type UserRepository interface {
	// CreateUser stores a new user with their hashed API key in the database.
	// Returns an error if the user already exists or if the operation fails.
	CreateUser(ctx context.Context, user *api.User, apiKeyHash string) error

	// CreateUserWithExpiration stores a new user with their hashed API key and optional TTL.
	// If expiresAtUnix is 0, no TTL is set. If expiresAtUnix is > 0, it sets expires_at for automatic deletion.
	CreateUserWithExpiration(ctx context.Context, user *api.User, apiKeyHash string, expiresAtUnix int64) error

	// RemoveExpiration removes the expires_at field from a user record, making them permanent.
	RemoveExpiration(ctx context.Context, email string) error

	// GetUserByEmail retrieves a user by their email address.
	// Returns nil if the user doesn't exist.
	GetUserByEmail(ctx context.Context, email string) (*api.User, error)

	// GetUserByAPIKeyHash retrieves a user by their hashed API key.
	// Used for authentication. Returns nil if no user has this API key.
	GetUserByAPIKeyHash(ctx context.Context, apiKeyHash string) (*api.User, error)

	// UpdateLastUsed updates the last_used timestamp for a user.
	// Called after successful API key authentication.
	UpdateLastUsed(ctx context.Context, email string) (*time.Time, error)

	// RevokeUser marks a user's API key as revoked without deleting the record.
	// Useful for audit trails.
	RevokeUser(ctx context.Context, email string) error

	// Pending API key operations

	// CreatePendingAPIKey stores a pending API key with a secret token.
	CreatePendingAPIKey(ctx context.Context, pending *api.PendingAPIKey) error

	// GetPendingAPIKey retrieves a pending API key by its secret token.
	// Returns nil if the token doesn't exist or has expired.
	GetPendingAPIKey(ctx context.Context, secretToken string) (*api.PendingAPIKey, error)

	// MarkAsViewed atomically marks a pending key as viewed with the IP address.
	MarkAsViewed(ctx context.Context, secretToken string, ipAddress string) error

	// DeletePendingAPIKey removes a pending API key from the database.
	DeletePendingAPIKey(ctx context.Context, secretToken string) error

	// ListUsers returns all users in the system (excluding API key hashes for security).
	// Used by admins to view all users and their basic information.
	ListUsers(ctx context.Context) ([]*api.User, error)
}

// ExecutionRepository defines the interface for execution-related database operations.
type ExecutionRepository interface {
	// CreateExecution stores a new execution record in the database.
	CreateExecution(ctx context.Context, execution *api.Execution) error

	// GetExecution retrieves an execution by its execution ID.
	GetExecution(ctx context.Context, executionID string) (*api.Execution, error)

	// UpdateExecution updates an existing execution record.
	UpdateExecution(ctx context.Context, execution *api.Execution) error

	// ListExecutions returns all executions currently present in the database.
	// Implementations may choose an efficient retrieval strategy; order is newest first.
	ListExecutions(ctx context.Context) ([]*api.Execution, error)
}

// ConnectionRepository defines the interface for WebSocket connection-related database operations.
type ConnectionRepository interface {
	// CreateConnection stores a new WebSocket connection record in the database.
	CreateConnection(ctx context.Context, connection *api.WebSocketConnection) error

	// DeleteConnections removes WebSocket connections from the database.
	DeleteConnections(ctx context.Context, connectionIDs []string) (int, error)

	// GetConnectionsByExecutionID retrieves all active WebSocket connections for a given execution ID.
	// Returns a list of connection IDs that are subscribed to the specified execution.
	GetConnectionsByExecutionID(ctx context.Context, executionID string) ([]string, error)

	// GetConnectionsWithMetadataByExecutionID retrieves all active WebSocket connections with full metadata
	// for a given execution ID. Returns full connection objects including last_index.
	GetConnectionsWithMetadataByExecutionID(ctx context.Context, executionID string) ([]*api.WebSocketConnection, error)
}

// LogRepository defines the interface for log storage operations.
// Logs are stored with sequential indexes for reliable ordering and gap-free streaming.
type LogRepository interface {
	// StoreLogs stores log events in DynamoDB with sequential indexes.
	// Returns the highest index stored.
	// Uses atomic counter to prevent race conditions when multiple forwarders process logs simultaneously.
	StoreLogs(ctx context.Context, executionID string, events []api.LogEvent) (int64, error)

	// GetLogsSinceIndex retrieves logs starting from a specific index (exclusive).
	// Returns logs sorted by log_index ascending.
	// lastIndex: the highest index the client has already seen (logs with index > lastIndex will be returned).
	GetLogsSinceIndex(ctx context.Context, executionID string, lastIndex int64) ([]api.LogEvent, error)

	// GetMaxIndex returns the highest index for an execution (or 0 if none exist).
	GetMaxIndex(ctx context.Context, executionID string) (int64, error)

	// SetExpiration sets TTL for all logs of an execution.
	// Updates expires_at attribute for all log items.
	// expiresAt: Unix timestamp in seconds for TTL.
	SetExpiration(ctx context.Context, executionID string, expiresAt int64) error
}
