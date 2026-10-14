package telemetry

import (
	"context"
	"log/slog"
)

// AddSlogHandler returns a logger that writes to both its existing handler and the provided handler.
func AddSlogHandler(logger *slog.Logger, handler slog.Handler) *slog.Logger {
	if logger == nil || handler == nil {
		return logger
	}
	return slog.New(&multiSlogHandler{handlers: []slog.Handler{logger.Handler(), handler}})
}

type multiSlogHandler struct {
	handlers []slog.Handler
}

func (h *multiSlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiSlogHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, record.Level) {
			if err := handler.Handle(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *multiSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for index, handler := range h.handlers {
		handlers[index] = handler.WithAttrs(attrs)
	}
	return &multiSlogHandler{handlers: handlers}
}

func (h *multiSlogHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for index, handler := range h.handlers {
		handlers[index] = handler.WithGroup(name)
	}
	return &multiSlogHandler{handlers: handlers}
}
