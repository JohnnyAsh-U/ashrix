package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestId := uuid.NewString()

			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			r.Header.Set("X-Request-ID", requestId)
			ctx := context.WithValue(
				r.Context(), "request_id", requestId,
			)

			next.ServeHTTP(rw, r.WithContext(ctx))

			duration := time.Since(start)

			log.Info("incoming request",
				slog.String("request_id", requestId),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status_code", rw.statusCode),
				slog.Duration("duration", duration),
				slog.String("remote_addr", r.RemoteAddr),
			)
		})
	}
}
