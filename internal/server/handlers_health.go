package server

import (
	"encoding/json"
	"net/http"

	"runvoy/internal/api"
	"runvoy/internal/constants"
)

// handleHealth returns a simple health check response.
func (r *Router) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(constants.ContentTypeHeader, "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{
		Status:   "ok",
		Version:  *constants.GetVersion(),
		Region:   r.svc.Region,
		Provider: r.svc.Provider,
	})
}

// handleReconcileHealth triggers a full health reconciliation across managed resources.
// It requires authentication and is intended for admin/maintenance use.
func (r *Router) handleReconcileHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(constants.ContentTypeHeader, "application/json")
	report, err := r.svc.ReconcileResources(req.Context())
	if err != nil {
		statusCode, errorCode, errorDetails := extractErrorInfo(err)

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
		Status string            `json:"status"`
		Report *api.HealthReport `json:"report"`
	}{
		Status: "ok",
		Report: report,
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
