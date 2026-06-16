package middleware

import (
	"net/http"
	"strings"
)

// ── SecurityHeaders ────────────────────────────────────────────────────────
// Sets security headers on every response.
// Adjust CSP as needed when the dashboard frontend is added.

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/docs") {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
		}
		next.ServeHTTP(w, r)
	})
}
