package posture

import (
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ============================================================
// 9. MIDDLEWARE (internal/posture/middleware.go)
// ============================================================
// Chi middleware that collects posture and stores it in context.
// ============================================================

func PostureMiddleware(collector *Collector, log *zap.Logger) func(http.Handler) http.Handler {
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
				log.Info("posture collection failed: %v", zap.Error(err))
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
	return path == "/health" || path == "/logout" ||
		strings.HasPrefix(path, "/_auth/")
}