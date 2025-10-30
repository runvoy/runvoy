package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"net/mail"
	"time"

	"runvoy/internal/api"
	appaws "runvoy/internal/app/aws"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

// Runner abstracts provider-specific command execution (e.g., AWS ECS, GCP, etc.).
type Runner interface {
	// StartTask triggers an execution on the underlying platform and returns
	// a provider-specific task ARN and a stable executionID.
	StartTask(ctx context.Context, userEmail string, req api.ExecutionRequest) (executionID string, taskARN string, err error)
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
func NewService(userRepo database.UserRepository, executionRepo database.ExecutionRepository, runner Runner, logger *slog.Logger, provider constants.BackendProvider) *Service {
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
		apiKey, err = generateAPIKey()
		if err != nil {
			return nil, apperrors.ErrInternalError("failed to generate API key", err)
		}
	}

	apiKeyHash := hashAPIKey(apiKey)

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

	apiKeyHash := hashAPIKey(apiKey)

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

// generateAPIKey creates a cryptographically secure random API key.
// The key is base64-encoded and approximately 32 characters long.
func generateAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// hashAPIKey creates a SHA-256 hash of the API key for secure storage.
// NOTICE: we never store plain API keys in the database.
func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))

	return base64.StdEncoding.EncodeToString(hash[:])
}

// RunCommand starts a provider-specific task and records the execution.
func (s *Service) RunCommand(ctx context.Context, userEmail string, req api.ExecutionRequest) (*api.ExecutionResponse, error) {
	if s.executionRepo == nil {
		return nil, apperrors.ErrInternalError("execution repository not configured", nil)
	}

	if req.Command == "" {
		return nil, apperrors.ErrBadRequest("command is required", nil)
	}
	executionID, taskARN, err := s.runner.StartTask(ctx, userEmail, req)
	if err != nil {
		return nil, err
	}

	reqLogger := logger.DeriveRequestLogger(ctx, s.Logger)

	if taskARN != "" {
		reqLogger.Info("provider task started", "executionID", executionID, "taskARN", taskARN)
	}

	// Create execution record
	startedAt := time.Now().UTC()
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
		Status:          "RUNNING",
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
		Status:      "RUNNING",
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
		s.Logger.Debug("fetched log events", "executionID", executionID, "events", events)
		if err != nil {
			return nil, err
		}
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
