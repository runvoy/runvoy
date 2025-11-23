package errors

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		expected string
	}{
		{
			name: "error with cause",
			err: &AppError{
				Code:       ErrCodeInvalidRequest,
				Message:    "validation failed",
				StatusCode: http.StatusBadRequest,
				Cause:      errors.New("field x is required"),
			},
			expected: "validation failed: field x is required",
		},
		{
			name: "error without cause",
			err: &AppError{
				Code:       ErrCodeNotFound,
				Message:    "resource not found",
				StatusCode: http.StatusNotFound,
				Cause:      nil,
			},
			expected: "resource not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &AppError{
		Code:       ErrCodeInternalError,
		Message:    "something went wrong",
		StatusCode: http.StatusInternalServerError,
		Cause:      cause,
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, cause, unwrapped)
}

func TestAppError_Is(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		target   error
		expected bool
	}{
		{
			name: "same error code matches",
			err: &AppError{
				Code:       ErrCodeUnauthorized,
				Message:    "unauthorized",
				StatusCode: http.StatusUnauthorized,
			},
			target: &AppError{
				Code:       ErrCodeUnauthorized,
				Message:    "different message",
				StatusCode: http.StatusUnauthorized,
			},
			expected: true,
		},
		{
			name: "different error codes don't match",
			err: &AppError{
				Code:       ErrCodeUnauthorized,
				Message:    "unauthorized",
				StatusCode: http.StatusUnauthorized,
			},
			target: &AppError{
				Code:       ErrCodeNotFound,
				Message:    "not found",
				StatusCode: http.StatusNotFound,
			},
			expected: false,
		},
		{
			name: "empty code doesn't match",
			err: &AppError{
				Code:       "",
				Message:    "error",
				StatusCode: http.StatusInternalServerError,
			},
			target: &AppError{
				Code:       "",
				Message:    "error",
				StatusCode: http.StatusInternalServerError,
			},
			expected: false,
		},
		{
			name: "non-AppError doesn't match",
			err: &AppError{
				Code:       ErrCodeUnauthorized,
				Message:    "unauthorized",
				StatusCode: http.StatusUnauthorized,
			},
			target:   errors.New("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Is(tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewClientError(t *testing.T) {
	t.Run("valid client error", func(t *testing.T) {
		err := NewClientError(http.StatusBadRequest, ErrCodeInvalidRequest, "test error", nil)
		assert.Equal(t, ErrCodeInvalidRequest, err.Code)
		assert.Equal(t, "test error", err.Message)
		assert.Equal(t, http.StatusBadRequest, err.StatusCode)
	})

	t.Run("panics with non-client status code", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = NewClientError(http.StatusInternalServerError, "CODE", "test", nil)
		})
	})

	t.Run("panics with status code below 400", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = NewClientError(http.StatusOK, "CODE", "test", nil)
		})
	})
}

func TestNewServerError(t *testing.T) {
	t.Run("valid server error", func(t *testing.T) {
		err := NewServerError(http.StatusInternalServerError, ErrCodeInternalError, "test error", nil)
		assert.Equal(t, ErrCodeInternalError, err.Code)
		assert.Equal(t, "test error", err.Message)
		assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
	})

	t.Run("panics with non-server status code", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = NewServerError(http.StatusBadRequest, "CODE", "test", nil)
		})
	})

	t.Run("panics with status code 600+", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = NewServerError(600, "CODE", "test", nil)
		})
	})
}

func TestErrUnauthorized(t *testing.T) {
	err := ErrUnauthorized("access denied", nil)
	assert.Equal(t, ErrCodeUnauthorized, err.Code)
	assert.Equal(t, "access denied", err.Message)
	assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
}

func TestErrForbidden(t *testing.T) {
	err := ErrForbidden("access forbidden", nil)
	assert.Equal(t, ErrCodeForbidden, err.Code)
	assert.Equal(t, "access forbidden", err.Message)
	assert.Equal(t, http.StatusForbidden, err.StatusCode)
}

func TestErrInvalidAPIKey(t *testing.T) {
	err := ErrInvalidAPIKey(nil)
	assert.Equal(t, ErrCodeInvalidAPIKey, err.Code)
	assert.Equal(t, "Invalid API key", err.Message)
	assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
}

func TestErrAPIKeyRevoked(t *testing.T) {
	err := ErrAPIKeyRevoked(nil)
	assert.Equal(t, ErrCodeAPIKeyRevoked, err.Code)
	assert.Equal(t, "API key has been revoked", err.Message)
	assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
}

func TestErrNotFound(t *testing.T) {
	err := ErrNotFound("user not found", nil)
	assert.Equal(t, ErrCodeNotFound, err.Code)
	assert.Equal(t, "user not found", err.Message)
	assert.Equal(t, http.StatusNotFound, err.StatusCode)
}

