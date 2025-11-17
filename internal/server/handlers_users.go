package server

import (
	"encoding/json"
	"net/http"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"
)

// handleCreateUser handles POST /api/v1/users to create a new user with an API key.
func (r *Router) handleCreateUser(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())
	var createReq api.CreateUserRequest

	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	user, ok := r.getUserFromContext(req)
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "user not found in context")
		return
	}

	resp, err := r.svc.CreateUser(req.Context(), createReq, user.Email)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorDetails := apperrors.GetErrorDetails(err)

		logger.Debug("failed to create user", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to create user", errorDetails)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleRevokeUser handles POST /api/v1/users/revoke to revoke a user's API key.
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
		errorDetails := apperrors.GetErrorDetails(err)

		logger.Debug("failed to revoke user", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to revoke user", errorDetails)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.RevokeUserResponse{
		Message: "user API key revoked successfully",
		Email:   revokeReq.Email,
	})
}

// handleListUsers handles GET /api/v1/users to list all users.
func (r *Router) handleListUsers(w http.ResponseWriter, req *http.Request) {
	r.handleListWithAuth(w, req, "you do not have permission to list users",
		func() (any, error) { return r.svc.ListUsers(req.Context()) },
		"list users")
}
