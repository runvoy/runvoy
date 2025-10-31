package logger

import (
	"log/slog"
	"os"

	"runvoy/internal/constants"

	"github.com/lmittmann/tint"
)

// Initialize sets up the global slog logger based on the environment
func Initialize(env constants.Environment, level slog.Level) *slog.Logger {
	var handler slog.Handler

	if env == constants.Production {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		handler = tint.NewHandler(os.Stderr, &tint.Options{
			Level: level,
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	slog.Debug("logger initialized", "env", env, "level", level)

	return logger
}
