package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"runvoy/internal/constants"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

const (
	requestIDContextKey contextKey = "requestID"
	loggerContextKey    contextKey = "logger"
)

// requestIDMiddleware extracts the Lambda request ID from the context and adds it to the request context
// This middleware should be added early in the middleware chain to ensure request ID is available for logging
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestID := ""
		if lc, ok := lambdacontext.FromContext(req.Context()); ok {
			requestID = lc.AwsRequestID
		}

		ctx := context.WithValue(req.Context(), requestIDContextKey, requestID)

		if requestID != "" {
			logger := slog.With("requestID", requestID)
			ctx = context.WithValue(ctx, loggerContextKey, logger)
		}

		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from the context
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDContextKey).(string); ok {
		return requestID
	}

	return ""
}

// setContentTypeJSONMiddleware sets Content-Type to application/json for all responses
func setContentTypeJSONMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set(constants.ContentTypeHeader, "application/json")
		next.ServeHTTP(w, req)
	})
}

// authenticateRequestMiddleware authenticates requests
// NOTICE: adds authenticated user to request context
// NOTICE: uses service logger with request ID if available
func (r *Router) authenticateRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logger := getLoggerWithRequestID(r, req.Context())
		apiKey := req.Header.Get(constants.ApiKeyHeader)
		logger.Debug("authenticating request")

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

		ctx := context.WithValue(req.Context(), userContextKey, user)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

// requestLoggingMiddleware logs incoming requests and their responses
// Uses service logger with request ID if available
func (r *Router) requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logger := getLoggerWithRequestID(r, req.Context())
		start := time.Now()

		// Wrap the response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // default status code
		}

		logger.Info("Incoming request",
			"method", req.Method,
			"path", req.URL.Path,
			"remoteAddr", req.RemoteAddr,
		)

		next.ServeHTTP(wrapped, req)
		duration := time.Since(start)

		logger.Info("Request completed",
			"method", req.Method,
			"path", req.URL.Path,
			"status", wrapped.statusCode,
			"duration", duration,
		)
	})
}

// getLoggerWithRequestID returns a logger with the request ID if available
// falls back to the service logger if no request ID is available
func getLoggerWithRequestID(r *Router, ctx context.Context) *slog.Logger {
	logger := r.svc.Logger
	if requestID := GetRequestID(ctx); requestID != "" {
		logger = logger.With(string(requestIDContextKey), requestID)
	}

	return logger
}
