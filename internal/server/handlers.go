package server

import (
	"encoding/json"
	"net/http"
	"runvoy/internal/api"
	"runvoy/internal/constants"
)

// handleCreateUser handles POST /api/v1/users to create a new user with an API key
func (r *Router) handleCreateUser(w http.ResponseWriter, req *http.Request) {
	logger := getLoggerWithRequestID(r, req.Context())
	var createReq api.CreateUserRequest

	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	resp, err := r.svc.CreateUser(req.Context(), createReq)
	if err != nil {
		statusCode := http.StatusInternalServerError

		if err.Error() == "email is required" ||
			err.Error() == "user with this email already exists" ||
			containsString(err.Error(), "invalid email address") {
			statusCode = http.StatusBadRequest
		}

		if err.Error() == "user with this email already exists" {
			statusCode = http.StatusConflict
		}

		logger.Debug("failed to create user", "error", err)
		writeErrorResponse(w, statusCode, "failed to create user", err.Error())
		return
	}

	// Return successful response with the created user and API key
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// handleRevokeUser handles POST /api/v1/users/revoke to revoke a user's API key
func (r *Router) handleRevokeUser(w http.ResponseWriter, req *http.Request) {
	logger := getLoggerWithRequestID(r, req.Context())
	var revokeReq api.RevokeUserRequest

	if err := json.NewDecoder(req.Body).Decode(&revokeReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if err := r.svc.RevokeUser(req.Context(), revokeReq.Email); err != nil {
		statusCode := http.StatusInternalServerError

		if err.Error() == "email is required" {
			statusCode = http.StatusBadRequest
		}

		if err.Error() == "user not found" {
			statusCode = http.StatusNotFound
		}

		logger.Debug("failed to revoke user", "error", err)
		writeErrorResponse(w, statusCode, "failed to revoke user", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(api.RevokeUserResponse{
		Message: "user API key revoked successfully",
		Email:   revokeReq.Email,
	})
}

// handleHealth returns a simple health check response
func (r *Router) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(api.HealthResponse{
		Status:  "ok",
		Version: *constants.GetVersion(),
	})
}
