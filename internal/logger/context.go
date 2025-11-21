// Package logger provides structured logging utilities for runvoy.
// It includes context-aware logging and log level management.
package logger

import (
	"context"
	"log/slog"
	"time"

	"runvoy/internal/constants"
)

type contextKey string

const (
	requestIDContextKey contextKey = "requestID"
)

// ContextExtractor defines an interface for extracting request IDs from various
// context sources (e.g., AWS Lambda, HTTP servers, other cloud providers).
// This interface allows the logger to remain portable and not bound to any
// specific provider implementation.
type ContextExtractor interface {
	// ExtractRequestID attempts to extract a request ID from the given context.
	// Returns the request ID and true if found, empty string and false otherwise.
	ExtractRequestID(ctx context.Context) (requestID string, found bool)
}

// contextExtractors holds the registered context extractors.
// Extractors are tried in order until one successfully extracts a request ID.
var contextExtractors []ContextExtractor

// RegisterContextExtractor registers a new context extractor.
// Extractors are called in the order they are registered.
func RegisterContextExtractor(extractor ContextExtractor) {
	contextExtractors = append(contextExtractors, extractor)
}

// ClearContextExtractors removes all registered context extractors.
// This is primarily useful for testing.
func ClearContextExtractors() {
	contextExtractors = nil
}

// WithRequestID returns a new context with the given request ID attached.
// This should be used by server middleware to add request IDs to the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

// GetRequestID extracts the request ID from the context.
// The request ID is set by server middleware when available.
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDContextKey).(string); ok {
		return requestID
	}

	return ""
}

// ExtractRequestIDFromContext attempts to extract a request ID from the context
// using the same priority as DeriveRequestLogger: first checks for a request ID
// set via WithRequestID, then tries all registered ContextExtractors in order.
// Returns the first request ID found, or an empty string if none is found.
// This allows middleware to extract request IDs without knowing about provider-specific
// implementations (e.g., AWS Lambda).
func ExtractRequestIDFromContext(ctx context.Context) string {
	if requestID := GetRequestID(ctx); requestID != "" {
		return requestID
	}

	for _, extractor := range contextExtractors {
		if requestID, found := extractor.ExtractRequestID(ctx); found && requestID != "" {
			return requestID
		}
	}

	return ""
}

// DeriveRequestLogger returns a logger enriched with request-scoped fields
// available in the provided context. It first checks for a request ID set via
// WithRequestID, then tries all registered ContextExtractors in order.
// This allows supporting multiple providers (AWS Lambda, HTTP servers, etc.)
// without hardcoding provider-specific logic.
//
// IMPORTANT: The requestID field is always added at the root level of the log entry
// (never nested in a "context" object). This ensures we can  query logs by requestID
// directly using e.g. filter requestID = "value".
// Do NOT add requestID to any "context" map objects in log calls.
func DeriveRequestLogger(ctx context.Context, base *slog.Logger) *slog.Logger {
	if base == nil {
		return slog.Default()
	}

	if requestID := GetRequestID(ctx); requestID != "" {
		return base.With(constants.RequestIDLogField, requestID)
	}

	for _, extractor := range contextExtractors {
		if requestID, found := extractor.ExtractRequestID(ctx); found && requestID != "" {
			return base.With(constants.RequestIDLogField, requestID)
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
