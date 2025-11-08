package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"slices"
	"strings"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/auth"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"
)

// Runner abstracts provider-specific command execution (e.g., AWS ECS, GCP, etc.).
type Runner interface {
	// StartTask triggers an execution on the underlying platform and returns
	// a stable executionID and the task creation timestamp.
	// The createdAt timestamp comes from the provider (e.g., ECS CreatedAt) when available.
	StartTask(
		ctx context.Context,
		userEmail string,
		req *api.ExecutionRequest) (executionID string, createdAt *time.Time, err error)
	// KillTask terminates a running task identified by executionID.
	// Returns an error if the task is already terminated or cannot be terminated.
	KillTask(ctx context.Context, executionID string) error
	// RegisterImage registers a Docker image as a task definition in the execution platform.
	// isDefault: if true, explicitly set as default.
	// If nil or false, becomes default only if no default exists (first image behavior).
	RegisterImage(ctx context.Context, image string, isDefault *bool) error
	// ListImages lists all registered Docker images.
	ListImages(ctx context.Context) ([]api.ImageInfo, error)
	// RemoveImage removes a Docker image and deregisters its task definitions.
	RemoveImage(ctx context.Context, image string) error
	// FetchLogsByExecutionID retrieves logs for a specific execution.
	// Returns empty slice if logs are not available or not supported by the provider.
	FetchLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error)
}

// Service provides the core business logic for command execution and user management.
type Service struct {
	userRepo            database.UserRepository
	executionRepo       database.ExecutionRepository
	connRepo            database.ConnectionRepository
	runner              Runner
	Logger              *slog.Logger
	Provider            constants.BackendProvider
	websocketAPIBaseURL string // Base WebSocket API URL (without wss:// prefix or path)
}

// NOTE: provider-specific configuration has been moved to subpackages (e.g., app/aws).

// NewService creates a new service instance.
// If userRepo is nil, user-related operations will not be available.
// This allows the service to work without database dependencies for simple operations.
func NewService(
	userRepo database.UserRepository,
	executionRepo database.ExecutionRepository,
	connRepo database.ConnectionRepository,
	runner Runner,
	log *slog.Logger,
	provider constants.BackendProvider,
	websocketAPIBaseURL string) *Service {
	return &Service{
		userRepo:            userRepo,
		executionRepo:       executionRepo,
		connRepo:            connRepo,
		runner:              runner,
		Logger:              log,
		Provider:            provider,
		websocketAPIBaseURL: websocketAPIBaseURL,
	}
}

// validateCreateUserRequest validates the email in the create user request.
func (s *Service) validateCreateUserRequest(ctx context.Context, email string) error {
	if email == "" {
		return apperrors.ErrBadRequest("email is required", nil)
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return apperrors.ErrBadRequest("invalid email address", err)
	}

	existingUser, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}

	if existingUser != nil {
		return apperrors.ErrConflict("user with this email already exists", nil)
	}

	return nil
}

// generateOrUseAPIKey generates a new API key if none is provided.
func generateOrUseAPIKey(providedKey string) (string, error) {
	if providedKey != "" {
		return providedKey, nil
	}
	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		return "", apperrors.ErrInternalError("failed to generate API key", err)
	}
	return apiKey, nil
}

// createPendingClaim creates a pending API key claim record.
func (s *Service) createPendingClaim(
	ctx context.Context, apiKey, email, createdByEmail string, expiresAt int64,
) (string, error) {
	secretToken, err := auth.GenerateSecretToken()
	if err != nil {
		return "", apperrors.ErrInternalError("failed to generate secret token", err)
	}

	pending := &api.PendingAPIKey{
		SecretToken: secretToken,
		APIKey:      apiKey,
		UserEmail:   email,
		CreatedBy:   createdByEmail,
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   expiresAt,
		Viewed:      false,
	}

	if err = s.userRepo.CreatePendingAPIKey(ctx, pending); err != nil {
		return "", apperrors.ErrDatabaseError("failed to create pending API key", err)
	}

	return secretToken, nil
}

