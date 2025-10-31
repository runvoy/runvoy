// Package logger provides structured logging utilities for runvoy.
// It includes context-aware logging and log level management.
package logger

import (
	"context"
	"log/slog"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

type contextKey string

const (
	requestIDContextKey contextKey = "requestID"
)

// GetRequestID extracts the request ID from the context.
// The request ID is set by server middleware when available.
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDContextKey).(string); ok {
		return requestID
	}

	return ""
}

// RequestIDContextKey returns the context key used for storing request IDs.
// This allows other packages (like server) to set request IDs in context.
func RequestIDContextKey() contextKey {
	return requestIDContextKey
}

// DeriveRequestLogger returns a logger enriched with request-scoped fields
// available in the provided context. Today it extracts AWS Lambda request ID
// when present. In the future, additional providers can be added here without
// changing call sites across the codebase.
func DeriveRequestLogger(ctx context.Context, base *slog.Logger) *slog.Logger {
	if base == nil {
		return slog.Default()
	}

	// AWS Lambda request ID
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		if lc.AwsRequestID != "" {
			return base.With("requestID", lc.AwsRequestID)
		}
	}

	return base
}
