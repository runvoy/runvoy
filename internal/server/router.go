package server

import (
	"context"
	"encoding/json"
	"net/http"

	"runvoy/internal/app"

	"github.com/go-chi/chi/v5"
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

	// Add middleware to set Content-Type header for all routes
	r.Use(setContentTypeJSON)

	// Set up routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", router.handleHealth)
		r.Get("/greet/{name}", router.handleGreet)
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

// handleHealth returns a simple health check response
func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// handleGreet handles greeting messages
func (r *Router) handleGreet(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	message := r.svc.Greet(name)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": message,
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
	return context.WithValue(ctx, "service", svc)
}