// CreateUser creates a new user with an API key and returns a claim token.
// If no API key is provided in the request, one will be generated.
func (s *Service) CreateUser(
	ctx context.Context, req api.CreateUserRequest, createdByEmail string,
) (*api.CreateUserResponse, error) {
	if s.userRepo == nil {
		return nil, apperrors.ErrInternalError("user repository not configured", nil)
	}

	if err := s.validateCreateUserRequest(ctx, req.Email); err != nil {
		return nil, err
	}

	apiKey, err := generateOrUseAPIKey(req.APIKey)
	if err != nil {
		return nil, err
	}

	apiKeyHash := auth.HashAPIKey(apiKey)

	user := &api.User{
		Email:     req.Email,
		CreatedAt: time.Now().UTC(),
		Revoked:   false,
	}

	expiresAt := time.Now().Add(constants.ClaimURLExpirationMinutes * time.Minute).Unix()

	if err = s.userRepo.CreateUserWithExpiration(ctx, user, apiKeyHash, expiresAt); err != nil {
		return nil, apperrors.ErrDatabaseError("failed to create user", err)
	}

	secretToken, err := s.createPendingClaim(ctx, apiKey, req.Email, createdByEmail, expiresAt)
	if err != nil {
		_ = s.userRepo.RevokeUser(ctx, req.Email)
		return nil, err
	}

	return &api.CreateUserResponse{
		User:       user,
		ClaimToken: secretToken,
	}, nil
}

// ClaimAPIKey retrieves and claims a pending API key by its secret token.
func (s *Service) ClaimAPIKey(
	ctx context.Context,
	secretToken string,
	ipAddress string,
) (*api.ClaimAPIKeyResponse, error) {
	if s.userRepo == nil {
		return nil, apperrors.ErrInternalError("user repository not configured", nil)
	}

	// Retrieve pending key
	pending, err := s.userRepo.GetPendingAPIKey(ctx, secretToken)
	if err != nil {
		return nil, apperrors.ErrDatabaseError("failed to retrieve pending key", err)
	}

	if pending == nil {
		return nil, apperrors.ErrNotFound("invalid or expired token", nil)
	}

	// Check if already viewed
	if pending.Viewed {
		return nil, apperrors.ErrConflict("key has already been claimed", nil)
	}

	// Check if expired
	now := time.Now().Unix()
	if pending.ExpiresAt < now {
		return nil, apperrors.ErrNotFound("token has expired", nil)
	}

	// Mark as viewed atomically
	if markErr := s.userRepo.MarkAsViewed(ctx, secretToken, ipAddress); markErr != nil {
		return nil, markErr
	}

	// Remove expiration from user record (make user permanent)
	if removeErr := s.userRepo.RemoveExpiration(ctx, pending.UserEmail); removeErr != nil {
		// Log error but don't fail the claim - user already exists and can authenticate
		s.Logger.Error("failed to remove expiration from user record", "error", removeErr, "email", pending.UserEmail)
	}

	return &api.ClaimAPIKeyResponse{
		APIKey:    pending.APIKey,
		UserEmail: pending.UserEmail,
		Message:   "API key claimed successfully",
	}, nil
}

// AuthenticateUser verifies an API key and returns the associated user.
// Returns appropriate errors for invalid API keys, revoked keys, or server errors.
func (s *Service) AuthenticateUser(ctx context.Context, apiKey string) (*api.User, error) {
	if s.userRepo == nil {
		return nil, apperrors.ErrInternalError("user repository not configured", nil)
	}

	if apiKey == "" {
		return nil, apperrors.ErrBadRequest("API key is required", nil)
	}

	apiKeyHash := auth.HashAPIKey(apiKey)

	user, err := s.userRepo.GetUserByAPIKeyHash(ctx, apiKeyHash)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, apperrors.ErrInvalidAPIKey(nil)
	}

	if user.Revoked {
		return nil, apperrors.ErrAPIKeyRevoked(nil)
	}

	return user, nil
}

