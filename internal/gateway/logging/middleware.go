package logging

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/go-chi/chi/v5/middleware"
)

// ============================================================
// ACCESS LOG MIDDLEWARE
// ============================================================
// Logs every HTTP request with structured fields. Runs at the
// outer layer so it can capture the final status code and
// latency of the entire chain (including Policy → Proxy).
// ============================================================

type countingReadCloser struct {
	io.ReadCloser
	bytes int64
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	c.bytes += int64(n)
	return n, err
}

type logResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	bytes       int64
	wroteHeader bool
}

func newLogResponseWriter(w http.ResponseWriter) *logResponseWriter {
	return &logResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (lrw *logResponseWriter) WriteHeader(code int) {
	if !lrw.wroteHeader {
		lrw.statusCode = code
		lrw.wroteHeader = true
	}
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *logResponseWriter) Write(b []byte) (int, error) {
	if !lrw.wroteHeader {
		lrw.WriteHeader(http.StatusOK)
	}
	n, err := lrw.ResponseWriter.Write(b)
	lrw.bytes += int64(n)
	return n, err
}

func (lrw *logResponseWriter) Flush() {
	if f, ok := lrw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (lrw *logResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := lrw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
	}

	return h.Hijack()
}

// AccessLogMiddleware logs every request to rotated file and optionally CP asynchronously.
func AccessLogMiddleware(accessLogger *AccessLogger, gatewayID string, fallbackLogger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if shouldSkipAccessLog(r) {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Create request-scoped access context.
			ctx, accessCtx := WithAccessContext(r.Context())
			r = r.WithContext(ctx)

			requestCounter := &countingReadCloser{
				ReadCloser: r.Body,
			}
			r.Body = requestCounter

			// Wrap writer to capture status and bytes
			lrw := newLogResponseWriter(w)

			next.ServeHTTP(lrw, r)

			bytesIn := requestCounter.bytes
			bytesOut := lrw.bytes

			// ----------------------------------------------------
			// Read everything collected during the request.
			// ----------------------------------------------------

			appID := accessCtx.AppID

			latency := time.Since(start)

			// Build structured log attributes
			attrs := []any{
				"event", "http.access",
				"gateway_id", gatewayID,
				"connector_id", accessCtx.ConnectorID,
				"app_id", appID,
				"app_name", accessCtx.AppName,
				"request_id", middleware.GetReqID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"host", r.Host,
				"remote_addr", r.RemoteAddr,
				"status", lrw.statusCode,
				"bytes_in", bytesIn,
				"bytes_out", bytesOut,
				"latency", latency.Milliseconds(),
				"user_agent", r.UserAgent(),
			}

			if accessLogger != nil {
				accessLogger.Log(r.Context(), attrs...)
			}
			if fallbackLogger != nil {
				switch {
				case lrw.statusCode >= 500:
					fallbackLogger.Error("http access", attrs...)
				case lrw.statusCode >= 400:
					fallbackLogger.Warn("http access", attrs...)
				default:
					fallbackLogger.Info("http access", attrs...)
				}
			}
		})
	}
}

func shouldSkipAccessLog(r *http.Request) bool {
	path := r.URL.Path
	return strings.HasPrefix(path, "/_ashrix")
}
