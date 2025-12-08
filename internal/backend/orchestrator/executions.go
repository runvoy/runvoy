package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/auth/authorization"
	"github.com/runvoy/runvoy/internal/constants"
	apperrors "github.com/runvoy/runvoy/internal/errors"
	"github.com/runvoy/runvoy/internal/logger"
)

// ValidateExecutionResourceAccess checks if a user can access all resources required for execution.
// The resolvedImage parameter contains the image that was resolved from the request and will be validated.
// All secrets referenced in the execution request are also validated for access.
// Returns an error if the user lacks access to any required resource.
func (s *Service) ValidateExecutionResourceAccess(
	ctx context.Context,
	userEmail string,
	req *api.ExecutionRequest,
	resolvedImage *api.ImageInfo,
) error {
	enforcer := s.GetEnforcer()

	if resolvedImage != nil {
		imagePath := "/api/v1/images/" + resolvedImage.ImageID
		allowed, err := enforcer.Enforce(ctx, userEmail, imagePath, authorization.ActionUse)
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

		secretPath := "/api/v1/secrets/" + name
		allowed, err := enforcer.Enforce(ctx, userEmail, secretPath, authorization.ActionUse)
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
	clientIPAtCreationTime *string,
	req *api.ExecutionRequest,
	resolvedImage *api.ImageInfo,
) (*api.ExecutionResponse, error) {
	if req.Command == "" {
		return nil, apperrors.ErrBadRequest("command is required", nil)
	}

	// Always pass and store the resolved image ID when available
	if resolvedImage != nil && resolvedImage.ImageID != "" {
		req.Image = resolvedImage.ImageID
	}

	secretEnvVars, err := s.resolveSecretsForExecution(ctx, req.Secrets)
	if err != nil {
		return nil, err
	}
	s.applyResolvedSecrets(req, secretEnvVars)

	executionID, createdAt, err := s.taskManager.StartTask(ctx, userEmail, req)
	if err != nil {
		return nil, apperrors.ErrInternalError("failed to start task", fmt.Errorf("start task: %w", err))
	}

	if execErr := s.recordExecution(
		ctx, userEmail, req, executionID, createdAt, constants.ExecutionStarting,
	); execErr != nil {
		return nil, fmt.Errorf("failed to record execution: %w", execErr)
	}

	websocketURL := s.wsManager.GenerateWebSocketURL(ctx, executionID, &userEmail, clientIPAtCreationTime)

	imageID := req.Image

	return &api.ExecutionResponse{
		ExecutionID:  executionID,
		Status:       string(constants.ExecutionStarting),
		ImageID:      imageID,
		WebSocketURL: websocketURL,
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

	requestID := logger.ExtractRequestIDFromContext(ctx)
	execution := &api.Execution{
		ExecutionID:         executionID,
		CreatedBy:           userEmail,
		OwnedBy:             []string{userEmail},
		Command:             req.Command,
		ImageID:             req.Image,
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

	if err := s.repos.Execution.CreateExecution(ctx, execution); err != nil {
		reqLogger.Error("failed to create execution record, but task has been accepted by the provider",
			"context", map[string]string{
				"execution_id": executionID,
				"error":        err.Error(),
			},
		)
		return fmt.Errorf("failed to create execution record, but task has been accepted by the provider: %w", err)
	}

	if err := s.addExecutionOwnershipToEnforcer(ctx, executionID, execution.OwnedBy); err != nil {
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

	execution, err := s.repos.Execution.GetExecution(ctx, executionID)
	if err != nil {
		// Wrap the error - AppError types will still be found via errors.As() in the chain
		return nil, fmt.Errorf("get execution: %w", err)
	}
	if execution == nil {
		return nil, apperrors.ErrNotFound("execution not found", nil)
	}

	isTerminal := slices.ContainsFunc(constants.TerminalExecutionStatuses(), func(status constants.ExecutionStatus) bool {
		return execution.Status == string(status)
	})

	if isTerminal {
		// For terminal executions: return events (always an array, even if empty), no websocket URL
		logEvents, fetchErr := s.logManager.FetchLogsByExecutionID(ctx, executionID)
		if fetchErr != nil {
			return nil, apperrors.ErrInternalError("failed to fetch logs", fmt.Errorf("fetch logs: %w", fetchErr))
		}
		// Ensure events is always a slice (never nil) for terminal executions
		if logEvents == nil {
			logEvents = []api.LogEvent{}
		}
		return &api.LogsResponse{
			ExecutionID:  executionID,
			Status:       execution.Status,
			Events:       logEvents,
			WebSocketURL: "", // Empty string will be omitted due to omitempty tag
		}, nil
	}

	// For running executions: return websocket URL only, events is nil
	websocketURL := s.wsManager.GenerateWebSocketURL(ctx, executionID, userEmail, clientIPAtCreationTime)
	return &api.LogsResponse{
		ExecutionID:  executionID,
		Status:       execution.Status,
		Events:       nil, // Explicitly nil for running executions
		WebSocketURL: websocketURL,
	}, nil
}

// FetchTrace retrieves backend logs and related resources for a request ID.
func (s *Service) FetchTrace(ctx context.Context, requestID string) (*api.TraceResponse, error) {
	if requestID == "" {
		return nil, apperrors.ErrBadRequest("requestID is required", nil)
	}

	var (
		logs      []api.LogEvent
		resources api.RelatedResources
	)

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		fetchedLogs, logsErr := s.observabilityManager.FetchBackendLogs(egCtx, requestID)
		if logsErr != nil {
			reqLogger := logger.DeriveRequestLogger(egCtx, s.Logger)
			reqLogger.Error("failed to fetch backend logs", "context", map[string]any{
				"request_id": requestID,
				"error":      logsErr,
			})
			return apperrors.ErrInternalError("failed to fetch backend logs", fmt.Errorf("fetch backend logs: %w", logsErr))
		}
		logs = fetchedLogs
		return nil
	})

	eg.Go(func() error {
		var resourcesErr error
		resources, resourcesErr = s.fetchTraceRelatedResources(egCtx, requestID)
		return resourcesErr
	})

	if err := eg.Wait(); err != nil {
		reqLogger := logger.DeriveRequestLogger(ctx, s.Logger)
		reqLogger.Error("failed to fetch trace", "context", map[string]any{
			"request_id": requestID,
			"error":      err,
		})
		return nil, apperrors.ErrServiceUnavailable("failed to fetch trace", err)
	}

	return &api.TraceResponse{
		Logs:             logs,
		RelatedResources: resources,
	}, nil
}

func (s *Service) fetchTraceRelatedResources(
	ctx context.Context,
	requestID string,
) (api.RelatedResources, error) {
	var (
		executions []*api.Execution
		secrets    []*api.Secret
		users      []*api.User
		images     []api.ImageInfo
	)

	eg, egCtx := errgroup.WithContext(ctx)

	fetchResourceByRequestID(
		egCtx, s, eg, requestID,
		func(execs []*api.Execution) { executions = execs },
		func(fetchCtx context.Context, reqID string) ([]*api.Execution, error) {
			return s.repos.Execution.GetExecutionsByRequestID(fetchCtx, reqID)
		},
		"executions",
	)
	fetchResourceByRequestID(
		egCtx, s, eg, requestID,
		func(secs []*api.Secret) { secrets = secs },
		func(fetchCtx context.Context, reqID string) ([]*api.Secret, error) {
			return s.repos.Secrets.GetSecretsByRequestID(fetchCtx, reqID)
		},
		"secrets",
	)
	fetchResourceByRequestID(
		egCtx, s, eg, requestID,
		func(usrs []*api.User) { users = usrs },
		func(fetchCtx context.Context, reqID string) ([]*api.User, error) {
			return s.repos.User.GetUsersByRequestID(fetchCtx, reqID)
		},
		"users",
	)
	fetchResourceByRequestID(
		egCtx, s, eg, requestID,
		func(imgs []api.ImageInfo) { images = imgs },
		func(fetchCtx context.Context, reqID string) ([]api.ImageInfo, error) {
			return s.repos.Image.GetImagesByRequestID(fetchCtx, reqID)
		},
		"images",
	)

	if err := eg.Wait(); err != nil {
		return api.RelatedResources{},
			apperrors.ErrInternalError(
				"failed to fetch related resources",
				fmt.Errorf("fetch related resources: %w", err),
			)
	}

	return api.RelatedResources{
		Executions: executions,
		Secrets:    secrets,
		Users:      users,
		Images:     images,
	}, nil
}

func fetchResourceByRequestID[T any](
	ctx context.Context,
	svc *Service,
	eg *errgroup.Group,
	requestID string,
	assign func(T),
	fetch func(context.Context, string) (T, error),
	resourceName string,
) {
	eg.Go(func() error {
		resources, fetchErr := fetch(ctx, requestID)
		if fetchErr != nil {
			reqLogger := logger.DeriveRequestLogger(ctx, svc.Logger)
			failureMessage := fmt.Sprintf("failed to fetch %s by request ID", resourceName)
			reqLogger.Error(failureMessage, "context", map[string]any{
				"request_id": requestID,
				"error":      fetchErr,
			})
			return apperrors.ErrDatabaseError(
				failureMessage,
				fmt.Errorf("get %s by request ID: %w", resourceName, fetchErr),
			)
		}
		assign(resources)
		return nil
	})
}

// GetExecutionStatus returns the current status and metadata for a given execution ID.
func (s *Service) GetExecutionStatus(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error) {
	if executionID == "" {
		return nil, apperrors.ErrBadRequest("executionID is required", nil)
	}

	execution, err := s.repos.Execution.GetExecution(ctx, executionID)
	if err != nil {
		// Wrap the error - AppError types will still be found via errors.As() in the chain
		return nil, fmt.Errorf("get execution: %w", err)
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

	if execution.Command == "" || execution.ImageID == "" {
		missing := []string{}
		if execution.Command == "" {
			missing = append(missing, "command")
		}
		if execution.ImageID == "" {
			missing = append(missing, "image_id")
		}
		return nil, apperrors.ErrInternalError(
			"execution is missing required metadata",
			fmt.Errorf("missing %s for execution %s", strings.Join(missing, ","), executionID),
		)
	}

	return &api.ExecutionStatusResponse{
		ExecutionID: execution.ExecutionID,
		Status:      execution.Status,
		Command:     execution.Command,
		ImageID:     execution.ImageID,
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

	execution, err := s.repos.Execution.GetExecution(ctx, executionID)
	if err != nil {
		// Wrap the error - AppError types will still be found via errors.As() in the chain
		return nil, fmt.Errorf("get execution: %w", err)
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

	if killErr := s.taskManager.KillTask(ctx, executionID); killErr != nil {
		return nil, apperrors.ErrInternalError("failed to kill task", fmt.Errorf("kill task: %w", killErr))
	}

	if updateErr := s.updateExecutionStatus(ctx, execution, targetStatus, reqLogger); updateErr != nil {
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

// updateExecutionStatus updates an execution's status and persists it to the database.
func (s *Service) updateExecutionStatus(
	ctx context.Context,
	execution *api.Execution,
	targetStatus constants.ExecutionStatus,
	reqLogger *slog.Logger,
) error {
	execution.Status = string(targetStatus)
	execution.CompletedAt = nil

	requestID := logger.ExtractRequestIDFromContext(ctx)
	if requestID != "" {
		execution.ModifiedByRequestID = requestID
	}

	if updateErr := s.repos.Execution.UpdateExecution(ctx, execution); updateErr != nil {
		reqLogger.Error("failed to update execution status", "context", map[string]string{
			"execution_id": execution.ExecutionID,
			"status":       execution.Status,
			"error":        updateErr.Error(),
		})
		return apperrors.ErrDatabaseError("failed to update execution", fmt.Errorf("update execution: %w", updateErr))
	}

	return nil
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
	executions, err := s.repos.Execution.ListExecutions(ctx, limit, statuses)
	if err != nil {
		// Check if it's already an AppError - if so, wrap it to satisfy wrapcheck
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			return nil, fmt.Errorf("list executions: %w", err)
		}
		// Otherwise, wrap the external error with an AppError
		// Use ErrInternalError for generic errors (test expects 500, not 503)
		return nil, apperrors.ErrInternalError("failed to list executions", fmt.Errorf("list executions: %w", err))
	}
	return executions, nil
}

func (s *Service) addExecutionOwnershipToEnforcer(ctx context.Context, executionID string, ownedBy []string) error {
	resourceID := authorization.FormatResourceID("execution", executionID)
	for _, owner := range ownedBy {
		if err := s.enforcer.AddOwnershipForResource(ctx, resourceID, owner); err != nil {
			return fmt.Errorf("failed to add ownership for execution %s: %w", executionID, err)
		}
	}
	return nil
}
