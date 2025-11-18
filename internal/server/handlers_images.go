package server

import (
	"encoding/json"
	"net/http"

	"runvoy/internal/api"
)

// handleRegisterImage handles POST /api/v1/images/register to register a new Docker image.
func (r *Router) handleRegisterImage(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())
	var registerReq api.RegisterImageRequest

	if err := decodeRequestBody(w, req, &registerReq); err != nil {
		return
	}

	user, ok := r.requireAuthenticatedUser(w, req)
	if !ok {
		return
	}

	resp, err := r.svc.RegisterImage(
		req.Context(),
		&registerReq,
		user.Email,
	)
	if err != nil {
		statusCode, errorCode, errorDetails := extractErrorInfo(err)

		logger.Error("failed to register image", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to register image", errorDetails)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleListImages handles GET /api/v1/images to list all registered Docker images.
func (r *Router) handleListImages(w http.ResponseWriter, req *http.Request) {
	r.handleListWithAuth(w, req,
		func() (any, error) { return r.svc.ListImages(req.Context()) },
		"list images")
}

// handleGetImage handles GET /api/v1/images/{image} to get a single registered Docker image.
// The image parameter may contain slashes and colons and uses a catch-all (*) route to match paths with slashes.
func (r *Router) handleGetImage(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	image, ok := getImagePath(w, req)
	if !ok {
		return
	}

	imageInfo, err := r.svc.GetImage(req.Context(), image)
	if err != nil {
		statusCode, errorCode, errorDetails := extractErrorInfo(err)

		logger.Error("failed to get image", "error", err, "status_code", statusCode, "error_code", errorCode)

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

	image, ok := getImagePath(w, req)
	if !ok {
		return
	}

	err := r.svc.RemoveImage(req.Context(), image)
	if err != nil {
		statusCode, errorCode, errorDetails := extractErrorInfo(err)

		logger.Error("failed to remove image", "error", err, "status_code", statusCode, "error_code", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to remove image", errorDetails)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.RemoveImageResponse{
		Image:   image,
		Message: "Image removed successfully",
	})
}
