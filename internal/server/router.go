package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/backend/orchestrator"

	"github.com/go-chi/chi/v5"
)

// Router wraps a chi router with service dependencies for handling API requests.
type Router struct {
	router *chi.Mux
	svc    *orchestrator.Service
}

type contextKey string

const (
	userContextKey    contextKey = "user"
	serviceContextKey contextKey = "service"
)

// NewRouter creates a new chi router with routes configured.
// If requestTimeout is > 0, adds a per-request timeout middleware.
// If requestTimeout is 0, no timeout middleware is added, allowing the
// environment (e.g., Lambda with its own timeout) to handle timeouts.
func NewRouter(
	svc *orchestrator.Service,
	requestTimeout time.Duration,
	allowedOrigins []string,
) *Router {
	r := chi.NewRouter()
	router := &Router{
		router: r,
		svc:    svc,
	}

	if requestTimeout > 0 {
		r.Use(router.requestTimeoutMiddleware(requestTimeout))
	}
	r.Use(corsMiddleware(allowedOrigins))
	r.Use(setContentTypeJSONMiddleware)
	r.Use(router.requestIDMiddleware)
	r.Use(router.requestLoggingMiddleware)

	r.Route("/api/v1", func(r chi.Router) {
		router.registerPublicRoutes(r)
		router.registerAuthenticatedRoutes(r)
	})

	return router
}

// responseWriter is a wrapper around http.ResponseWriter to capture status code.
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

	n, err := rw.ResponseWriter.Write(b)
	if err != nil {
		return n, fmt.Errorf("failed to write response: %w", err)
	}
	return n, nil
}

// ServeHTTP implements http.Handler for use with chi router.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

// ChiMux returns the underlying chi router for advanced usage.
func (r *Router) ChiMux() *chi.Mux {
	return r.router
}

// Handler returns an http.Handler for the router.
func (r *Router) Handler() http.Handler {
	return r.router
}

// WithContext adds the service to the request context.
func (r *Router) WithContext(ctx context.Context, svc *orchestrator.Service) context.Context {
	return context.WithValue(ctx, serviceContextKey, svc)
}

// writeErrorResponse is a helper to write consistent error responses.
func writeErrorResponse(w http.ResponseWriter, statusCode int, message, details string) {
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(api.ErrorResponse{
		Error:   message,
		Details: details,
	})
}

// writeErrorResponseWithCode is a helper to write error responses with error codes.
func writeErrorResponseWithCode(w http.ResponseWriter, statusCode int, code, message, details string) {
	w.WriteHeader(statusCode)
	resp := api.ErrorResponse{
		Error:   message,
		Details: details,
	}
	if code != "" {
		resp.Code = code
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// registerPublicRoutes registers public routes that don't require authentication.
func (r *Router) registerPublicRoutes(router chi.Router) {
	router.Get("/claim/{token}", r.handleClaimAPIKey)
	router.Get("/health", r.handleHealth)
}

// registerAuthenticatedRoutes registers routes that require authentication and authorization.
func (r *Router) registerAuthenticatedRoutes(router chi.Router) {
	authMiddleware := router.With(
		r.authenticateRequestMiddleware,
		r.authorizeRequestMiddleware,
	)

	authMiddleware.Post("/health/reconcile", r.handleReconcileHealth)
	authMiddleware.Post("/run", r.handleRunCommand)

	r.registerUsersRoutes(authMiddleware)
	r.registerImagesRoutes(authMiddleware)
	r.registerSecretsRoutes(authMiddleware)
	r.registerExecutionsRoutes(authMiddleware)
	r.registerBackendLogsTraceRoutes(authMiddleware)
}

// registerUsersRoutes registers user management routes.
func (r *Router) registerUsersRoutes(router chi.Router) {
	router.Route("/users", func(route chi.Router) {
		route.Get("/", r.handleListUsers)
		route.Post("/create", r.handleCreateUser)
		route.Post("/revoke", r.handleRevokeUser)
	})
}

// registerImagesRoutes registers image management routes.
func (r *Router) registerImagesRoutes(router chi.Router) {
	router.Route("/images", func(route chi.Router) {
		route.Post("/register", r.handleRegisterImage)
		route.Get("/", r.handleListImages)
		route.Get("/*", r.handleGetImage)
		route.Delete("/*", r.handleRemoveImage)
	})
}

// registerSecretsRoutes registers secret management routes.
func (r *Router) registerSecretsRoutes(router chi.Router) {
	router.Route("/secrets", func(route chi.Router) {
		route.Get("/", r.handleListSecrets)
		route.Post("/", r.handleCreateSecret)
		route.Get("/{name}", r.handleGetSecret)
		route.Put("/{name}", r.handleUpdateSecret)
		route.Delete("/{name}", r.handleDeleteSecret)
	})
}

// registerExecutionsRoutes registers execution management routes.
func (r *Router) registerExecutionsRoutes(router chi.Router) {
	router.Route("/executions", func(route chi.Router) {
		route.Get("/", r.handleListExecutions)
		route.Get("/{executionID}/logs", r.handleGetExecutionLogs)
		route.Get("/{executionID}/status", r.handleGetExecutionStatus)
		route.Delete("/{executionID}", r.handleKillExecution)
	})
}

// registerBackendLogsTraceRoutes registers backend log tracing routes.
func (r *Router) registerBackendLogsTraceRoutes(router chi.Router) {
	router.Route("/trace", func(route chi.Router) {
		route.Get("/{requestID}", r.handleGetBackendLogsTrace)
	})
}
