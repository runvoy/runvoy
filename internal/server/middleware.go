package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

const (
	requestIDContextKey contextKey = "requestID"
)

// requestIDMiddleware extracts the Lambda request ID from the context and adds it to the request context
// This middleware should be added early in the middleware chain to ensure request ID is available for logging
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Extract Lambda request ID from the context if available
		requestID := ""
		if lc, ok := lambdacontext.FromContext(req.Context()); ok {
			requestID = lc.AwsRequestID
		}

		// Add request ID to the request context
		ctx := context.WithValue(req.Context(), requestIDContextKey, requestID)
		
		// Update the logger to include request ID in all subsequent log messages for this request
		if requestID != "" {
			// Create a logger with request ID for this request
			logger := slog.With("requestID", requestID)
			// Store the logger in context for use by handlers
			ctx = context.WithValue(ctx, "logger", logger)
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

// GetLoggerFromContext extracts a logger with request ID from the context, or returns the default logger
func GetLoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value("logger").(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}