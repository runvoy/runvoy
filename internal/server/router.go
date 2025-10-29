package server

import (
	"context"
	"encoding/json"
	"net/http"

	"runvoy/internal/api"
	"runvoy/internal/app"

	"github.com/go-chi/chi/v5"
)

type Router struct {
	router *chi.Mux
	svc    *app.Service
}

type contextKey string

const (
	userContextKey    contextKey = "user"
	serviceContextKey contextKey = "service"
)

// NewRouter creates a new chi router with routes configured
func NewRouter(svc *app.Service) *Router {
	r := chi.NewRouter()
	router := &Router{
		router: r,
		svc:    svc,
	}

	r.Use(setContentTypeJSONMiddleware)
	r.Use(requestIDMiddleware)
	r.Use(router.requestLoggingMiddleware)

	r.Route("/api/v1", func(r chi.Router) {
		// public routes
		r.Get("/health", router.handleHealth)

		// authenticated routes
		r.With(router.authenticateRequestMiddleware).Route("/users", func(r chi.Router) {
			r.Post("/create", router.handleCreateUser)
			r.Post("/revoke", router.handleRevokeUser)
		})
	})

	return router
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
