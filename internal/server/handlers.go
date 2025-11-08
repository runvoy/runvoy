// Package server implements the HTTP server and handlers for runvoy.
// It provides REST API endpoints for user management and command execution.
package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"

	"github.com/go-chi/chi/v5"
)

// getUserFromContext extracts the authenticated user from request context
// Returns the user and true if found, or nil and false if not found
// Callers should check the boolean return value before using the user
func (r *Router) getUserFromContext(req *http.Request) (*api.User, bool) {
	user, ok := req.Context().Value(userContextKey).(*api.User)
	return user, ok && user != nil
}

// handleCreateUser handles POST /api/v1/users to create a new user with an API key
func (r *Router) handleCreateUser(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())
	var createReq api.CreateUserRequest

	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())

		return
	}

	// Extract admin user from context
	user, ok := r.getUserFromContext(req)
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "user not found in context")
		return
	}

	resp, err := r.svc.CreateUser(req.Context(), createReq, user.Email)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to create user", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to create user", errorMsg)

		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleRevokeUser handles POST /api/v1/users/revoke to revoke a user's API key
func (r *Router) handleRevokeUser(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())
	var revokeReq api.RevokeUserRequest

	if err := json.NewDecoder(req.Body).Decode(&revokeReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())

		return
	}

	if err := r.svc.RevokeUser(req.Context(), revokeReq.Email); err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to revoke user", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to revoke user", errorMsg)

		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.RevokeUserResponse{
		Message: "user API key revoked successfully",
		Email:   revokeReq.Email,
	})
}

// handleListUsers handles GET /api/v1/users to list all users
func (r *Router) handleListUsers(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	resp, err := r.svc.ListUsers(req.Context())
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to list users", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to list users", errorMsg)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleRunCommand handles POST /api/v1/run to execute a command in an ephemeral container
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

	resp, err := r.svc.RunCommand(req.Context(), user.Email, &execReq)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Error("failed to run command", "context", map[string]string{
			"error":       err.Error(),
			"status_code": strconv.Itoa(statusCode),
			"error_code":  errorCode,
		})

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to run command", errorMsg)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGetExecutionLogs handles GET /api/v1/executions/{executionID}/logs to fetch logs for an execution
func (r *Router) handleGetExecutionLogs(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	executionID := strings.TrimSpace(chi.URLParam(req, "executionID"))
	if executionID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid execution id", "executionID is required")
		return
	}

	// Extract authenticated user from context
	user, ok := r.getUserFromContext(req)
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "user not found in context")
		return
	}

	// Extract client IP for tracing
	clientIP := getClientIP(req)

	lastSeenQuery := strings.TrimSpace(req.URL.Query().Get("last_seen_timestamp"))
	var lastSeenTimestamp *int64
	if lastSeenQuery != "" {
		parsedTimestamp, parseErr := strconv.ParseInt(lastSeenQuery, 10, 64)
		if parseErr != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid last_seen_timestamp", "must be a valid integer timestamp")
			return
		}
		lastSeenTimestamp = &parsedTimestamp
	}

	resp, err := r.svc.GetLogsByExecutionID(req.Context(), executionID, &user.Email, &clientIP, lastSeenTimestamp)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to get execution logs", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to get execution logs", errorMsg)

		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGetExecutionStatus handles GET /api/v1/executions/{executionID}/status to fetch execution status
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
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to get execution status",
			"execution_id", executionID,
			"error", err,
			"status_code", statusCode,
			"error_code", errorCode)

		writeErrorResponseWithCode(
			w, statusCode, errorCode,
			"failed to get execution status for executionID "+executionID,
			errorMsg,
		)
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleKillExecution handles POST /api/v1/executions/{executionID}/kill to terminate a running execution
func (r *Router) handleKillExecution(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	executionID := strings.TrimSpace(chi.URLParam(req, "executionID"))
	if executionID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid execution id", "executionID is required")
		return
	}

	err := r.svc.KillExecution(req.Context(), executionID)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Info("failed to kill execution", "context", map[string]any{
			"execution_id": executionID,
			"error":        err,
			"status_code":  statusCode,
			"error_code":   errorCode,
			"error_msg":    errorMsg,
		})

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to kill execution", errorMsg)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.KillExecutionResponse{
		ExecutionID: executionID,
		Message:     "Execution termination initiated",
	})
}

// handleListExecutions handles GET /api/v1/executions to list all executions
func (r *Router) handleListExecutions(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	executions, err := r.svc.ListExecutions(req.Context())
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to list executions", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to list executions", errorMsg)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(executions)
}

// handleHealth returns a simple health check response
func (r *Router) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{
		Status:  "ok",
		Version: *constants.GetVersion(),
	})
}

// handleClaimAPIKey handles GET /claim/{token} to claim a pending API key
func (r *Router) handleClaimAPIKey(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	// Extract token from URL path
	secretToken := strings.TrimSpace(chi.URLParam(req, "token"))
	if secretToken == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid token", "token is required")
		return
	}

	// Get client IP address
	ipAddress := getClientIP(req)

	// Claim the API key
	claimResp, err := r.svc.ClaimAPIKey(req.Context(), secretToken, ipAddress)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to claim API key", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to claim API key", errorMsg)
		return
	}

	// Return JSON response
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(claimResp)
}

// handleRegisterImage handles POST /api/v1/images/register to register a new Docker image
func (r *Router) handleRegisterImage(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())
	var registerReq api.RegisterImageRequest

	if err := json.NewDecoder(req.Body).Decode(&registerReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	resp, err := r.svc.RegisterImage(req.Context(), registerReq.Image, registerReq.IsDefault)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to register image", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to register image", errorMsg)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleListImages handles GET /api/v1/images to list all registered Docker images
func (r *Router) handleListImages(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	resp, err := r.svc.ListImages(req.Context())
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to list images", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to list images", errorMsg)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleRemoveImage handles DELETE /api/v1/images/{image} to remove a registered Docker image
// The image parameter may contain slashes and colons (e.g.,
// "ecr-public.us-east-1.amazonaws.com/docker/library/ubuntu:22.04")
// Uses catch-all route (*) to match paths with slashes
func (r *Router) handleRemoveImage(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	imagePath := strings.TrimPrefix(strings.TrimSpace(chi.URLParam(req, "*")), "/")

	if imagePath == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid image", "image parameter is required")
		return
	}

	image, decodeErr := url.PathUnescape(imagePath)
	if decodeErr != nil {
		image = imagePath
	}
	image = strings.TrimSpace(image)
	if image == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid image", "image parameter is required")
		return
	}

	err := r.svc.RemoveImage(req.Context(), image)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to remove image", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to remove image", errorMsg)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.RemoveImageResponse{
		Image:   image,
		Message: "Image removed successfully",
	})
}

// getClientIP extracts the client IP address from request headers
func getClientIP(req *http.Request) string {
	// Check X-Forwarded-For header (used by proxies/load balancers)
	xff := req.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header (alternative proxy header)
	xRealIP := req.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return strings.TrimSpace(xRealIP)
	}

	// Fall back to RemoteAddr, stripping the port if present
	ip := req.RemoteAddr
	if colonIndex := strings.LastIndex(ip, ":"); colonIndex != -1 {
		ip = ip[:colonIndex]
	}
	return ip
}
