package logger

import (
	"log/slog"
	"os"

	"runvoy/internal/constants"
)

// Initialize sets up the global slog logger based on the environment
func Initialize(env constants.Environment, level slog.Level) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: level,
	}

	if env == constants.Production {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		// Use colored handler for local/development environments
		handler = NewColorHandler(os.Stderr, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	slog.Debug("logger initialized", "env", env, "level", level)

	return logger
}