func TestErrConflict(t *testing.T) {
	err := ErrConflict("resource already exists", nil)
	assert.Equal(t, ErrCodeConflict, err.Code)
	assert.Equal(t, "resource already exists", err.Message)
	assert.Equal(t, http.StatusConflict, err.StatusCode)
}

func TestErrBadRequest(t *testing.T) {
	err := ErrBadRequest("invalid input", nil)
	assert.Equal(t, ErrCodeInvalidRequest, err.Code)
	assert.Equal(t, "invalid input", err.Message)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
}

func TestErrInternalError(t *testing.T) {
	err := ErrInternalError("internal server error", nil)
	assert.Equal(t, ErrCodeInternalError, err.Code)
	assert.Equal(t, "internal server error", err.Message)
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
}

func TestErrDatabaseError(t *testing.T) {
	err := ErrDatabaseError("database connection failed", nil)
	assert.Equal(t, ErrCodeDatabaseError, err.Code)
	assert.Equal(t, "database connection failed", err.Message)
	assert.Equal(t, http.StatusServiceUnavailable, err.StatusCode)
}

func TestErrServiceUnavailable(t *testing.T) {
	err := ErrServiceUnavailable("resource not ready yet", nil)
	assert.Equal(t, ErrCodeServiceUnavailable, err.Code)
	assert.Equal(t, "resource not ready yet", err.Message)
	assert.Equal(t, http.StatusServiceUnavailable, err.StatusCode)
}

func TestErrSecretNotFound(t *testing.T) {
	err := ErrSecretNotFound("secret not found", nil)
	assert.Equal(t, ErrCodeSecretNotFound, err.Code)
	assert.Equal(t, "secret not found", err.Message)
	assert.Equal(t, http.StatusNotFound, err.StatusCode)
}

func TestErrSecretAlreadyExists(t *testing.T) {
	err := ErrSecretAlreadyExists("secret already exists", nil)
	assert.Equal(t, ErrCodeSecretExists, err.Code)
	assert.Equal(t, "secret already exists", err.Message)
	assert.Equal(t, http.StatusConflict, err.StatusCode)
}

func TestGetStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "AppError returns its status code",
			err:      ErrNotFound("test", nil),
			expected: http.StatusNotFound,
		},
		{
			name:     "wrapped AppError returns its status code",
			err:      errors.Join(ErrBadRequest("test", nil), errors.New("other")),
			expected: http.StatusBadRequest,
		},
		{
			name:     "non-AppError returns 500",
			err:      errors.New("generic error"),
			expected: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStatusCode(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "AppError returns its error code",
			err:      ErrUnauthorized("test", nil),
			expected: ErrCodeUnauthorized,
		},
		{
			name:     "wrapped AppError returns its error code",
			err:      errors.Join(ErrConflict("test", nil), errors.New("other")),
			expected: ErrCodeConflict,
		},
		{
			name:     "non-AppError returns empty string",
			err:      errors.New("generic error"),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetErrorCode(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "AppError returns its message",
			err:      ErrNotFound("resource not found", nil),
			expected: "resource not found",
		},
		{
			name:     "wrapped AppError returns its message",
			err:      errors.Join(ErrBadRequest("bad input", nil), errors.New("other")),
			expected: "bad input",
		},
		{
			name:     "non-AppError returns error string",
			err:      errors.New("generic error"),
			expected: "generic error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetErrorMessage(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetErrorDetails(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "AppError with cause returns cause message",
			err:      ErrNotFound("resource not found", errors.New("underlying cause")),
			expected: "underlying cause",
		},
		{
			name:     "AppError without cause returns main message",
			err:      ErrNotFound("resource not found", nil),
			expected: "resource not found",
		},
		{
			name:     "wrapped AppError with cause returns cause message",
			err:      errors.Join(ErrBadRequest("bad input", errors.New("validation failed")), errors.New("other")),
			expected: "validation failed",
		},
		{
			name:     "non-AppError returns error string",
			err:      errors.New("generic error"),
			expected: "generic error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetErrorDetails(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	t.Run("can wrap and unwrap errors", func(t *testing.T) {
		baseErr := errors.New("base error")
		appErr := ErrInternalError("wrapped error", baseErr)

		// Test errors.Is
		require.True(t, errors.Is(appErr, baseErr))

		// Test errors.As
		var targetErr *AppError
		require.True(t, errors.As(appErr, &targetErr))
		assert.Equal(t, ErrCodeInternalError, targetErr.Code)
	})

	t.Run("error chain preserves AppError properties", func(t *testing.T) {
		baseErr := errors.New("base error")
		appErr := ErrBadRequest("validation failed", baseErr)

		// Extract status code from wrapped error
		statusCode := GetStatusCode(appErr)
		assert.Equal(t, http.StatusBadRequest, statusCode)

		// Extract error code from wrapped error
		errorCode := GetErrorCode(appErr)
		assert.Equal(t, ErrCodeInvalidRequest, errorCode)
	})
}
