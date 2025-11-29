package database

import (
	"context"
	"time"

	"github.com/runvoy/runvoy/internal/api"
)

// UserRepository defines the interface for user-related database operations.
// This abstraction allows for different implementations (DynamoDB, PostgreSQL, etc.)
// without changing the business logic layer.
type UserRepository interface {
	// CreateUser stores a new user with their hashed API key in the database.
	// If expiresAtUnix is 0, no TTL is set (permanent user).
	// If expiresAtUnix is > 0, it sets expires_at for automatic deletion.
	// Returns an error if the user already exists or if the operation fails.
	CreateUser(ctx context.Context, user *api.User, apiKeyHash string, expiresAtUnix int64) error

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

	// GetUsersByRequestID retrieves all users created or modified by a specific request ID.
	GetUsersByRequestID(ctx context.Context, requestID string) ([]*api.User, error)
}

// ExecutionRepository defines the interface for execution-related database operations.
type ExecutionRepository interface {
	// CreateExecution stores a new execution record in the database.
	CreateExecution(ctx context.Context, execution *api.Execution) error

	// GetExecution retrieves an execution by its execution ID.
	GetExecution(ctx context.Context, executionID string) (*api.Execution, error)

	// UpdateExecution updates an existing execution record.
	UpdateExecution(ctx context.Context, execution *api.Execution) error

	// ListExecutions returns executions from the database with optional filtering and pagination.
	// Parameters:
	//   - limit: maximum number of executions to return. Use 0 to fetch all executions.
	//   - statuses: optional slice of execution statuses to filter by.
	//              If empty, all executions are returned.
	// Results are ordered newest first.
	ListExecutions(ctx context.Context, limit int, statuses []string) ([]*api.Execution, error)

	// GetExecutionsByRequestID retrieves all executions created or modified by a specific request ID.
	GetExecutionsByRequestID(ctx context.Context, requestID string) ([]*api.Execution, error)
}

// ConnectionRepository defines the interface for WebSocket connection-related database operations.
type ConnectionRepository interface {
	// CreateConnection stores a new WebSocket connection record in the database.
	CreateConnection(ctx context.Context, connection *api.WebSocketConnection) error

	// DeleteConnections removes WebSocket connections from the database.
	DeleteConnections(ctx context.Context, connectionIDs []string) (int, error)

	// GetConnectionsByExecutionID retrieves all active WebSocket connection records for a given execution ID.
	// Returns the complete connection objects including token and other metadata.
	GetConnectionsByExecutionID(ctx context.Context, executionID string) ([]*api.WebSocketConnection, error)

	// UpdateLastEventID stores the last delivered log event identifier for a connection.
	UpdateLastEventID(ctx context.Context, connectionID, lastEventID string) error
}

// LogEventRepository defines the interface for storing and deleting execution log events.
type LogEventRepository interface {
	// SaveLogEvents stores new log events for an execution.
	SaveLogEvents(ctx context.Context, executionID string, logEvents []api.LogEvent) error

	// ListLogEvents retrieves all buffered log events for an execution ordered by timestamp and event ID.
	ListLogEvents(ctx context.Context, executionID string) ([]api.LogEvent, error)

	// DeleteLogEvents schedules removal of all log events for an execution. This is typically invoked when
	// an execution finishes to prune buffered logs via DynamoDB TTL.
	DeleteLogEvents(ctx context.Context, executionID string) error
}

// TokenRepository defines the interface for WebSocket token validation operations.
type TokenRepository interface {
	// CreateToken stores a new WebSocket authentication token with metadata.
	CreateToken(ctx context.Context, token *api.WebSocketToken) error

	// GetToken retrieves a token by its value and validates it hasn't expired.
	// Returns nil if the token doesn't exist or has expired (DynamoDB TTL handles expiration).
	GetToken(ctx context.Context, tokenValue string) (*api.WebSocketToken, error)

	// DeleteToken removes a token from the database (used after validation or explicit cleanup).
	DeleteToken(ctx context.Context, tokenValue string) error
}

// ImageRepository defines the interface for image metadata storage operations.
type ImageRepository interface {
	// GetImagesByRequestID retrieves all images created or modified by a specific request ID.
	GetImagesByRequestID(ctx context.Context, requestID string) ([]api.ImageInfo, error)
}

// Repositories groups all database repository interfaces together.
// This struct is used to pass repositories as a cohesive unit while maintaining
// explicit access to individual repositories in service methods.
type Repositories struct {
	User       UserRepository
	Execution  ExecutionRepository
	Connection ConnectionRepository
	LogEvent   LogEventRepository
	Token      TokenRepository
	Image      ImageRepository
	Secrets    SecretsRepository
}
