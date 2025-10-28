package logger

import (
	"log/slog"
	"os"

	"runvoy/internal/constants"
)

// Initialize sets up the global slog logger based on the environment
// env: "development" for human-readable logs, "production" for JSON logs
func Initialize(env constants.Environment, level slog.Level) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: level,
	}

	if env == constants.Production {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		// Human-readable format for development
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// New creates a new logger instance without setting it as the default
func New(env constants.Environment, level slog.Level) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: level,
	}

	if env == constants.Production {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
