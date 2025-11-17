package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"

	"github.com/go-chi/chi/v5"
)

// handleRegisterImage handles POST /api/v1/images/register to register a new Docker image.
func (r *Router) handleRegisterImage(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())
	var registerReq api.RegisterImageRequest

	if err := json.NewDecoder(req.Body).Decode(&registerReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if !r.authorizeRequest(req, "create") {
		writeErrorResponse(w, http.StatusForbidden, "Forbidden", "you do not have permission to register images")
		return
	}

	resp, err := r.svc.RegisterImage(
		req.Context(),
		registerReq.Image,
		registerReq.IsDefault,
		registerReq.TaskRoleName,
		registerReq.TaskExecutionRoleName,
		registerReq.CPU,
		registerReq.Memory,
		registerReq.RuntimePlatform,
	)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorDetails := apperrors.GetErrorDetails(err)

		logger.Debug("failed to register image", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to register image", errorDetails)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleListImages handles GET /api/v1/images to list all registered Docker images.
func (r *Router) handleListImages(w http.ResponseWriter, req *http.Request) {
	r.handleListWithAuth(w, req, "you do not have permission to list images",
		func() (any, error) { return r.svc.ListImages(req.Context()) },
		"list images")
}

// handleGetImage handles GET /api/v1/images/{image} to get a single registered Docker image.
// The image parameter may contain slashes and colons and uses a catch-all (*) route to match paths with slashes.
func (r *Router) handleGetImage(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	if !r.authorizeRequest(req, "read") {
		writeErrorResponse(w, http.StatusForbidden, "Forbidden", "you do not have permission to read images")
		return
	}

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

	imageInfo, err := r.svc.GetImage(req.Context(), image)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorDetails := apperrors.GetErrorDetails(err)

		logger.Debug("failed to get image", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to get image", errorDetails)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(imageInfo)
}

// handleRemoveImage handles DELETE /api/v1/images/{image} to remove a registered Docker image.
// The image parameter may contain slashes and colons and uses a catch-all (*) route to match paths with slashes.
func (r *Router) handleRemoveImage(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	if !r.authorizeRequest(req, "delete") {
		writeErrorResponse(w, http.StatusForbidden, "Forbidden", "you do not have permission to remove images")
		return
	}

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
		errorDetails := apperrors.GetErrorDetails(err)

		logger.Debug("failed to remove image", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to remove image", errorDetails)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.RemoveImageResponse{
		Image:   image,
		Message: "Image removed successfully",
	})
}
