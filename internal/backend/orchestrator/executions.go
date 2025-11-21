package orchestrator

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/logger"
)

// ValidateExecutionResourceAccess checks if a user can access all resources required for execution.
// The resolvedImage parameter contains the image that was resolved from the request and will be validated.
// All secrets referenced in the execution request are also validated for access.
// Returns an error if the user lacks access to any required resource.
func (s *Service) ValidateExecutionResourceAccess(
	userEmail string,
	req *api.ExecutionRequest,
	resolvedImage *api.ImageInfo,
) error {
	enforcer := s.GetEnforcer()

	if resolvedImage != nil {
		imagePath := fmt.Sprintf("/api/v1/images/%s", resolvedImage.ImageID)
		allowed, err := enforcer.Enforce(userEmail, imagePath, authorization.ActionUse)
		if err != nil {
			return apperrors.ErrInternalError(
				"failed to validate image access",
				fmt.Errorf("enforcement error: %w", err),
			)
		}
		if !allowed {
			return apperrors.ErrForbidden(
				fmt.Sprintf(
					"you do not have permission to execute with image %q (resolved to %s)",
					req.Image,
					resolvedImage.ImageID,
				),
				nil,
			)
		}
	}

	for _, secretName := range req.Secrets {
		name := strings.TrimSpace(secretName)
		if name == "" {
			continue
		}

		secretPath := fmt.Sprintf("/api/v1/secrets/%s", name)
		allowed, err := enforcer.Enforce(userEmail, secretPath, authorization.ActionUse)
		if err != nil {
			return apperrors.ErrInternalError(
				"failed to validate secret access",
				fmt.Errorf("enforcement error for secret %q: %w", name, err),
			)
		}
		if !allowed {
			return apperrors.ErrForbidden(
				fmt.Sprintf(
					"you do not have permission to use secret %q",
					name,
				),
				nil,
			)
		}
	}

	return nil
}

// RunCommand starts a provider-specific task and records the execution.
// The resolvedImage parameter contains the validated image that will be used for execution.
// The request's Image field is replaced with the imageID before passing to the runner.
// Secret references are resolved to environment variables before starting the task.
// Execution status is set to STARTING after the task has been accepted by the provider.
func (s *Service) RunCommand(
	ctx context.Context,
	userEmail string,
	req *api.ExecutionRequest,
	resolvedImage *api.ImageInfo,
) (*api.ExecutionResponse, error) {
	if req.Command == "" {
		return nil, apperrors.ErrBadRequest("command is required", nil)
	}

	if resolvedImage != nil {
		req.Image = resolvedImage.ImageID
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
		ImageID:     resolvedImage.ImageID,
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
		ExecutionID:         executionID,
		CreatedBy:           userEmail,
		OwnedBy:             []string{userEmail},
		Command:             req.Command,
		StartedAt:           startedAt,
		Status:              string(status),
		CreatedByRequestID:  requestID,
		ModifiedByRequestID: requestID,
		ComputePlatform:     string(s.Provider),
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

	if err := s.addExecutionOwnershipToEnforcer(executionID, execution.OwnedBy); err != nil {
		reqLogger.Error("failed to synchronize execution ownership with enforcer", "context", map[string]string{
			"execution_id": executionID,
			"user":         userEmail,
			"error":        err.Error(),
		})
		return fmt.Errorf("failed to synchronize execution ownership: %w", err)
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
		isTerminal := slices.ContainsFunc(constants.TerminalExecutionStatuses(), func(status constants.ExecutionStatus) bool {
			return execution.Status == string(status)
		})
		if !isTerminal {
			websocketURL = s.wsManager.GenerateWebSocketURL(ctx, executionID, userEmail, clientIPAtCreationTime)
		}
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
//
// This operation is idempotent: if the execution is already in a terminal state (SUCCEEDED, FAILED,
// STOPPED, TERMINATING), it returns nil, nil (which results in HTTP 204 No Content), indicating
// that no action was taken.
// If termination is initiated, returns a KillExecutionResponse with the execution ID and a success message.
//
// Returns an error if the execution is not found or termination fails.
func (s *Service) KillExecution(ctx context.Context, executionID string) (*api.KillExecutionResponse, error) {
	if executionID == "" {
		return nil, apperrors.ErrBadRequest("executionID is required", nil)
	}

	reqLogger := logger.DeriveRequestLogger(ctx, s.Logger)

	execution, err := s.executionRepo.GetExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if execution == nil {
		return nil, apperrors.ErrNotFound("execution not found", nil)
	}

	currentStatus := constants.ExecutionStatus(execution.Status)
	targetStatus := constants.ExecutionTerminating

	if !constants.CanTransition(currentStatus, targetStatus) {
		reqLogger.Info("execution already in terminal state, no action taken",
			"context", map[string]any{
				"execution_id": executionID,
				"status":       currentStatus,
			})
		return nil, nil
	}

	if killErr := s.runner.KillTask(ctx, executionID); killErr != nil {
		return nil, killErr
	}

	execution.Status = string(targetStatus)
	execution.CompletedAt = nil

	// Extract request ID from context and set ModifiedByRequestID
	requestID := logger.GetRequestID(ctx)
	if requestID != "" {
		execution.ModifiedByRequestID = requestID
	}

	if updateErr := s.executionRepo.UpdateExecution(ctx, execution); updateErr != nil {
		reqLogger.Error("failed to update execution status", "context", map[string]string{
			"execution_id": executionID,
			"status":       execution.Status,
			"error":        updateErr.Error(),
		})
		return nil, updateErr
	}

	reqLogger.Info("execution updated successfully", "context", map[string]any{
		"execution_id": executionID,
		"status":       execution.Status,
		"started_at":   execution.StartedAt.String(),
	})

	return &api.KillExecutionResponse{
		ExecutionID: executionID,
		Message:     "Execution termination initiated",
	}, nil
}

// ListExecutions returns executions from the database with optional filtering.
// Parameters:
//   - limit: maximum number of executions to return
//   - statuses: optional list of execution statuses to filter by
//
// If statuses is provided, only executions matching one of the specified statuses are returned.
// Results are returned sorted by started_at in descending order (newest first).
// Fields with no values are omitted in JSON due to omitempty tags on api.Execution.
func (s *Service) ListExecutions(ctx context.Context, limit int, statuses []string) ([]*api.Execution, error) {
	executions, err := s.executionRepo.ListExecutions(ctx, limit, statuses)
	if err != nil {
		return nil, err
	}
	return executions, nil
}

func (s *Service) addExecutionOwnershipToEnforcer(executionID string, ownedBy []string) error {
	resourceID := authorization.FormatResourceID("execution", executionID)
	for _, owner := range ownedBy {
		if err := s.enforcer.AddOwnershipForResource(resourceID, owner); err != nil {
			return fmt.Errorf("failed to add ownership for execution %s: %w", executionID, err)
		}
	}
	return nil
}
