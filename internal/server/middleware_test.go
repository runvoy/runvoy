package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"runvoy/internal/app"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

func TestRequestIDMiddleware(t *testing.T) {
	// Create a test handler that checks for request ID in context
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r.Context())
		// Request ID should be set (empty string if no Lambda context)
		_ = requestID // Just verify it doesn't panic
		w.WriteHeader(http.StatusOK)
	})

	// Test without Lambda context
	t.Run("without lambda context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		// Create a Router with a test service for the middleware
		svc := app.NewService(nil, nil, nil, slog.Default())
		router := &Router{svc: svc}
		middleware := router.requestIDMiddleware(handler)
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	// Test with Lambda context
	t.Run("with lambda context", func(t *testing.T) {
		// Create a handler that specifically checks for the expected request ID
		lambdaHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := GetRequestID(r.Context())
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

		// Create a Router with a test service for the middleware
		svc := app.NewService(nil, nil, nil, slog.Default())
		router := &Router{svc: svc}
		middleware := router.requestIDMiddleware(lambdaHandler)
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
			ctx:      context.WithValue(context.Background(), requestIDContextKey, "test-id"),
			expected: "test-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetRequestID(tt.ctx)
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

		rw.Write([]byte("test"))
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
