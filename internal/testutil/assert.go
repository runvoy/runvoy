package testutil

import (
	stderrors "errors"
	"testing"

	apperrors "github.com/runvoy/runvoy/internal/errors"

	"github.com/stretchr/testify/assert"
)

// AssertErrorType checks if the error is of a specific type using errors.Is.
func AssertErrorType(t *testing.T, err, target error, _ ...any) bool {
	t.Helper()
	if !stderrors.Is(err, target) {
		return assert.Fail(t, "Error type mismatch", "Expected error type %T, got %T", target, err)
	}
	return true
}

// AssertAppErrorCode checks if the error has a specific error code.
func AssertAppErrorCode(t *testing.T, err error, expectedCode string, _ ...any) bool {
	t.Helper()
	code := apperrors.GetErrorCode(err)
	if code != expectedCode {
		return assert.Fail(t, "Error code mismatch", "Expected error code %q, got %q", expectedCode, code)
	}
	return true
}

// AssertAppErrorStatus checks if the error has a specific HTTP status code.
func AssertAppErrorStatus(t *testing.T, err error, expectedStatus int, _ ...any) bool {
	t.Helper()
	status := apperrors.GetStatusCode(err)
	if status != expectedStatus {
		return assert.Fail(t, "Status code mismatch", "Expected status %d, got %d", expectedStatus, status)
	}
	return true
}

// AssertNoError is a wrapper around assert.NoError with context.
func AssertNoError(t *testing.T, err error, msgAndArgs ...any) bool {
	t.Helper()
	return assert.NoError(t, err, msgAndArgs...)
}

// AssertError is a wrapper around assert.Error with context.
func AssertError(t *testing.T, err error, msgAndArgs ...any) bool {
	t.Helper()
	return assert.Error(t, err, msgAndArgs...)
}

// AssertEqual is a wrapper around assert.Equal with context.
func AssertEqual(t *testing.T, expected, actual any, msgAndArgs ...any) bool {
	t.Helper()
	return assert.Equal(t, expected, actual, msgAndArgs...)
}

// AssertNotEmpty is a wrapper around assert.NotEmpty with context.
func AssertNotEmpty(t *testing.T, obj any, msgAndArgs ...any) bool {
	t.Helper()
	return assert.NotEmpty(t, obj, msgAndArgs...)
}

// AssertNil is a wrapper around assert.Nil with context.
func AssertNil(t *testing.T, obj any, msgAndArgs ...any) bool {
	t.Helper()
	return assert.Nil(t, obj, msgAndArgs...)
}

// AssertNotNil is a wrapper around assert.NotNil with context.
func AssertNotNil(t *testing.T, obj any, msgAndArgs ...any) bool {
	t.Helper()
	return assert.NotNil(t, obj, msgAndArgs...)
}
