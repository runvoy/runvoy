package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/app"

	"github.com/go-chi/chi/v5"
)

type contextKey string

const (
	userContextKey    contextKey = "user"
	serviceContextKey contextKey = "service"
)

type Router struct {
	router *chi.Mux
	svc    *app.Service
}

// NewRouter creates a new chi router with routes configured
func NewRouter(svc *app.Service) *Router {
	r := chi.NewRouter()
	router := &Router{
		router: r,
		svc:    svc,
	}

	r.Use(setContentTypeJSON)
	r.Use(requestIDMiddleware)
	r.Use(router.requestLoggingMiddleware)

	r.Route("/api/v1", func(r chi.Router) {
		// public routes
		r.Get("/health", router.handleHealth)

		// authenticated routes
		r.Use(router.authenticateRequest)
		r.Post("/users/create", router.handleCreateUser)
		r.Post("/users/revoke", router.handleRevokeUser)
	})
	return router
}

// setContentTypeJSON middleware sets Content-Type to application/json for all responses
func setContentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, req)
	})
}

// authenticateRequest middleware authenticates requests
func (r *Router) authenticateRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Use service logger with request ID if available
		logger := r.svc.Logger
		if requestID := GetRequestID(req.Context()); requestID != "" {
			logger = logger.With("requestID", requestID)
		}

		apiKey := req.Header.Get("X-API-Key")
		logger.Debug("authenticating request") // removed logging of apiKey (security)

		if apiKey == "" {
			writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "API key is required")
			return
		}

		user, err := r.svc.AuthenticateUser(req.Context(), apiKey)
		if err != nil {
			writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "Invalid API key")
			return
		}

		logger.Info("user authenticated successfully", "user", user)

		// Add authenticated user to request context
		ctx := context.WithValue(req.Context(), userContextKey, user)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

// responseWriter is a wrapper around http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.statusCode = http.StatusOK
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

// requestLoggingMiddleware logs incoming requests and their responses
func (r *Router) requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Use service logger with request ID if available
		logger := r.svc.Logger
		if requestID := GetRequestID(req.Context()); requestID != "" {
			logger = logger.With("requestID", requestID)
		}

		start := time.Now()

		// Wrap the response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // default status code
		}

		// Log incoming request
		logger.Info("Incoming request",
			"method", req.Method,
			"path", req.URL.Path,
			"remoteAddr", req.RemoteAddr,
		)

		// Call the next handler
		next.ServeHTTP(wrapped, req)

		// Calculate duration
		duration := time.Since(start)

		// Log response
		logger.Info("Request completed",
			"method", req.Method,
			"path", req.URL.Path,
			"status", wrapped.statusCode,
			"duration", duration,
		)
	})
}

// handleHealth returns a simple health check response
func (r *Router) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// ServeHTTP implements http.Handler for use with chi router
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

// ChiMux returns the underlying chi router for advanced usage
func (r *Router) ChiMux() *chi.Mux {
	return r.router
}

// Handler returns an http.Handler for the router
func (r *Router) Handler() http.Handler {
	return r.router
}

// WithContext adds the service to the request context
func (r *Router) WithContext(ctx context.Context, svc *app.Service) context.Context {
	return context.WithValue(ctx, serviceContextKey, svc)
}

// handleCreateUser handles POST /api/v1/users to create a new user with an API key
func (r *Router) handleCreateUser(w http.ResponseWriter, req *http.Request) {
	// Use service logger with request ID if available
	logger := r.svc.Logger
	if requestID := GetRequestID(req.Context()); requestID != "" {
		logger = logger.With("requestID", requestID)
	}

	var createReq api.CreateUserRequest

	// Decode JSON request body
	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Call service to create user
	resp, err := r.svc.CreateUser(req.Context(), createReq)
	if err != nil {
		// Determine appropriate status code based on error
		statusCode := http.StatusInternalServerError
		if err.Error() == "email is required" ||
			err.Error() == "user with this email already exists" ||
			containsString(err.Error(), "invalid email address") {
			statusCode = http.StatusBadRequest
		}
		if err.Error() == "user with this email already exists" {
			statusCode = http.StatusConflict
		}

		logger.Debug("Failed to create user", "error", err)
		writeErrorResponse(w, statusCode, "Failed to create user", err.Error())
		return
	}

	// Return successful response with the created user and API key
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// handleRevokeUser handles POST /api/v1/users/revoke to revoke a user's API key
func (r *Router) handleRevokeUser(w http.ResponseWriter, req *http.Request) {
	// Use service logger with request ID if available
	logger := r.svc.Logger
	if requestID := GetRequestID(req.Context()); requestID != "" {
		logger = logger.With("requestID", requestID)
	}

	var revokeReq api.RevokeUserRequest

	// Decode JSON request body
	if err := json.NewDecoder(req.Body).Decode(&revokeReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Call service to revoke user
	if err := r.svc.RevokeUser(req.Context(), revokeReq.Email); err != nil {
		// Determine appropriate status code based on error
		statusCode := http.StatusInternalServerError
		if err.Error() == "email is required" {
			statusCode = http.StatusBadRequest
		}
		if err.Error() == "user not found" {
			statusCode = http.StatusNotFound
		}

		logger.Debug("Failed to revoke user", "error", err)
		writeErrorResponse(w, statusCode, "Failed to revoke user", err.Error())
		return
	}

	// Return successful response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User API key revoked successfully",
		"email":   revokeReq.Email,
	})
}

// writeErrorResponse is a helper to write consistent error responses
func writeErrorResponse(w http.ResponseWriter, statusCode int, message string, details string) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(api.ErrorResponse{
		Error:   message,
		Details: details,
	})
}

// containsString checks if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

// findSubstring is a simple substring finder
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
