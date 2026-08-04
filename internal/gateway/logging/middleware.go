package logging

import (
	"net/http"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

// AccessLogMiddleware logs every request. Place it early in the
// middleware stack (after RequestID/RealIP but before the core chain).
func AccessLogMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
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
			attrs := []zap.Field{
				zap.String("event", "http.access"),
				zap.String("request_id", middleware.GetReqID(r.Context())),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("host", r.Host),
				zap.String("remote_addr", r.RemoteAddr),
				// zap.String("client_ip", extractClientIP(r).String()),
				zap.Int("status", lrw.statusCode),
				zap.Int64("bytes", lrw.bytes),
				zap.Duration("latency", time.Since(start)),
				zap.String("user_agent", r.UserAgent()),
				zap.String("user_id", userID),
				zap.String("tenant_id", tenantID),
				zap.String("decision_effect", decisionEffect),
				zap.String("policy_id", policyID),
			}

			// Log at appropriate level
			switch {
			case lrw.statusCode >= 500:
				logger.Log(zapcore.ErrorLevel, "http access", attrs...)
			case lrw.statusCode >= 400:
				logger.Log(zapcore.WarnLevel, "http access", attrs...)
			default:
				logger.Log(zapcore.InfoLevel, "http access", attrs...)
			}
		})
	}
}
