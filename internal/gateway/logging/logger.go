package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	App   *slog.Logger
)

type Config struct {
	Env        string
	LogDir     string
	Level      string
	MaxSizeMB  int
	MaxAgeDays int
	MaxBackups int
	Compress   bool
}

// ElegantConsoleHandler is a custom slog.Handler that outputs colored, readable log entries.
type ElegantConsoleHandler struct {
	w     io.Writer
	opts  slog.HandlerOptions
	attrs []slog.Attr
}

func NewElegantConsoleHandler(w io.Writer, opts slog.HandlerOptions) *ElegantConsoleHandler {
	return &ElegantConsoleHandler{w: w, opts: opts}
}

func (h *ElegantConsoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *ElegantConsoleHandler) Handle(ctx context.Context, r slog.Record) error {
	// Color codes
	var levelColor string
	switch r.Level {
	case slog.LevelDebug:
		levelColor = "\033[36m" // Cyan
	case slog.LevelInfo:
		levelColor = "\033[32m" // Green
	case slog.LevelWarn:
		levelColor = "\033[33m" // Yellow
	case slog.LevelError:
		levelColor = "\033[31m" // Red
	default:
		levelColor = "\033[35m" // Magenta
	}
	resetColor := "\033[0m"

	// Time format
	timeStr := r.Time.Format("15:04:05.000")

	// Caller info (if enabled)
	var callerStr string
	if h.opts.AddSource && r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		if f.File != "" {
			shortFile := f.File
			if idx := strings.LastIndex(f.File, "/"); idx != -1 {
				shortFile = f.File[idx+1:]
			}
			callerStr = " \033[90m" + shortFile + ":" + strconv.Itoa(f.Line) + "\033[0m"
		}
	}

	// Format level
	levelStr := levelColor + r.Level.String() + resetColor

	// Message
	msg := r.Message

	// Format attributes
	var attrs []string
	for _, a := range h.attrs {
		attrs = append(attrs, a.Key+"="+a.Value.String())
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a.Key+"="+a.Value.String())
		return true
	})

	attrsStr := ""
	if len(attrs) > 0 {
		attrsStr = " \033[90m{" + strings.Join(attrs, ", ") + "}\033[0m"
	}

	_, err := fmt.Fprintf(h.w, "%s %s%s %s%s\n", timeStr, levelStr, callerStr, msg, attrsStr)
	return err
}

func (h *ElegantConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := append([]slog.Attr{}, h.attrs...)
	newAttrs = append(newAttrs, attrs...)
	return &ElegantConsoleHandler{
		w:     h.w,
		opts:  h.opts,
		attrs: newAttrs,
	}
}

func (h *ElegantConsoleHandler) WithGroup(name string) slog.Handler {
	return h
}

func Init(cfg Config) error {
	switch cfg.Env {
	case "dev":
		return initDev(cfg)
	case "prod":
		return initProd(cfg)
	default:
		return initProd(cfg)
	}
}

func initDev(cfg Config) error {
	level := getSlogLevel(cfg.Level)
	opts := slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}
	App = slog.New(NewElegantConsoleHandler(os.Stdout, opts))

	return nil
}

func initProd(cfg Config) error {
	if err := os.MkdirAll(cfg.LogDir, 0750); err != nil {
		return err
	}

	level := getSlogLevel(cfg.Level)
	opts := slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	appWriter := &lumberjack.Logger{
		Filename:   cfg.LogDir + "/gateway.log",
		MaxSize:    cfg.MaxSizeMB,
		MaxAge:     cfg.MaxAgeDays,
		MaxBackups: cfg.MaxBackups,
		Compress:   cfg.Compress,
	}
	fileHandler := slog.NewJSONHandler(appWriter, &opts)
	consoleHandler := NewElegantConsoleHandler(os.Stdout, opts)

	App = slog.New(&MultiHandler{
		handlers: []slog.Handler{
			fileHandler,
			consoleHandler,
		},
	}).With("service", "gateway")

	return nil
}

func getSlogLevel(lvl string) slog.Level {
	switch strings.ToLower(lvl) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// MultiHandler multiplexes log records to multiple slog.Handlers
type MultiHandler struct {
	handlers []slog.Handler
}

func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: newHandlers}
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: newHandlers}
}
