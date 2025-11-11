package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"runvoy/internal/api"
	"runvoy/internal/app"
	"runvoy/internal/database"
	appErrors "runvoy/internal/errors"

	"github.com/go-chi/chi/v5"
)

// handleCreateSecret handles POST /api/v1/secrets
func (r *Router) handleCreateSecret(w http.ResponseWriter, req *http.Request) {
	var createReq api.CreateSecretRequest
	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	user := req.Context().Value(userContextKey).(*api.User)
	svc := req.Context().Value(serviceContextKey).(*app.Service)
	secretsManager := svc.GetSecretsManager()

	if secretsManager == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "secrets service not available", "")
		return
	}

	secret, err := secretsManager.CreateSecret(req.Context(), &createReq, user.Email)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(api.CreateSecretResponse{
		Secret:  secret,
		Message: "Secret created successfully",
	})
}

// handleGetSecret handles GET /api/v1/secrets/{name}
func (r *Router) handleGetSecret(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	if name == "" {
		writeErrorResponse(w, http.StatusBadRequest, "secret name is required", "")
		return
	}

	svc := req.Context().Value(serviceContextKey).(*app.Service)
	secretsManager := svc.GetSecretsManager()

	if secretsManager == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "secrets service not available", "")
		return
	}

	secret, err := secretsManager.GetSecret(req.Context(), name)
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
	user := req.Context().Value(userContextKey).(*api.User)
	svc := req.Context().Value(serviceContextKey).(*app.Service)
	secretsManager := svc.GetSecretsManager()

	if secretsManager == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "secrets service not available", "")
		return
	}

	secrets, err := secretsManager.ListSecrets(req.Context(), user.Email)
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
	name := chi.URLParam(req, "name")
	if name == "" {
		writeErrorResponse(w, http.StatusBadRequest, "secret name is required", "")
		return
	}

	var updateReq api.UpdateSecretRequest
	if err := json.NewDecoder(req.Body).Decode(&updateReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	user := req.Context().Value(userContextKey).(*api.User)
	svc := req.Context().Value(serviceContextKey).(*app.Service)
	secretsManager := svc.GetSecretsManager()

	if secretsManager == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "secrets service not available", "")
		return
	}

	secret, err := secretsManager.UpdateSecret(req.Context(), name, &updateReq, user.Email)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.UpdateSecretResponse{
		Secret:  secret,
		Message: "Secret updated successfully",
	})
}

// handleDeleteSecret handles DELETE /api/v1/secrets/{name}
func (r *Router) handleDeleteSecret(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	if name == "" {
		writeErrorResponse(w, http.StatusBadRequest, "secret name is required", "")
		return
	}

	svc := req.Context().Value(serviceContextKey).(*app.Service)
	secretsManager := svc.GetSecretsManager()

	if secretsManager == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "secrets service not available", "")
		return
	}

	err := secretsManager.DeleteSecret(req.Context(), name)
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
	var appErr *appErrors.AppError

	switch {
	case errors.As(err, &appErr):
		writeErrorResponse(w, appErr.StatusCode, appErr.Message, "")
	case errors.Is(err, database.ErrSecretNotFound):
		writeErrorResponse(w, http.StatusNotFound, "Secret not found", "")
	case errors.Is(err, database.ErrSecretAlreadyExists):
		writeErrorResponse(w, http.StatusConflict, "Secret already exists", "")
	default:
		writeErrorResponse(w, http.StatusInternalServerError, "Internal server error", err.Error())
	}
}