// UpdateUserLastUsed updates the user's last_used timestamp after successful authentication.
// This is a best-effort operation; callers may choose to log failures without failing the request.
func (s *Service) UpdateUserLastUsed(ctx context.Context, email string) (*time.Time, error) {
	if s.userRepo == nil {
		return nil, apperrors.ErrInternalError("user repository not configured", nil)
	}
	if email == "" {
		return nil, apperrors.ErrBadRequest("email is required", nil)
	}
	return s.userRepo.UpdateLastUsed(ctx, email)
}

// RevokeUser marks a user's API key as revoked.
// Returns an error if the user does not exist or revocation fails.
func (s *Service) RevokeUser(ctx context.Context, email string) error {
	if s.userRepo == nil {
		return apperrors.ErrInternalError("user repository not configured", nil)
	}

	if email == "" {
		return apperrors.ErrBadRequest("email is required", nil)
	}

	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		// Propagate database errors as-is
		return err
	}
	if user == nil {
		return apperrors.ErrNotFound("user not found", nil)
	}

	if revokeErr := s.userRepo.RevokeUser(ctx, email); revokeErr != nil {
		// Propagate errors as-is (they already have proper status codes)
		return revokeErr
	}

	return nil
}

// ListUsers returns all users in the system (excluding API key hashes for security).
// Returns an error if the user repository is not configured or if the query fails.
// Sort by email ascending.
func (s *Service) ListUsers(ctx context.Context) (*api.ListUsersResponse, error) {
	if s.userRepo == nil {
		return nil, apperrors.ErrInternalError("user repository not configured", nil)
	}

	users, err := s.userRepo.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(users, func(a, b *api.User) int {
		return strings.Compare(a.Email, b.Email)
	})

	return &api.ListUsersResponse{
		Users: users,
	}, nil
}

// RunCommand starts a provider-specific task and records the execution.
func (s *Service) RunCommand(
	ctx context.Context,
	userEmail string,
	req *api.ExecutionRequest) (*api.ExecutionResponse, error) {
	if s.executionRepo == nil {
		return nil, apperrors.ErrInternalError("execution repository not configured", nil)
	}

	if req.Command == "" {
		return nil, apperrors.ErrBadRequest("command is required", nil)
	}
	executionID, createdAt, err := s.runner.StartTask(ctx, userEmail, req)
	if err != nil {
		return nil, err
	}

	reqLogger := logger.DeriveRequestLogger(ctx, s.Logger)
	startedAt := time.Now().UTC()
	if createdAt != nil {
		startedAt = createdAt.UTC()
	}

	requestID := logger.GetRequestID(ctx)
	execution := &api.Execution{
		ExecutionID:     executionID,
		UserEmail:       userEmail,
		Command:         req.Command,
		LockName:        req.Lock,
		StartedAt:       startedAt,
		Status:          string(constants.ExecutionRunning),
		RequestID:       requestID,
		ComputePlatform: string(s.Provider),
	}

	if requestID == "" {
		reqLogger.Warn("request ID not available; storing execution without request ID",
			"execution_id", executionID,
		)
	}

	if err = s.executionRepo.CreateExecution(ctx, execution); err != nil {
		reqLogger.Error("failed to create execution record, but task started",
			"error", err,
			"execution_id", executionID,
		)
		// Continue even if recording fails - the task is already running
	}

	return &api.ExecutionResponse{
		ExecutionID: executionID,
		Status:      string(constants.ExecutionRunning),
	}, nil
}

