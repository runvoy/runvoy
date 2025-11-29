// Package errors provides error types and handling for runvoy.
// It includes custom error types with HTTP status codes and error codes.
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError represents an application error with an associated HTTP status code.
type AppError struct {
	// Code is an optional error code string for programmatic handling
	Code string
	// Message is a user-friendly error message
	Message string
	// StatusCode is the HTTP status code to return
	StatusCode int
	// Cause is the underlying error (for error wrapping)
	Cause error
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the underlying error for error unwrapping.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is allows errors.Is to work with AppError.
func (e *AppError) Is(target error) bool {
	if t, ok := target.(*AppError); ok {
		return e.Code != "" && e.Code == t.Code
	}
	return false
}

// Predefined error codes.
const (
	// Client error codes.
	ErrCodeInvalidRequest = "INVALID_REQUEST"
	ErrCodeUnauthorized   = "UNAUTHORIZED"
	ErrCodeForbidden      = "FORBIDDEN"
	ErrCodeNotFound       = "NOT_FOUND"
	ErrCodeConflict       = "CONFLICT"
	ErrCodeSecretNotFound = "SECRET_NOT_FOUND"
	ErrCodeSecretExists   = "SECRET_ALREADY_EXISTS"
	ErrCodeInvalidAPIKey  = "INVALID_API_KEY" //nolint:gosec // this is not an API key, it's a request error code
	ErrCodeAPIKeyRevoked  = "API_KEY_REVOKED" //nolint:gosec // this is not an API key, it's a request error code

	// Server error codes.
	ErrCodeInternalError      = "INTERNAL_ERROR"
	ErrCodeDatabaseError      = "DATABASE_ERROR"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// NewClientError creates a new client error (4xx status codes).
func NewClientError(statusCode int, code, message string, cause error) *AppError {
	if statusCode < 400 || statusCode >= 500 {
		panic(fmt.Sprintf("NewClientError called with non-client status code: %d", statusCode))
	}
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Cause:      cause,
	}
}

// NewServerError creates a new server error (5xx status codes).
func NewServerError(statusCode int, code, message string, cause error) *AppError {
	if statusCode < 500 || statusCode >= 600 {
		panic(fmt.Sprintf("NewServerError called with non-server status code: %d", statusCode))
	}
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Cause:      cause,
	}
}

// Convenience constructors for common errors

// ErrUnauthorized creates an unauthorized error (401).
func ErrUnauthorized(message string, cause error) *AppError {
	return NewClientError(http.StatusUnauthorized, ErrCodeUnauthorized, message, cause)
}

// ErrForbidden creates a forbidden error (403).
func ErrForbidden(message string, cause error) *AppError {
	return NewClientError(http.StatusForbidden, ErrCodeForbidden, message, cause)
}

// ErrInvalidAPIKey creates an invalid API key error (401).
func ErrInvalidAPIKey(cause error) *AppError {
	return NewClientError(http.StatusUnauthorized, ErrCodeInvalidAPIKey, "Invalid API key", cause)
}

// ErrAPIKeyRevoked creates an API key revoked error (401).
func ErrAPIKeyRevoked(cause error) *AppError {
	return NewClientError(http.StatusUnauthorized, ErrCodeAPIKeyRevoked, "API key has been revoked", cause)
}

// ErrNotFound creates a not found error (404).
func ErrNotFound(message string, cause error) *AppError {
	return NewClientError(http.StatusNotFound, ErrCodeNotFound, message, cause)
}

// ErrConflict creates a conflict error (409).
func ErrConflict(message string, cause error) *AppError {
	return NewClientError(http.StatusConflict, ErrCodeConflict, message, cause)
}

// ErrBadRequest creates a bad request error (400).
func ErrBadRequest(message string, cause error) *AppError {
	return NewClientError(http.StatusBadRequest, ErrCodeInvalidRequest, message, cause)
}

// ErrSecretNotFound creates a secret not found error (404).
func ErrSecretNotFound(message string, cause error) *AppError {
	return NewClientError(http.StatusNotFound, ErrCodeSecretNotFound, message, cause)
}

// ErrSecretAlreadyExists creates a secret already exists error (409).
func ErrSecretAlreadyExists(message string, cause error) *AppError {
	return NewClientError(http.StatusConflict, ErrCodeSecretExists, message, cause)
}

// ErrInternalError creates an internal server error (500).
func ErrInternalError(message string, cause error) *AppError {
	return NewServerError(http.StatusInternalServerError, ErrCodeInternalError, message, cause)
}

// ErrDatabaseError creates a database error (503 Service Unavailable).
// Database failures are typically transient issues.
func ErrDatabaseError(message string, cause error) *AppError {
	return NewServerError(http.StatusServiceUnavailable, ErrCodeDatabaseError, message, cause)
}

// ErrServiceUnavailable creates a service unavailable error (503).
// Use this for resources that are temporarily unavailable but may become available soon.
func ErrServiceUnavailable(message string, cause error) *AppError {
	return NewServerError(http.StatusServiceUnavailable, ErrCodeServiceUnavailable, message, cause)
}

// GetStatusCode extracts the HTTP status code from an error.
// Returns 500 if the error is not an AppError.
func GetStatusCode(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.StatusCode
	}
	return http.StatusInternalServerError
}

// GetErrorCode extracts the error code from an error.
// Returns empty string if the error is not an AppError.
func GetErrorCode(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

// GetErrorMessage extracts a user-friendly message from an error.
func GetErrorMessage(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return err.Error()
}

// GetErrorDetails extracts detailed error information including the underlying cause.
// Returns the underlying error message if available, otherwise returns the main error message.
func GetErrorDetails(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		if appErr.Cause != nil {
			return appErr.Cause.Error()
		}
		return appErr.Message
	}
	return err.Error()
}
