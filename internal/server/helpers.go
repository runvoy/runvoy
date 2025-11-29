package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/runvoy/runvoy/internal/api"
	apperrors "github.com/runvoy/runvoy/internal/errors"

	"github.com/go-chi/chi/v5"
)

// extractErrorInfo extracts statusCode, errorCode, and errorDetails from an error.
// Returns the HTTP status code, error code, and error details.
func extractErrorInfo(err error) (statusCode int, errorCode, errorDetails string) {
	return apperrors.GetStatusCode(err),
		apperrors.GetErrorCode(err),
		apperrors.GetErrorDetails(err)
}

// decodeRequestBody decodes JSON request body into the provided value.
// If decoding fails, writes an error response and returns the error.
// Returns nil on success.
func decodeRequestBody(w http.ResponseWriter, req *http.Request, v any) error {
	if err := json.NewDecoder(req.Body).Decode(v); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return fmt.Errorf("failed to decode request body: %w", err)
	}
	return nil
}

// requireAuthenticatedUser extracts and validates the authenticated user from request context.
// If the user is not found, writes an unauthorized error response and returns nil, false.
// Returns the user and true on success.
func (r *Router) requireAuthenticatedUser(w http.ResponseWriter, req *http.Request) (*api.User, bool) {
	user, ok := r.getUserFromContext(req)
	if !ok {
		writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "user not found in context")
		return nil, false
	}
	return user, true
}

// getRequiredURLParam extracts and validates a required URL parameter.
// If the parameter is missing or empty, writes a bad request error response and returns "", false.
// Returns the parameter value and true on success.
func getRequiredURLParam(w http.ResponseWriter, req *http.Request, name string) (string, bool) {
	param := strings.TrimSpace(chi.URLParam(req, name))
	if param == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid "+name, name+" is required")
		return "", false
	}
	return param, true
}

// getImagePath extracts and validates the image path from the catch-all (*) route parameter.
// Handles URL unescaping and path normalization.
// If the image path is missing or empty, writes a bad request error response and returns "", false.
// Returns the normalized image path and true on success.
func getImagePath(w http.ResponseWriter, req *http.Request) (string, bool) {
	imagePath := strings.TrimPrefix(strings.TrimSpace(chi.URLParam(req, "*")), "/")
	if imagePath == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid image", "image parameter is required")
		return "", false
	}

	image, decodeErr := url.PathUnescape(imagePath)
	if decodeErr != nil {
		image = imagePath
	}
	image = strings.TrimSpace(image)
	if image == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid image", "image parameter is required")
		return "", false
	}
	return image, true
}

// handleAndLogError logs an error and writes a standardized error response.
// Extracts HTTP status code, error code, and error details from the error,
// logs them with context, and writes a formatted error response.
// Use this for all service call failures in handlers.
//
// Example:
//
//	if err := r.svc.DeleteSecret(req.Context(), name); err != nil {
//	    r.handleAndLogError(w, req, err, "delete secret")
//	    return
//	}
func (r *Router) handleAndLogError(
	w http.ResponseWriter,
	req *http.Request,
	err error,
	operationName string,
) {
	logger := r.GetLoggerFromContext(req.Context())
	statusCode, errorCode, errorDetails := extractErrorInfo(err)

	logger.Error(
		"operation failed",
		"operation", operationName,
		"error", err,
		"status_code", statusCode,
		"error_code", errorCode,
	)

	writeErrorResponseWithCode(w, statusCode, errorCode, "failed to "+operationName, errorDetails)
}
