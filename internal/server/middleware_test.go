package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"runvoy/internal/app"
	"runvoy/internal/constants"
	"runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

func TestRequestIDMiddleware(t *testing.T) {
	// Test without Lambda context - should generate a random ID
	t.Run("without lambda context generates random UUID", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := logger.GetRequestID(r.Context())
			if requestID == "" {
				t.Error("Expected a generated request ID, got empty string")
			}
			if len(requestID) != 32 {
				t.Errorf("Expected hex ID format (32 chars), got length %d: %s", len(requestID), requestID)
			}
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		svc := app.NewService(nil, nil, nil, slog.Default(), constants.AWS)
		router := &Router{svc: svc}
		middleware := router.requestIDMiddleware(handler)
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	// Test with Lambda context - should use Lambda's request ID
	t.Run("with lambda context uses lambda request ID", func(t *testing.T) {
		lambdaHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := logger.GetRequestID(r.Context())
			if requestID != "test-request-id-123" {
				t.Errorf("Expected request ID 'test-request-id-123', got '%s'", requestID)
			}
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		// Add Lambda context with request ID
		lc := &lambdacontext.LambdaContext{
			AwsRequestID: "test-request-id-123",
		}
		ctx := lambdacontext.NewContext(req.Context(), lc)
		req = req.WithContext(ctx)

		svc := app.NewService(nil, nil, nil, slog.Default(), constants.AWS)
		router := &Router{svc: svc}
		middleware := router.requestIDMiddleware(lambdaHandler)
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	// Test with existing request ID in context - should use the existing one
	t.Run("with existing request ID in context uses existing ID", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := logger.GetRequestID(r.Context())
			if requestID != "existing-request-id-456" {
				t.Errorf("Expected request ID 'existing-request-id-456', got '%s'", requestID)
			}
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		// Add existing request ID to context
		ctx := logger.WithRequestID(req.Context(), "existing-request-id-456")
		req = req.WithContext(ctx)

		svc := app.NewService(nil, nil, nil, slog.Default(), constants.AWS)
		router := &Router{svc: svc}
		middleware := router.requestIDMiddleware(handler)
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	// Test priority: existing context ID should take precedence over Lambda ID
	t.Run("existing context ID takes precedence over lambda ID", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := logger.GetRequestID(r.Context())
			if requestID != "context-priority-id" {
				t.Errorf("Expected request ID 'context-priority-id', got '%s'", requestID)
			}
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		// Add both existing request ID and Lambda context
		ctx := logger.WithRequestID(req.Context(), "context-priority-id")
		lc := &lambdacontext.LambdaContext{
			AwsRequestID: "lambda-request-id-should-be-ignored",
		}
		ctx = lambdacontext.NewContext(ctx, lc)
		req = req.WithContext(ctx)

		svc := app.NewService(nil, nil, nil, slog.Default(), constants.AWS)
		router := &Router{svc: svc}
		middleware := router.requestIDMiddleware(handler)
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})
}

func TestGetRequestID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "context with request ID",
			ctx:      logger.WithRequestID(context.Background(), "test-id"),
			expected: "test-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logger.GetRequestID(tt.ctx)
			if result != tt.expected {
				t.Errorf("GetRequestID() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestResponseWriter(t *testing.T) {
	t.Run("captures status code from WriteHeader", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := &responseWriter{
			ResponseWriter: rr,
			statusCode:     http.StatusOK,
		}

		rw.WriteHeader(http.StatusNotFound)
		if rw.statusCode != http.StatusNotFound {
			t.Errorf("Expected status code %d, got %d", http.StatusNotFound, rw.statusCode)
		}
	})

	t.Run("captures status code from Write", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := &responseWriter{
			ResponseWriter: rr,
			statusCode:     http.StatusOK,
		}

		_, _ = rw.Write([]byte("test"))
		if rw.statusCode != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, rw.statusCode)
		}
		if rw.written != true {
			t.Error("Expected written flag to be true")
		}
	})

	t.Run("WriteHeader only sets status once", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := &responseWriter{
			ResponseWriter: rr,
			statusCode:     http.StatusOK,
		}

		rw.WriteHeader(http.StatusBadRequest)
		rw.WriteHeader(http.StatusInternalServerError)
		if rw.statusCode != http.StatusBadRequest {
			t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, rw.statusCode)
		}
	})

	t.Run("writes to underlying ResponseWriter", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := &responseWriter{
			ResponseWriter: rr,
			statusCode:     http.StatusOK,
		}

		testData := []byte("test data")
		n, err := rw.Write(testData)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if n != len(testData) {
			t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
		}

		// Verify data was written to underlying recorder
		body, _ := io.ReadAll(rr.Body)
		if string(body) != "test data" {
			t.Errorf("Expected body 'test data', got '%s'", string(body))
		}
	})
}
