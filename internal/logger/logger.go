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
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		// Human-readable format for development
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	slog.Debug("logger initialized", "env", env, "level", level)
	return logger
}
