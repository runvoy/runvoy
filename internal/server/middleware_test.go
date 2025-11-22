package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"runvoy/internal/backend/orchestrator"
	"runvoy/internal/constants"
	"runvoy/internal/database"
	"runvoy/internal/logger"
	awsOrchestrator "runvoy/internal/providers/aws/orchestrator"
	"runvoy/internal/testutil"

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

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		rr := httptest.NewRecorder()

		tokenRepo := &testTokenRepository{}

		repos := database.Repositories{
			User:       &testUserRepository{},
			Execution:  &testExecutionRepository{},
			Connection: nil,
			Token:      tokenRepo,
			Image:      &testImageRepository{},
			Secrets:    &testSecretsRepository{},
		}
		svc, err := orchestrator.NewService(
			context.Background(),
			&repos,
			&testRunner{}, // TaskManager
			&testRunner{}, // ImageRegistry
			&testRunner{}, // LogManager
			&testRunner{}, // ObservabilityManager
			testutil.SilentLogger(),
			constants.AWS,
			nil,
			nil,
			newPermissiveTestEnforcerForHandlers(t),
		)
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}
		router := &Router{svc: svc}
		middleware := router.requestIDMiddleware(handler)
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	// Test with Lambda context - should use Lambda's request ID
	t.Run("with lambda context uses lambda request ID", func(t *testing.T) {
		// Register Lambda context extractor for this test
		logger.RegisterContextExtractor(awsOrchestrator.NewLambdaContextExtractor())
		defer logger.ClearContextExtractors()

		lambdaHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := logger.GetRequestID(r.Context())
			if requestID != "test-request-id-123" {
				t.Errorf("Expected request ID 'test-request-id-123', got '%s'", requestID)
			}
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		rr := httptest.NewRecorder()

		// Add Lambda context with request ID
		lc := &lambdacontext.LambdaContext{
			AwsRequestID: "test-request-id-123",
		}
		ctx := lambdacontext.NewContext(req.Context(), lc)
		req = req.WithContext(ctx)

		tokenRepo := &testTokenRepository{}

		repos := database.Repositories{
			User:       &testUserRepository{},
			Execution:  &testExecutionRepository{},
			Connection: nil,
			Token:      tokenRepo,
			Image:      &testImageRepository{},
			Secrets:    &testSecretsRepository{},
		}
		svc, err := orchestrator.NewService(
			context.Background(),
			&repos,
			&testRunner{}, // TaskManager
			&testRunner{}, // ImageRegistry
			&testRunner{}, // LogManager
			&testRunner{}, // ObservabilityManager
			testutil.SilentLogger(),
			constants.AWS,
			nil,
			nil,
			newPermissiveTestEnforcerForHandlers(t),
		)
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}
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

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		rr := httptest.NewRecorder()

		// Add existing request ID to context
		ctx := logger.WithRequestID(req.Context(), "existing-request-id-456")
		req = req.WithContext(ctx)

		tokenRepo := &testTokenRepository{}

		repos := database.Repositories{
			User:       &testUserRepository{},
			Execution:  &testExecutionRepository{},
			Connection: nil,
			Token:      tokenRepo,
			Image:      &testImageRepository{},
			Secrets:    &testSecretsRepository{},
		}
		svc, err := orchestrator.NewService(
			context.Background(),
			&repos,
			&testRunner{}, // TaskManager
			&testRunner{}, // ImageRegistry
			&testRunner{}, // LogManager
			&testRunner{}, // ObservabilityManager
			testutil.SilentLogger(),
			constants.AWS,
			nil,
			nil,
			newPermissiveTestEnforcerForHandlers(t),
		)
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}
		router := &Router{svc: svc}
		middleware := router.requestIDMiddleware(handler)
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	// Test priority: existing context ID should take precedence over Lambda ID
	t.Run("existing context ID takes precedence over lambda ID", func(t *testing.T) {
		// Register Lambda context extractor for this test
		logger.RegisterContextExtractor(awsOrchestrator.NewLambdaContextExtractor())
		defer logger.ClearContextExtractors()

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := logger.GetRequestID(r.Context())
			if requestID != "context-priority-id" {
				t.Errorf("Expected request ID 'context-priority-id', got '%s'", requestID)
			}
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		rr := httptest.NewRecorder()

		// Add both existing request ID and Lambda context
		ctx := logger.WithRequestID(req.Context(), "context-priority-id")
		lc := &lambdacontext.LambdaContext{
			AwsRequestID: "lambda-request-id-should-be-ignored",
		}
		ctx = lambdacontext.NewContext(ctx, lc)
		req = req.WithContext(ctx)

		tokenRepo := &testTokenRepository{}

		repos := database.Repositories{
			User:       &testUserRepository{},
			Execution:  &testExecutionRepository{},
			Connection: nil,
			Token:      tokenRepo,
			Image:      &testImageRepository{},
			Secrets:    &testSecretsRepository{},
		}
		svc, err := orchestrator.NewService(
			context.Background(),
			&repos,
			&testRunner{}, // TaskManager
			&testRunner{}, // ImageRegistry
			&testRunner{}, // LogManager
			&testRunner{}, // ObservabilityManager
			testutil.SilentLogger(),
			constants.AWS,
			nil,
			nil,
			newPermissiveTestEnforcerForHandlers(t),
		)
		if err != nil {
			t.Fatalf("failed to create service: %v", err)
		}
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

func TestCorsMiddleware(t *testing.T) {
	tokenRepo := &testTokenRepository{}

	runner := &testRunner{}
	repos := database.Repositories{
		User:       &testUserRepository{},
		Execution:  &testExecutionRepository{},
		Connection: nil,
		Token:      tokenRepo,
		Image:      &testImageRepository{},
		Secrets:    &testSecretsRepository{},
	}
	svc, err := orchestrator.NewService(
		context.Background(),
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		testutil.SilentLogger(),
		constants.AWS,
		nil,
		nil,
		newPermissiveTestEnforcerForHandlers(t),
	)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	t.Run("allows origin without trailing slash when configured with trailing slash", func(t *testing.T) {
		allowedOrigins := []string{"https://runvoy.site/"}
		router := NewRouter(svc, 0, allowedOrigins)

		req := httptest.NewRequest("GET", "/api/v1/health", http.NoBody)
		req.Header.Set("Origin", "https://runvoy.site")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
		accessControlOrigin := rr.Header().Get("Access-Control-Allow-Origin")
		if accessControlOrigin != "https://runvoy.site" {
			t.Errorf("Expected Access-Control-Allow-Origin 'https://runvoy.site', got '%s'", accessControlOrigin)
		}
	})

	t.Run("allows origin with trailing slash when configured without trailing slash", func(t *testing.T) {
		allowedOrigins := []string{"https://runvoy.site"}
		router := NewRouter(svc, 0, allowedOrigins)

		req := httptest.NewRequest("GET", "/api/v1/health", http.NoBody)
		req.Header.Set("Origin", "https://runvoy.site/")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
		accessControlOrigin := rr.Header().Get("Access-Control-Allow-Origin")
		if accessControlOrigin != "https://runvoy.site/" {
			t.Errorf("Expected Access-Control-Allow-Origin 'https://runvoy.site/', got '%s'", accessControlOrigin)
		}
	})

	t.Run("rejects origin not in allowed list", func(t *testing.T) {
		allowedOrigins := []string{"https://runvoy.site"}
		router := NewRouter(svc, 0, allowedOrigins)

		req := httptest.NewRequest("GET", "/api/v1/health", http.NoBody)
		req.Header.Set("Origin", "https://evil.com")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
		accessControlOrigin := rr.Header().Get("Access-Control-Allow-Origin")
		if accessControlOrigin != "" {
			t.Errorf("Expected empty Access-Control-Allow-Origin, got '%s'", accessControlOrigin)
		}
	})

	t.Run("handles OPTIONS preflight request", func(t *testing.T) {
		allowedOrigins := []string{"https://runvoy.site"}
		router := NewRouter(svc, 0, allowedOrigins)

		req := httptest.NewRequest("OPTIONS", "/api/v1/health", http.NoBody)
		req.Header.Set("Origin", "https://runvoy.site")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
		accessControlOrigin := rr.Header().Get("Access-Control-Allow-Origin")
		if accessControlOrigin != "https://runvoy.site" {
			t.Errorf("Expected Access-Control-Allow-Origin 'https://runvoy.site', got '%s'", accessControlOrigin)
		}
		accessControlMethods := rr.Header().Get("Access-Control-Allow-Methods")
		if accessControlMethods == "" {
			t.Error("Expected Access-Control-Allow-Methods header to be set")
		}
	})

	t.Run("allows all origins when wildcard is configured", func(t *testing.T) {
		allowedOrigins := []string{"*"}
		router := NewRouter(svc, 0, allowedOrigins)

		testOrigin := "https://6921dbf211316ed7d40a7984--tranquil-toffee-07b637.netlify.app"
		req := httptest.NewRequest("GET", "/api/v1/health", http.NoBody)
		req.Header.Set("Origin", testOrigin)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
		accessControlOrigin := rr.Header().Get("Access-Control-Allow-Origin")
		if accessControlOrigin != testOrigin {
			t.Errorf("Expected Access-Control-Allow-Origin '%s', got '%s'", testOrigin, accessControlOrigin)
		}
	})

	t.Run("allows all origins when wildcard is in list with other origins", func(t *testing.T) {
		allowedOrigins := []string{"https://runvoy.site", "*"}
		router := NewRouter(svc, 0, allowedOrigins)

		req := httptest.NewRequest("GET", "/api/v1/health", http.NoBody)
		req.Header.Set("Origin", "https://any-origin.com")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
		accessControlOrigin := rr.Header().Get("Access-Control-Allow-Origin")
		if accessControlOrigin != "https://any-origin.com" {
			t.Errorf("Expected Access-Control-Allow-Origin 'https://any-origin.com', got '%s'", accessControlOrigin)
		}
	})
}
