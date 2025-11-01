package logger

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"runvoy/internal/constants"

	"github.com/lmittmann/tint"
)

// flattenMapAttr flattens nested maps into a readable key=value format
// Example: map[deadline:none status:SUCCEEDED] becomes "deadline=none status=SUCCEEDED"
func flattenMapAttr(prefix string, value any) string {
	var parts []string

	switch v := value.(type) {
	case map[string]any:
		// Sort keys for consistent output
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			val := v[key]
			// Build the full key path
			fullKey := key
			if prefix != "" {
				fullKey = prefix + "." + key
			}

			// Handle nested maps recursively
			if nestedMap, ok := val.(map[string]any); ok {
				// Recursively flatten with the full key path as new prefix
				nested := flattenMapAttr(fullKey, nestedMap)
				parts = append(parts, nested)
			} else if nestedMap, ok := val.(map[string]string); ok {
				// Recursively flatten with the full key path as new prefix
				nested := flattenMapAttr(fullKey, nestedMap)
				parts = append(parts, nested)
			} else {
				// Leaf value - format as key=value
				parts = append(parts, fmt.Sprintf("%s=%v", fullKey, val))
			}
		}
	case map[string]string:
		// Sort keys for consistent output
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			fullKey := key
			if prefix != "" {
				fullKey = prefix + "." + key
			}
			parts = append(parts, fmt.Sprintf("%s=%v", fullKey, v[key]))
		}
	default:
		// Not a map, return as-is
		return fmt.Sprintf("%v", value)
	}

	return strings.Join(parts, " ")
}

// replaceAttrForDev transforms attributes for better readability in dev environment
func replaceAttrForDev(groups []string, a slog.Attr) slog.Attr {
	// Flatten map attributes for better readability
	switch v := a.Value.Any().(type) {
	case map[string]any, map[string]string:
		flattened := flattenMapAttr(a.Key, v)
		return slog.String(a.Key, flattened)
	}
	return a
}

// Initialize sets up the global slog logger based on the environment
func Initialize(env constants.Environment, level slog.Level) *slog.Logger {
	var handler slog.Handler

	if env == constants.Production {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		handler = tint.NewHandler(os.Stderr, &tint.Options{
			Level:       level,
			ReplaceAttr: replaceAttrForDev,
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	slog.Debug("logger initialized", "env", env, "level", level)

	return logger
}
