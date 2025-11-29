package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/runvoy/runvoy/internal/errors"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractErrorInfo(t *testing.T) {
	tests := []struct {
		name             string
		err              error
		wantStatusCode   int
		wantErrorCode    string
		wantErrorDetails string
	}{
		{
			name:             "not found error",
			err:              apperrors.ErrNotFound("resource not found", nil),
			wantStatusCode:   http.StatusNotFound,
			wantErrorCode:    "NOT_FOUND",
			wantErrorDetails: "",
		},
		{
			name:             "bad request error",
			err:              apperrors.ErrBadRequest("invalid input", nil),
			wantStatusCode:   http.StatusBadRequest,
			wantErrorCode:    "INVALID_REQUEST",
			wantErrorDetails: "",
		},
		{
			name:             "database error",
			err:              apperrors.ErrDatabaseError("connection failed", nil),
			wantStatusCode:   http.StatusServiceUnavailable,
			wantErrorCode:    "DATABASE_ERROR",
			wantErrorDetails: "",
		},
		{
			name:             "invalid api key error",
			err:              apperrors.ErrInvalidAPIKey(nil),
			wantStatusCode:   http.StatusUnauthorized,
			wantErrorCode:    "INVALID_API_KEY",
			wantErrorDetails: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusCode, errorCode, _ := extractErrorInfo(tt.err)

			assert.Equal(t, tt.wantStatusCode, statusCode)
			assert.Equal(t, tt.wantErrorCode, errorCode)
		})
	}
}

func TestDecodeRequestBody(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantErr    bool
		wantStatus int
	}{
		{
			name:       "valid json",
			body:       `{"name": "test", "value": 123}`,
			wantErr:    false,
			wantStatus: 0,
		},
		{
			name:       "invalid json",
			body:       `{invalid json`,
			wantErr:    true,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			body:       ``,
			wantErr:    true,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "null body",
			body:       `null`,
			wantErr:    false,
			wantStatus: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte(tt.body)))
			w := httptest.NewRecorder()

			var result map[string]any
			err := decodeRequestBody(w, req, &result)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.wantStatus, w.Code)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDecodeRequestBody_StructType(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	t.Run("decode into struct", func(t *testing.T) {
		body := `{"name": "test", "value": 42}`
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte(body)))
		w := httptest.NewRecorder()

		var result TestStruct
		err := decodeRequestBody(w, req, &result)

		require.NoError(t, err)
		assert.Equal(t, "test", result.Name)
		assert.Equal(t, 42, result.Value)
	})

	t.Run("decode with extra fields", func(t *testing.T) {
		body := `{"name": "test", "value": 42, "extra": "ignored"}`
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte(body)))
		w := httptest.NewRecorder()

		var result TestStruct
		err := decodeRequestBody(w, req, &result)

		require.NoError(t, err)
		assert.Equal(t, "test", result.Name)
		assert.Equal(t, 42, result.Value)
	})
}

func TestGetRequiredURLParam(t *testing.T) {
	tests := []struct {
		name       string
		paramName  string
		paramValue string
		wantOK     bool
		wantValue  string
	}{
		{
			name:       "valid parameter",
			paramName:  "id",
			paramValue: "exec-123",
			wantOK:     true,
			wantValue:  "exec-123",
		},
		{
			name:       "empty parameter",
			paramName:  "id",
			paramValue: "",
			wantOK:     false,
			wantValue:  "",
		},
		{
			name:       "whitespace only parameter",
			paramName:  "id",
			paramValue: "   ",
			wantOK:     false,
			wantValue:  "",
		},
		{
			name:       "parameter with whitespace",
			paramName:  "id",
			paramValue: "  exec-123  ",
			wantOK:     true,
			wantValue:  "exec-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			w := httptest.NewRecorder()

			// Set up chi route context with the parameter value
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add(tt.paramName, tt.paramValue)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Call the function directly since we've manually set up the context
			gotValue, gotOK := getRequiredURLParam(w, req, tt.paramName)

			assert.Equal(t, tt.wantOK, gotOK)
			if tt.wantOK {
				assert.Equal(t, tt.wantValue, gotValue)
			} else {
				assert.Equal(t, http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestGetImagePath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantOK     bool
		wantValue  string
		wantStatus int
	}{
		{
			name:      "simple image path",
			path:      "alpine:latest",
			wantOK:    true,
			wantValue: "alpine:latest",
		},
		{
			name:      "image with registry",
			path:      "docker.io/library/ubuntu:22.04",
			wantOK:    true,
			wantValue: "docker.io/library/ubuntu:22.04",
		},
		{
			name:       "empty path",
			path:       "",
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "whitespace only",
			path:       "   ",
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:      "url encoded path",
			path:      "alpine%3Alatest",
			wantOK:    true,
			wantValue: "alpine:latest",
		},
		{
			name:      "path with leading slash stripped",
			path:      "/alpine:latest",
			wantOK:    true,
			wantValue: "alpine:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/images/test", http.NoBody)
			w := httptest.NewRecorder()

			// Set up chi route context with the catch-all parameter
			rctx := chi.NewRouteContext()
			// For catch-all routes, chi uses "*" as the parameter name
			rctx.URLParams.Add("*", tt.path)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Call the function directly since we've manually set up the context
			gotValue, gotOK := getImagePath(w, req)

			assert.Equal(t, tt.wantOK, gotOK)
			if tt.wantOK {
				assert.Equal(t, tt.wantValue, gotValue)
			} else {
				assert.Equal(t, tt.wantStatus, w.Code)
				// Verify error response
				var errResp map[string]string
				err := json.NewDecoder(w.Body).Decode(&errResp)
				require.NoError(t, err)
				assert.Contains(t, errResp["error"], "invalid image")
			}
		})
	}
}

func TestGetImagePath_ComplexPaths(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantValue string
	}{
		{
			name:      "ECR image",
			path:      "123456789.dkr.ecr.us-east-1.amazonaws.com/my-image:v1.0",
			wantValue: "123456789.dkr.ecr.us-east-1.amazonaws.com/my-image:v1.0",
		},
		{
			name:      "GCR image",
			path:      "gcr.io/my-project/my-image:latest",
			wantValue: "gcr.io/my-project/my-image:latest",
		},
		{
			name:      "image with digest",
			path:      "alpine@sha256:abc123",
			wantValue: "alpine@sha256:abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			var gotValue string
			var gotOK bool

			r.Get("/images/*", func(w http.ResponseWriter, req *http.Request) {
				gotValue, gotOK = getImagePath(w, req)
			})

			req := httptest.NewRequest(http.MethodGet, "/images/"+tt.path, http.NoBody)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.True(t, gotOK)
			assert.Equal(t, tt.wantValue, gotValue)
		})
	}
}
