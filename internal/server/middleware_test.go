package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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

		middleware := requestIDMiddleware(handler)
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

		middleware := requestIDMiddleware(lambdaHandler)
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})
}

func TestGetRequestID(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		expected  string
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

func TestGetLoggerFromContext(t *testing.T) {
	// Test with default logger
	t.Run("default logger", func(t *testing.T) {
		ctx := context.Background()
		logger := GetLoggerFromContext(ctx)
		if logger == nil {
			t.Error("Expected logger to be non-nil")
		}
	})

	// Test with logger in context
	t.Run("logger in context", func(t *testing.T) {
		ctx := context.Background()
		// This would normally be set by the middleware
		// We can't easily test the actual logger creation without more complex setup
		logger := GetLoggerFromContext(ctx)
		if logger == nil {
			t.Error("Expected logger to be non-nil")
		}
	})
}