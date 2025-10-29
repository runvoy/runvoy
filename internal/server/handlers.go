package server

import (
	"encoding/json"
	"net/http"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
)

// handleCreateUser handles POST /api/v1/users to create a new user with an API key
func (r *Router) handleCreateUser(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())
	var createReq api.CreateUserRequest

	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())

		return
	}

	resp, err := r.svc.CreateUser(req.Context(), createReq)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to create user", "error", err, "statusCode", statusCode, "errorCode", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to create user", errorMsg)

		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
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

		logger.Debug("failed to revoke user", "error", err, "statusCode", statusCode, "errorCode", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to revoke user", errorMsg)

		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(api.RevokeUserResponse{
		Message: "user API key revoked successfully",
		Email:   revokeReq.Email,
	})
}

// handleRunCommand handles POST /api/v1/run to execute a command in an ECS Fargate container
func (r *Router) handleRunCommand(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	// Get authenticated user from context
	user, ok := req.Context().Value(userContextKey).(*api.User)
	if !ok || user == nil {
		writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "user not found in context")
		return
	}

	var execReq api.ExecutionRequest
	if err := json.NewDecoder(req.Body).Decode(&execReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	resp, err := r.svc.RunCommand(req.Context(), user.Email, execReq)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to run command", "error", err, "statusCode", statusCode, "errorCode", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to run command", errorMsg)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}

// handleHealth returns a simple health check response
func (r *Router) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(api.HealthResponse{
		Status:  "ok",
		Version: *constants.GetVersion(),
	})
}