// GetLogsByExecutionID returns aggregated Cloud logs for a given execution
// WebSocket endpoint is stored without protocol (normalized in config)
// Always use wss:// for production WebSocket connections
// WebSocket URL is only provided if the execution is still RUNNING
// userEmail: authenticated user email for audit trail
// clientIPAtLogsTime: client IP from the logs request for tracing
func (s *Service) GetLogsByExecutionID(
	ctx context.Context,
	executionID string,
	lastSeenTimestamp int64,
	userEmail *string,
	clientIPAtLogsTime *string,
) (*api.LogsResponse, error) {
	if s.executionRepo == nil {
		return nil, apperrors.ErrInternalError("execution repository not configured", nil)
	}
	if executionID == "" {
		return nil, apperrors.ErrBadRequest("executionID is required", nil)
	}

	// Fetch execution status
	execution, err := s.executionRepo.GetExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if execution == nil {
		return nil, apperrors.ErrNotFound("execution not found", nil)
	}

	// For RUNNING executions: return only websocket URL, no events
	if execution.Status == string(constants.ExecutionRunning) {
		var websocketURL string
		if s.websocketAPIBaseURL != "" {
			url, wsErr := s.createWebSocketPendingConnection(
				ctx, executionID, lastSeenTimestamp, userEmail, clientIPAtLogsTime,
			)
			if wsErr != nil {
				return nil, wsErr
			}
			websocketURL = url
		}

		return &api.LogsResponse{
			ExecutionID:  executionID,
			Status:       execution.Status,
			Events:       nil, // No events for running executions, stream via websocket
			WebSocketURL: websocketURL,
		}, nil
	}

	// For COMPLETED executions: return all logs filtered by lastSeenTimestamp
	events, err := s.runner.FetchLogsByExecutionID(ctx, executionID)
	if err != nil {
		return nil, err
	}

	// Filter events to only include those after lastSeenTimestamp
	var filteredEvents []api.LogEvent
	for _, event := range events {
		if event.Timestamp > lastSeenTimestamp {
			filteredEvents = append(filteredEvents, event)
		}
	}

	return &api.LogsResponse{
		ExecutionID:  executionID,
		Status:       execution.Status,
		Events:       filteredEvents,
		WebSocketURL: "", // No websocket for completed executions
	}, nil
}

// createWebSocketPendingConnection creates a pending WebSocket connection for streaming logs
// Returns the WebSocket URL with authentication token or an error
// lastSeenTimestamp is the cursor position for resumable streaming
func (s *Service) createWebSocketPendingConnection(
	ctx context.Context,
	executionID string,
	lastSeenTimestamp int64,
	userEmail *string,
	clientIPAtLogsTime *string,
) (string, error) {
	var email string
	if userEmail != nil {
		email = *userEmail
	}

	var clientIP string
	if clientIPAtLogsTime != nil {
		clientIP = *clientIPAtLogsTime
	}
	// Generate a secure token for WebSocket authentication
	token, tokenErr := auth.GenerateSecretToken()
	if tokenErr != nil {
		s.Logger.Error("failed to generate websocket token",
			"error", tokenErr,
			"execution_id", executionID)
		return "", apperrors.ErrInternalError("failed to generate websocket token", tokenErr)
	}

	// Create a pending connection entry with the token
	expiresAt := time.Now().Add(constants.ConnectionTTLHours * time.Hour).Unix()
	pendingConnection := &api.WebSocketConnection{
		ConnectionID:         fmt.Sprintf("pending_%s", executionID),
		ExecutionID:          executionID,
		Functionality:        constants.FunctionalityLogStreaming,
		ExpiresAt:            expiresAt,
		Token:                token,
		UserEmail:            email,
		ClientIPAtLogsTime:   clientIP,
		LastSeenLogTimestamp: lastSeenTimestamp, // Initialize cursor for resumable streaming
	}

	if connErr := s.connRepo.CreateConnection(ctx, pendingConnection); connErr != nil {
		s.Logger.Error("failed to create pending websocket connection",
			"error", connErr,
			"execution_id", executionID,
			"user_email", email)
		return "", apperrors.ErrInternalError("failed to create websocket connection", connErr)
	}

	wsURL := "wss://" + s.websocketAPIBaseURL
	return fmt.Sprintf("%s?execution_id=%s&token=%s", wsURL, executionID, token), nil
}

