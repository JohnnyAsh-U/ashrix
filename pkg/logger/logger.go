package logger

import (
	"log/slog"
	"os"
)

// New returns a structured logger.
// JSON handler in production (for log aggregators).
// Text handler in development (human-readable).
func New(env string) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		// Development: include debug logs, text format
		opts.Level = slog.LevelDebug
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
