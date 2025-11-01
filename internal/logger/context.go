// Package logger provides structured logging utilities for runvoy.
// It includes context-aware logging and log level management.
package logger

import (
	"context"
	"log/slog"
	"time"

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

	// Try to get requestID from context value first (used in HTTP server)
	if requestID := GetRequestID(ctx); requestID != "" {
		return base.With("requestID", requestID)
	}

	// Fall back to AWS Lambda request ID (used in Lambda functions)
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		if lc.AwsRequestID != "" {
			return base.With("requestID", lc.AwsRequestID)
		}
	}

	return base
}

// GetDeadlineInfo returns logging attributes for context deadline information.
// Returns the absolute deadline time and remaining duration if set, or "none" if no deadline.
func GetDeadlineInfo(ctx context.Context) []any {
	deadline, ok := ctx.Deadline()
	if !ok {
		return []any{"deadline", "none", "deadline_remaining", "none"}
	}

	remaining := time.Until(deadline)
	return []any{
		"deadline", deadline.Format(time.RFC3339),
		"deadline_remaining", remaining.String(),
	}
}

// SliceToMap converts a slice of alternating key-value pairs to a map[string]any.
// It expects the slice to have an even number of elements with string keys.
// Non-string keys are skipped.
func SliceToMap(args []any) map[string]any {
	argsMap := make(map[string]any)
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			if key, ok := args[i].(string); ok {
				argsMap[key] = args[i+1]
			}
		}
	}
	return argsMap
}
