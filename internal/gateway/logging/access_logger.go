package logging

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

type AccessLogger struct {
	fileLogger *slog.Logger
	writer     *lumberjack.Logger
	closeOnce  sync.Once
}

func NewAccessLogger(cfg *config.Config) *AccessLogger {

	accessWriter := &lumberjack.Logger{
		Filename:   filepath.Join(cfg.Logging.Dir, "access.log"),
		MaxSize:    100, // MB
		MaxAge:     30,  // days
		MaxBackups: 10,
		Compress:   true,
	}

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	fileHandler := slog.NewJSONHandler(accessWriter, opts)
	fileLogger := slog.New(fileHandler).With("service", "gateway_access")

	return &AccessLogger{
		fileLogger: fileLogger,
		writer:     accessWriter,
	}
}

func (al *AccessLogger) Log(ctx context.Context, attrs ...any) {
	if al.fileLogger != nil {
		al.fileLogger.InfoContext(ctx, "access_log", attrs...)
	}
}

func (al *AccessLogger) Close() {
	al.closeOnce.Do(func() {
		if al.writer != nil {
			_ = al.writer.Close()
		}
	})
}
