package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"

	"github.com/go-chi/chi/v5"
)

// handleRunCommand handles POST /api/v1/run to execute a command in an ephemeral container.
func (r *Router) handleRunCommand(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	user, ok := r.getUserFromContext(req)
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "user not found in context")
		return
	}

	var execReq api.ExecutionRequest
	if err := json.NewDecoder(req.Body).Decode(&execReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if err := r.svc.ValidateExecutionResourceAccess(user.Email, &execReq); err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorDetails := apperrors.GetErrorDetails(err)

		logger.Error("authorization denied for execution resources", "context", map[string]string{
			"error":       err.Error(),
			"status_code": strconv.Itoa(statusCode),
			"error_code":  errorCode,
		})

		writeErrorResponseWithCode(w, statusCode, errorCode, "forbidden", errorDetails)
		return
	}

	resp, err := r.svc.RunCommand(req.Context(), user.Email, &execReq)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorDetails := apperrors.GetErrorDetails(err)

		logger.Error("failed to run command", "context", map[string]string{
			"error":       err.Error(),
			"status_code": strconv.Itoa(statusCode),
			"error_code":  errorCode,
		})

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to run command", errorDetails)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGetExecutionLogs handles GET /api/v1/executions/{executionID}/logs to fetch logs for an execution.
func (r *Router) handleGetExecutionLogs(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	executionID := strings.TrimSpace(chi.URLParam(req, "executionID"))
	if executionID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid execution id", "executionID is required")
		return
	}

	user, ok := r.getUserFromContext(req)
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "user not found in context")
		return
	}

	clientIP := getClientIP(req)

	resp, err := r.svc.GetLogsByExecutionID(req.Context(), executionID, &user.Email, &clientIP)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorDetails := apperrors.GetErrorDetails(err)

		logger.Debug("failed to get execution logs", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to get execution logs", errorDetails)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGetExecutionStatus handles GET /api/v1/executions/{executionID}/status to fetch execution status.
func (r *Router) handleGetExecutionStatus(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	executionID := strings.TrimSpace(chi.URLParam(req, "executionID"))
	if executionID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid execution id", "executionID is required")
		return
	}

	resp, err := r.svc.GetExecutionStatus(req.Context(), executionID)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorDetails := apperrors.GetErrorDetails(err)

		logger.Debug("failed to get execution status",
			"execution_id", executionID,
			"error", err,
			"status_code", statusCode,
			"error_code", errorCode)

		writeErrorResponseWithCode(
			w, statusCode, errorCode,
			"failed to get execution status for executionID "+executionID,
			errorDetails,
		)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleKillExecution handles DELETE /api/v1/executions/{executionID}/kill to terminate a running execution.
func (r *Router) handleKillExecution(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	executionID := strings.TrimSpace(chi.URLParam(req, "executionID"))
	if executionID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid execution id", "executionID is required")
		return
	}

	resp, err := r.svc.KillExecution(req.Context(), executionID)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorDetails := apperrors.GetErrorDetails(err)

		logger.Info("failed to kill execution", "context", map[string]any{
			"execution_id":  executionID,
			"error":         err,
			"status_code":   statusCode,
			"error_code":    errorCode,
			"error_details": errorDetails,
		})

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to kill execution", errorDetails)
		return
	}

	if resp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleListExecutions handles GET /api/v1/executions to list executions with optional filtering.
// Query parameters:
//   - limit: maximum number of executions to return (default: 10, use 0 to return all)
//   - status: comma-separated list of execution statuses to filter by (e.g., "RUNNING,TERMINATING")
//
// Example: GET /api/v1/executions?limit=20&status=RUNNING,TERMINATING
func (r *Router) handleListExecutions(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	limit := constants.DefaultExecutionListLimit
	if limitParam := req.URL.Query().Get("limit"); limitParam != "" {
		parsedLimit, err := strconv.Atoi(limitParam)
		if err != nil {
			logger.Debug("invalid limit parameter", "error", err, "limit", limitParam)
			writeErrorResponseWithCode(w, http.StatusBadRequest, "invalid_request", "invalid limit parameter", "")
			return
		}
		if parsedLimit < 0 {
			logger.Debug("invalid limit parameter", "error", "limit must be >= 0", "limit", limitParam)
			writeErrorResponseWithCode(w, http.StatusBadRequest, "invalid_request", "invalid limit parameter", "")
			return
		}
		limit = parsedLimit
	}

	var statuses []string
	if statusParam := req.URL.Query().Get("status"); statusParam != "" {
		statuses = strings.Split(statusParam, ",")
		for i, s := range statuses {
			statuses[i] = strings.TrimSpace(s)
		}
	}

	executions, err := r.svc.ListExecutions(req.Context(), limit, statuses)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorDetails := apperrors.GetErrorDetails(err)

		logger.Debug("failed to list executions", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to list executions", errorDetails)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(executions)
}
