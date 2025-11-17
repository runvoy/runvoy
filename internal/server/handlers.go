// Package server implements the HTTP server and handlers for runvoy.
// It provides REST API endpoints for user management and command execution.
package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"
)

// getUserFromContext extracts the authenticated user from request context
// Returns the user and true if found, or nil and false if not found
// Callers should check the boolean return value before using the user
func (r *Router) getUserFromContext(req *http.Request) (*api.User, bool) {
	user, ok := req.Context().Value(userContextKey).(*api.User)
	return user, ok && user != nil
}

// handleListWithAuth handles the common pattern for list operations with authorization.
// Checks user authentication, authorization, calls the service method, and writes response.
func (r *Router) handleListWithAuth(
	w http.ResponseWriter,
	req *http.Request,
	denialMsg string,
	serviceCall func() (any, error),
	operationName string,
) {
	logger := r.GetLoggerFromContext(req.Context())

	user, ok := r.getUserFromContext(req)
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "user not found in context")
		return
	}

	if !r.authorizeRequest(req.Context(), user.Email, req.URL.Path, "read") {
		writeErrorResponse(w, http.StatusForbidden, "Forbidden", denialMsg)
		return
	}

	resp, err := serviceCall()
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorDetails := apperrors.GetErrorDetails(err)

		logger.Debug("failed to "+operationName,
			"error", err,
			"status_code", statusCode,
			"error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to "+operationName, errorDetails)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
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
