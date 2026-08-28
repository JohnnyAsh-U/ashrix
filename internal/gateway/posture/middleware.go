package posture

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ============================================================
// 9. MIDDLEWARE (internal/posture/middleware.go)
// ============================================================
// Chi middleware that collects posture and stores it in context.
// ============================================================

func PostureMiddleware(collector *Collector, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip posture collection for health checks and auth callbacks
			if isInternalPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Collect posture (this may do GeoIP and Tor DNS lookups)
			dp, err := collector.Collect(r)
			if err != nil {
				// Log error but don't block the request
				// Posture collection failure should not be a hard failure
				log.Info("posture collection failed: %v", slog.Any("error", err))
				dp = &DevicePosture{
					Status:      "unknown",
					CollectedAt: time.Now(),
				}
			}

			// Store in context for downstream middleware
			ctx := WithContext(r.Context(), dp)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isInternalPath(path string) bool {
	return path == "/_ashrix/health" || path == "/_ashrix/logout" ||
		strings.HasPrefix(path, "/_ashrix/auth/") || strings.HasPrefix(path, "/_ashrix/static/")
}
