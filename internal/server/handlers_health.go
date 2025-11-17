package server

import (
	"encoding/json"
	"net/http"

	"runvoy/internal/api"
	"runvoy/internal/backend/health"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
)

// handleHealth returns a simple health check response.
func (r *Router) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{
		Status:  "ok",
		Version: *constants.GetVersion(),
	})
}

// handleReconcileHealth triggers a full health reconciliation across managed resources.
// It requires authentication and is intended for admin/maintenance use.
func (r *Router) handleReconcileHealth(w http.ResponseWriter, req *http.Request) {
	report, err := r.svc.ReconcileResources(req.Context())
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorDetails := apperrors.GetErrorDetails(err)

		writeErrorResponseWithCode(
			w,
			statusCode,
			errorCode,
			"failed to reconcile resources",
			errorDetails,
		)
		return
	}

	if report == nil {
		writeErrorResponse(w, http.StatusInternalServerError,
			"health report is nil", "health reconciliation returned no report")
		return
	}

	response := struct {
		Status string         `json:"status"`
		Report *health.Report `json:"report"`
	}{
		Status: "ok",
		Report: report,
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
