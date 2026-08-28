package logging

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/go-chi/chi/v5/middleware"
)

// ============================================================
// ACCESS LOG MIDDLEWARE
// ============================================================
// Logs every HTTP request with structured fields. Runs at the
// outer layer so it can capture the final status code and
// latency of the entire chain (including Policy → Proxy).
// ============================================================

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

// AccessLogMiddleware logs every request. Place it early in the
// middleware stack (after RequestID/RealIP but before the core chain).
func AccessLogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap writer to capture status and bytes
			lrw := newLogResponseWriter(w)

			next.ServeHTTP(lrw, r)

			// Gather identity & decision if available
			var userID, tenantID, decisionEffect, policyID string
			sess := session.IdentityFromCtx(r.Context())
			if sess != nil {
				userID = sess.UserId
				tenantID = sess.TenantId
			}

			if dec, ok := policy.DecisionFromContext(r.Context()); ok {
				decisionEffect = string(dec.Effect)
				policyID = dec.PolicyID
			}

			// Build structured log
			attrs := []any{
				"event", "http.access",
				"request_id", middleware.GetReqID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"host", r.Host,
				"remote_addr", r.RemoteAddr,
				"status", lrw.statusCode,
				"bytes", lrw.bytes,
				"latency", time.Since(start),
				"user_agent", r.UserAgent(),
				"user_id", userID,
				"tenant_id", tenantID,
				"decision_effect", decisionEffect,
				"policy_id", policyID,
			}

			// Log at appropriate level
			switch {
			case lrw.statusCode >= 500:
				logger.Error("http access", attrs...)
			case lrw.statusCode >= 400:
				logger.Warn("http access", attrs...)
			default:
				logger.Info("http access", attrs...)
			}
		})
	}
}