// GetExecutionStatus returns the current status and metadata for a given execution ID
func (s *Service) GetExecutionStatus(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error) {
	if s.executionRepo == nil {
		return nil, apperrors.ErrInternalError("execution repository not configured", nil)
	}
	if executionID == "" {
		return nil, apperrors.ErrBadRequest("executionID is required", nil)
	}

	execution, err := s.executionRepo.GetExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if execution == nil {
		return nil, apperrors.ErrNotFound("execution not found", nil)
	}

	var exitCodePtr *int
	if execution.CompletedAt != nil {
		// Only populate ExitCode if we have actually recorded completion
		ec := execution.ExitCode
		exitCodePtr = &ec
	}

	return &api.ExecutionStatusResponse{
		ExecutionID: execution.ExecutionID,
		Status:      execution.Status,
		ExitCode:    exitCodePtr,
		StartedAt:   execution.StartedAt,
		CompletedAt: execution.CompletedAt,
	}, nil
}

// KillExecution terminates a running execution identified by executionID.
// It verifies the execution exists in the database and checks task status before termination.
// Returns an error if the execution is not found, already terminated, or termination fails.
func (s *Service) KillExecution(ctx context.Context, executionID string) error {
	if s.executionRepo == nil {
		return apperrors.ErrInternalError("execution repository not configured", nil)
	}
	if executionID == "" {
		return apperrors.ErrBadRequest("executionID is required", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, s.Logger)

	// First, verify the execution exists in the database
	execution, err := s.executionRepo.GetExecution(ctx, executionID)
	if err != nil {
		return err
	}
	if execution == nil {
		return apperrors.ErrNotFound("execution not found", nil)
	}

	reqLogger.Debug("execution found", "execution_id", executionID, "status", execution.Status)

	// Check if execution is already in a terminal state
	terminalStatuses := constants.TerminalExecutionStatuses()
	if slices.ContainsFunc(terminalStatuses, func(status constants.ExecutionStatus) bool {
		return execution.Status == string(status)
	}) {
		return apperrors.ErrBadRequest(
			"execution is already terminated",
			fmt.Errorf("execution status: %s", execution.Status))
	}

	// Delegate to the runner to kill the task
	if killErr := s.runner.KillTask(ctx, executionID); killErr != nil {
		return killErr
	}

	return nil
}

// ListExecutions returns all executions currently present in the database.
// Fields with no values are omitted in JSON due to omitempty tags on api.Execution.
func (s *Service) ListExecutions(ctx context.Context) ([]*api.Execution, error) {
	if s.executionRepo == nil {
		return nil, apperrors.ErrInternalError("execution repository not configured", nil)
	}
	executions, err := s.executionRepo.ListExecutions(ctx)
	if err != nil {
		return nil, err
	}
	return executions, nil
}

// RegisterImage registers a Docker image and creates the corresponding task definition.
func (s *Service) RegisterImage(
	ctx context.Context,
	image string,
	isDefault *bool,
) (*api.RegisterImageResponse, error) {
	if image == "" {
		return nil, apperrors.ErrBadRequest("image is required", nil)
	}

	if err := s.runner.RegisterImage(ctx, image, isDefault); err != nil {
		return nil, apperrors.ErrInternalError("failed to register image", err)
	}

	return &api.RegisterImageResponse{
		Image:   image,
		Message: "Image registered successfully",
	}, nil
}

// ListImages returns all registered Docker images.
func (s *Service) ListImages(ctx context.Context) (*api.ListImagesResponse, error) {
	images, err := s.runner.ListImages(ctx)
	if err != nil {
		return nil, apperrors.ErrInternalError("failed to list images", err)
	}

	return &api.ListImagesResponse{
		Images: images,
	}, nil
}

// RemoveImage removes a Docker image and deregisters its task definitions.
func (s *Service) RemoveImage(ctx context.Context, image string) error {
	if image == "" {
		return apperrors.ErrBadRequest("image is required", nil)
	}

	if err := s.runner.RemoveImage(ctx, image); err != nil {
		return apperrors.ErrInternalError("failed to remove image", err)
	}

	return nil
}
