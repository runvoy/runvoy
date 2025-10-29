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
// Sets up a request-scoped logger in context that includes the request ID if available
func (r *Router) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestID := ""
		if lc, ok := lambdacontext.FromContext(req.Context()); ok {
			requestID = lc.AwsRequestID
		}

		ctx := context.WithValue(req.Context(), requestIDContextKey, requestID)

		logger := r.svc.Logger
		if requestID != "" {
			logger = logger.With(string(requestIDContextKey), requestID)
		}
		ctx = context.WithValue(ctx, loggerContextKey, logger)

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

// requestTimeoutMiddleware creates a context with timeout for each request.
// The timeout starts when the request is received, ensuring each request has
// a fair timeout regardless of connection reuse.
func (r *Router) requestTimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx, cancel := context.WithTimeout(req.Context(), timeout)
			defer cancel()

			req = req.WithContext(ctx)

			next.ServeHTTP(w, req)

			if ctx.Err() == context.DeadlineExceeded {
				logger := r.GetLoggerFromContext(req.Context())
				logger.Warn("request timeout exceeded",
					"method", req.Method,
					"path", req.URL.Path,
					"timeout", timeout,
				)

				// Note: Response may have already been written by handler
				// The context cancellation will have already propagated to
				// any operations (like DynamoDB calls) that were using the context
			}
		})
	}
}

// setContentTypeJSONMiddleware sets Content-Type to application/json for all responses
func setContentTypeJSONMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set(constants.ContentTypeHeader, "application/json")
		next.ServeHTTP(w, req)
	})
}

// authenticateRequestMiddleware authenticates requests
// Adds authenticated user to request context
func (r *Router) authenticateRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logger := r.GetLoggerFromContext(req.Context())
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
// Uses logger from context (includes request ID if available)
func (r *Router) requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logger := r.GetLoggerFromContext(req.Context())
		start := time.Now()
		deadlineString := ""
		if deadline, ok := req.Context().Deadline(); ok {
			deadlineString = deadline.Format(time.RFC3339)
		}

		// Wrap the response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // default status code
		}

		logger.Info("Incoming request",
			"method", req.Method,
			"path", req.URL.Path,
			"remoteAddr", req.RemoteAddr,
			"deadline", deadlineString,
		)

		next.ServeHTTP(wrapped, req)
		duration := time.Since(start)

		logger.Info("Request completed",
			"method", req.Method,
			"path", req.URL.Path,
			"status", wrapped.statusCode,
			"duration", duration.String(),
		)
	})
}

// GetLoggerFromContext extracts the logger from request context
// Returns the request-scoped logger (with request ID if available) or falls back to service logger
func (r *Router) GetLoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerContextKey).(*slog.Logger); ok && logger != nil {
		return logger
	}

	return r.svc.Logger
}
