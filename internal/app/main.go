package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"slices"
	"time"

	"runvoy/internal/api"
	appaws "runvoy/internal/app/aws"
	"runvoy/internal/auth"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

// Runner abstracts provider-specific command execution (e.g., AWS ECS, GCP, etc.).
type Runner interface {
	// StartTask triggers an execution on the underlying platform and returns
	// a provider-specific task ARN, a stable executionID, and the task creation timestamp.
	// The createdAt timestamp comes from the provider (e.g., ECS CreatedAt) when available.
	StartTask(
		ctx context.Context,
		userEmail string,
		req api.ExecutionRequest) (executionID string, taskARN string, createdAt *time.Time, err error)
	// KillTask terminates a running task identified by executionID.
	// Returns an error if the task is already terminated or cannot be terminated.
	KillTask(ctx context.Context, executionID string) error
}

// Service provides the core business logic for command execution and user management.
type Service struct {
	userRepo      database.UserRepository
	executionRepo database.ExecutionRepository
	runner        Runner
	Logger        *slog.Logger
	Provider      constants.BackendProvider
}

// NOTE: provider-specific configuration has been moved to subpackages (e.g., app/aws).

// NewService creates a new service instance.
// If userRepo is nil, user-related operations will not be available.
// This allows the service to work without database dependencies for simple operations.
func NewService(
	userRepo database.UserRepository,
	executionRepo database.ExecutionRepository,
	runner Runner,
	logger *slog.Logger,
	provider constants.BackendProvider) *Service {
	return &Service{
		userRepo:      userRepo,
		executionRepo: executionRepo,
		runner:        runner,
		Logger:        logger,
		Provider:      provider,
	}
}

// CreateUser creates a new user with an API key.
// If no API key is provided in the request, one will be generated.
// The API key is only returned in the response and should be stored by the client.
func (s *Service) CreateUser(ctx context.Context, req api.CreateUserRequest) (*api.CreateUserResponse, error) {
	if s.userRepo == nil {
		return nil, apperrors.ErrInternalError("user repository not configured", nil)
	}

	if req.Email == "" {
		return nil, apperrors.ErrBadRequest("email is required", nil)
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, apperrors.ErrBadRequest("invalid email address", err)
	}

	existingUser, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}

	if existingUser != nil {
		return nil, apperrors.ErrConflict("user with this email already exists", nil)
	}

	apiKey := req.APIKey
	if apiKey == "" {
		apiKey, err = auth.GenerateAPIKey()
		if err != nil {
			return nil, apperrors.ErrInternalError("failed to generate API key", err)
		}
	}

	apiKeyHash := auth.HashAPIKey(apiKey)

	user := &api.User{
		Email:     req.Email,
		CreatedAt: time.Now().UTC(),
		Revoked:   false,
	}

	if err := s.userRepo.CreateUser(ctx, user, apiKeyHash); err != nil {
		return nil, apperrors.ErrDatabaseError("failed to create user", err)
	}

	return &api.CreateUserResponse{
		User:   user,
		APIKey: apiKey, // Return plain API key (only time it's available!)
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

	if err := s.userRepo.RevokeUser(ctx, email); err != nil {
		// Propagate errors as-is (they already have proper status codes)
		return err
	}

	return nil
}

// RunCommand starts a provider-specific task and records the execution.
func (s *Service) RunCommand(
	ctx context.Context,
	userEmail string,
	req api.ExecutionRequest) (*api.ExecutionResponse, error) {
	if s.executionRepo == nil {
		return nil, apperrors.ErrInternalError("execution repository not configured", nil)
	}

	if req.Command == "" {
		return nil, apperrors.ErrBadRequest("command is required", nil)
	}
	executionID, taskARN, createdAt, err := s.runner.StartTask(ctx, userEmail, req)
	if err != nil {
		return nil, err
	}

	reqLogger := logger.DeriveRequestLogger(ctx, s.Logger)
	startedAt := time.Now().UTC()
	if createdAt != nil {
		startedAt = createdAt.UTC()
	}

	if taskARN != "" {
		reqLogger.Info("task started", "task", map[string]string{
			"executionID": executionID,
			"taskARN":     taskARN,
			"startedAt":   startedAt.Format(time.RFC3339),
		})
	}

	requestID := ""
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		requestID = lc.AwsRequestID
	}
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
			"executionID", executionID,
		)
	}

	if err := s.executionRepo.CreateExecution(ctx, execution); err != nil {
		reqLogger.Error("failed to create execution record, but task started",
			"error", err,
			"executionID", executionID,
		)
		// Continue even if recording fails - the task is already running
	}

	return &api.ExecutionResponse{
		ExecutionID: executionID,
		Status:      string(constants.ExecutionRunning),
	}, nil
}

// GetLogsByExecutionID returns aggregated Cloud logs for a given execution
func (s *Service) GetLogsByExecutionID(ctx context.Context, executionID string) (*api.LogsResponse, error) {
	if executionID == "" {
		return nil, apperrors.ErrBadRequest("executionID is required", nil)
	}

	switch s.Provider {
	case constants.AWS:
		events, err := s.getAWSLogsByExecutionID(ctx, executionID)
		if err != nil {
			return nil, err
		}
		reqLogger := logger.DeriveRequestLogger(ctx, s.Logger)
		reqLogger.Debug("fetched log events", "executionID", executionID, "events", events)
		return &api.LogsResponse{ExecutionID: executionID, Events: events}, nil
	default:
		return nil, apperrors.ErrInternalError("logs not supported for this provider", nil)
	}
}

// getAWSLogsByExecutionID delegates to the AWS implementation using the runner config
func (s *Service) getAWSLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error) {
	execImpl, ok := s.runner.(*appaws.Runner)
	if !ok {
		return nil, apperrors.ErrInternalError("aws runner not configured", nil)
	}
	return execImpl.FetchLogsByExecutionID(ctx, executionID)
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

	reqLogger.Debug("execution found", "executionID", executionID, "status", execution.Status)

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
	if err := s.runner.KillTask(ctx, executionID); err != nil {
		return err
	}

	reqLogger.Info("execution termination initiated", "executionID", executionID)

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
