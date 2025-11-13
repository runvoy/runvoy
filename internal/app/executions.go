package app

import (
	"context"
	"fmt"
	"slices"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"
)

// RunCommand starts a provider-specific task and records the execution.
// Resolve secret references to environment variables before starting the task.
// Set execution status to STARTING after the task has been accepted by the provider.
func (s *Service) RunCommand(
	ctx context.Context,
	userEmail string,
	req *api.ExecutionRequest,
) (*api.ExecutionResponse, error) {
	if s.executionRepo == nil {
		return nil, apperrors.ErrInternalError("execution repository not configured", nil)
	}

	if req.Command == "" {
		return nil, apperrors.ErrBadRequest("command is required", nil)
	}

	secretEnvVars, err := s.resolveSecretsForExecution(ctx, req.Secrets)
	if err != nil {
		return nil, err
	}
	s.applyResolvedSecrets(req, secretEnvVars)

	executionID, createdAt, err := s.runner.StartTask(ctx, userEmail, req)
	if err != nil {
		return nil, err
	}

	if execErr := s.recordExecution(
		ctx, userEmail, req, executionID, createdAt, constants.ExecutionStarting,
	); execErr != nil {
		return nil, fmt.Errorf("failed to record execution: %w", execErr)
	}

	return &api.ExecutionResponse{
		ExecutionID: executionID,
		Status:      string(constants.ExecutionStarting),
	}, nil
}

func (s *Service) recordExecution(
	ctx context.Context,
	userEmail string,
	req *api.ExecutionRequest,
	executionID string,
	createdAt *time.Time,
	status constants.ExecutionStatus,
) error {
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
		StartedAt:       startedAt,
		Status:          string(status),
		RequestID:       requestID,
		ComputePlatform: string(s.Provider),
	}

	if requestID == "" {
		reqLogger.Warn("request ID not available; storing execution without request ID",
			"execution_id", executionID,
		)
	}

	if err := s.executionRepo.CreateExecution(ctx, execution); err != nil {
		reqLogger.Error("failed to create execution record, but task has been accepted by the provider",
			"context", map[string]string{
				"execution_id": executionID,
				"error":        err.Error(),
			},
		)
		return fmt.Errorf("failed to create execution record, but task has been accepted by the provider: %w", err)
	}

	return nil
}

// GetLogsByExecutionID returns aggregated Cloud logs for a given execution.
// WebSocket endpoint is stored without protocol (normalized in config).
// Always use wss:// for production WebSocket connections.
// userEmail: authenticated user email for audit trail.
// clientIPAtCreationTime: client IP captured when the token was created (for tracing).
// If task is not running, don't return a WebSocket URL.
func (s *Service) GetLogsByExecutionID(
	ctx context.Context,
	executionID string,
	userEmail *string,
	clientIPAtCreationTime *string,
) (*api.LogsResponse, error) {
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

	events, err := s.runner.FetchLogsByExecutionID(ctx, executionID)
	if err != nil {
		return nil, err
	}

	var websocketURL string
	if s.wsManager != nil {
		websocketURL = s.wsManager.GenerateWebSocketURL(ctx, executionID, userEmail, clientIPAtCreationTime)
	}

	return &api.LogsResponse{
		ExecutionID:  executionID,
		Status:       execution.Status,
		Events:       events,
		WebSocketURL: websocketURL,
	}, nil
}

// GetExecutionStatus returns the current status and metadata for a given execution ID.
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
		// Only populate ExitCode if we have actually recorded completion.
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
// Updates the execution status to TERMINATING after successful task stop.
// Returns an error if the execution is not found, already terminated, or termination fails.
func (s *Service) KillExecution(ctx context.Context, executionID string) error {
	if s.executionRepo == nil {
		return apperrors.ErrInternalError("execution repository not configured", nil)
	}
	if executionID == "" {
		return apperrors.ErrBadRequest("executionID is required", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, s.Logger)

	execution, err := s.executionRepo.GetExecution(ctx, executionID)
	if err != nil {
		return err
	}
	if execution == nil {
		return apperrors.ErrNotFound("execution not found", nil)
	}

	reqLogger.Debug("execution found", "execution_id", executionID, "status", execution.Status)

	terminalStatuses := constants.TerminalExecutionStatuses()
	if slices.ContainsFunc(terminalStatuses, func(status constants.ExecutionStatus) bool {
		return execution.Status == string(status)
	}) {
		return apperrors.ErrBadRequest(
			"execution is already terminated",
			fmt.Errorf("execution status: %s", execution.Status))
	}

	if killErr := s.runner.KillTask(ctx, executionID); killErr != nil {
		return killErr
	}

	reqLogger.Info("task kill command sent successfully, updating execution status to TERMINATING",
		"context", map[string]string{
			"execution_id": executionID,
		},
	)

	execution.Status = string(constants.ExecutionTerminating)
	execution.CompletedAt = nil

	if updateErr := s.executionRepo.UpdateExecution(ctx, execution); updateErr != nil {
		reqLogger.Error("failed to update execution status to TERMINATING", "context", map[string]string{
			"execution_id": executionID,
			"status":       execution.Status,
			"error":        updateErr.Error(),
		})
		return updateErr
	}

	reqLogger.Info("execution status updated to TERMINATING",
		"context", map[string]string{
			"execution_id": executionID,
			"status":       execution.Status,
		},
	)

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
