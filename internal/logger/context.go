package logger

import (
    "context"
    "log/slog"

    "github.com/aws/aws-lambda-go/lambdacontext"
)

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


