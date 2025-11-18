package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// handleClaimAPIKey handles GET /claim/{token} to claim a pending API key.
func (r *Router) handleClaimAPIKey(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	secretToken := strings.TrimSpace(chi.URLParam(req, "token"))
	if secretToken == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid token", "token is required")
		return
	}

	ipAddress := getClientIP(req)

	claimResp, err := r.svc.ClaimAPIKey(req.Context(), secretToken, ipAddress)
	if err != nil {
		statusCode, errorCode, errorDetails := extractErrorInfo(err)

		logger.Debug("failed to claim API key", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to claim API key", errorDetails)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(claimResp)
}
