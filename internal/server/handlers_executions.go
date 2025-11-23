package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	"runvoy/internal/errors"
)

// handleRunCommand handles POST /api/v1/run to execute a command in an ephemeral container.
// The handler resolves the requested image to a specific imageID, validates the user has access
// to that image and any requested secrets, then starts the execution task.
func (r *Router) handleRunCommand(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	user, ok := r.requireAuthenticatedUser(w, req)
	if !ok {
		return
	}

	var execReq api.ExecutionRequest
	if err := decodeRequestBody(w, req, &execReq); err != nil {
		return
	}

	resolvedImage, err := r.svc.ResolveImage(req.Context(), execReq.Image)
	if err != nil {
		statusCode, errorCode, errorDetails := extractErrorInfo(err)

		logger.Error("failed to resolve image",
			"error", err,
			"status_code", statusCode,
			"error_code", errorCode,
			"image", execReq.Image)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to resolve image", errorDetails)
		return
	}

	accessErr := r.svc.ValidateExecutionResourceAccess(
		req.Context(), user.Email, &execReq, resolvedImage)
	if accessErr != nil {
		statusCode, errorCode, errorDetails := extractErrorInfo(accessErr)

		logger.Error("authorization denied for execution resources",
			"error", accessErr,
			"status_code", statusCode,
			"error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "forbidden", errorDetails)
		return
	}

	resp, err := r.svc.RunCommand(req.Context(), user.Email, &execReq, resolvedImage)
	if err != nil {
		statusCode, errorCode, errorDetails := extractErrorInfo(err)

		logger.Error("failed to run command", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to run command", errorDetails)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGetExecutionLogs handles GET /api/v1/executions/{executionID}/logs to fetch logs for an execution.
func (r *Router) handleGetExecutionLogs(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	executionID, ok := getRequiredURLParam(w, req, "executionID")
	if !ok {
		return
	}

	user, ok := r.requireAuthenticatedUser(w, req)
	if !ok {
		return
	}

	clientIP := getClientIP(req)

	resp, err := r.svc.GetLogsByExecutionID(req.Context(), executionID, &user.Email, &clientIP)
	if err != nil {
		statusCode, errorCode, errorDetails := extractErrorInfo(err)

		logger.Error("failed to get execution logs", "context", map[string]any{
			"execution_id": executionID,
			"error":        err,
			"status_code":  statusCode,
			"error_code":   errorCode,
		})

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to get execution logs", errorDetails)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGetBackendLogsTrace handles GET /api/v1/trace/{requestID} to query
// backend infrastructure logs and related resources by request ID.
func (r *Router) handleGetBackendLogsTrace(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	requestID, ok := getRequiredURLParam(w, req, "requestID")
	if !ok {
		writeErrorResponseWithCode(w, http.StatusBadRequest, errors.ErrCodeInvalidRequest, "requestID is required", "")
		return
	}

	trace, err := r.svc.FetchTrace(req.Context(), requestID)
	if err != nil {
		statusCode, errorCode, errorDetails := extractErrorInfo(err)

		logger.Error("failed to fetch trace", "context", map[string]any{
			"request_id":  requestID,
			"error":       err,
			"status_code": statusCode,
			"error_code":  errorCode,
		})

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to fetch trace", errorDetails)
		return
	}

	logger.Info("trace query completed", "context", map[string]any{
		"request_id":       requestID,
		"log_count":        len(trace.Logs),
		"executions_count": len(trace.RelatedResources.Executions),
		"secrets_count":    len(trace.RelatedResources.Secrets),
		"users_count":      len(trace.RelatedResources.Users),
		"images_count":     len(trace.RelatedResources.Images),
	})

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(trace)
}

// handleGetExecutionStatus handles GET /api/v1/executions/{executionID}/status to fetch execution status.
func (r *Router) handleGetExecutionStatus(w http.ResponseWriter, req *http.Request) {
	executionID, ok := getRequiredURLParam(w, req, "executionID")
	if !ok {
		return
	}

	resp, err := r.svc.GetExecutionStatus(req.Context(), executionID)
	if err != nil {
		logger := r.GetLoggerFromContext(req.Context())
		statusCode, errorCode, errorDetails := extractErrorInfo(err)

		logger.Error("failed to get execution status",
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

// handleKillExecution handles DELETE /api/v1/executions/{executionID} to terminate a running execution.
func (r *Router) handleKillExecution(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	executionID, ok := getRequiredURLParam(w, req, "executionID")
	if !ok {
		return
	}

	resp, err := r.svc.KillExecution(req.Context(), executionID)
	if err != nil {
		statusCode, errorCode, errorDetails := extractErrorInfo(err)

		logger.Error("failed to kill execution",
			"context", map[string]any{
				"execution_id": executionID,
				"error":        err,
				"status_code":  statusCode,
				"error_code":   errorCode,
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
			logger.Debug("invalid limit parameter", "context", map[string]any{
				"error": err,
				"limit": limitParam,
			})
			writeErrorResponseWithCode(w, http.StatusBadRequest, "invalid_request", "invalid limit parameter", "")
			return
		}
		if parsedLimit < 0 {
			logger.Debug("invalid limit parameter", "context", map[string]any{
				"error": "limit must be >= 0",
				"limit": limitParam,
			})
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
		statusCode, errorCode, errorDetails := extractErrorInfo(err)

		logger.Error("failed to list executions", "context", map[string]any{
			"error":       err,
			"status_code": statusCode,
			"error_code":  errorCode,
		})

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to list executions", errorDetails)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(executions)
}
