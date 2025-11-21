package server

import (
	"encoding/json"
	"net/http"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"
)

// handleCreateSecret handles POST /api/v1/secrets
func (r *Router) handleCreateSecret(w http.ResponseWriter, req *http.Request) {
	var createReq api.CreateSecretRequest
	if err := decodeRequestBody(w, req, &createReq); err != nil {
		return
	}

	user, ok := r.requireAuthenticatedUser(w, req)
	if !ok {
		return
	}

	if err := r.svc.CreateSecret(req.Context(), &createReq, user.Email); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(api.CreateSecretResponse{
		Message: "Secret created successfully",
	})
}

// handleGetSecret handles GET /api/v1/secrets/{name}
func (r *Router) handleGetSecret(w http.ResponseWriter, req *http.Request) {
	name, ok := getRequiredURLParam(w, req, "name")
	if !ok {
		return
	}

	secret, err := r.svc.GetSecret(req.Context(), name)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.GetSecretResponse{
		Secret: secret,
	})
}

// handleListSecrets handles GET /api/v1/secrets
func (r *Router) handleListSecrets(w http.ResponseWriter, req *http.Request) {
	secrets, err := r.svc.ListSecrets(req.Context())
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.ListSecretsResponse{
		Secrets: secrets,
		Total:   len(secrets),
	})
}

// handleUpdateSecret handles PUT /api/v1/secrets/{name}
// Updates secret metadata (description) and/or value in a single request.
func (r *Router) handleUpdateSecret(w http.ResponseWriter, req *http.Request) {
	name, ok := getRequiredURLParam(w, req, "name")
	if !ok {
		return
	}

	var updateReq api.UpdateSecretRequest
	if err := decodeRequestBody(w, req, &updateReq); err != nil {
		return
	}

	user, authOk := r.requireAuthenticatedUser(w, req)
	if !authOk {
		return
	}

	if err := r.svc.UpdateSecret(req.Context(), name, &updateReq, user.Email); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.UpdateSecretResponse{
		Message: "Secret updated successfully",
	})
}

// handleDeleteSecret handles DELETE /api/v1/secrets/{name}
func (r *Router) handleDeleteSecret(w http.ResponseWriter, req *http.Request) {
	name, ok := getRequiredURLParam(w, req, "name")
	if !ok {
		return
	}

	err := r.svc.DeleteSecret(req.Context(), name)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.DeleteSecretResponse{
		Name:    name,
		Message: "Secret deleted successfully",
	})
}

// handleServiceError converts service layer errors to HTTP responses
func handleServiceError(w http.ResponseWriter, err error) {
	statusCode, errorCode, errorDetails := extractErrorInfo(err)
	errorMsg := apperrors.GetErrorMessage(err)
	if errorMsg == "" {
		errorMsg = "Internal server error"
	}
	writeErrorResponseWithCode(w, statusCode, errorCode, errorMsg, errorDetails)
}
