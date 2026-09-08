package http_proxy

import (
	"net/http"
	"strconv"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
)

// ============================================================
// SECURITY HEADERS MIDDLEWARE
// ============================================================
// Sets baseline security headers on every response. Configurable
// per-tenant via env vars; sane defaults for the MVP.
// ============================================================

// SecurityConfig allows tuning headers without recompiling.
type SecurityConfig struct {
	// CSP directives. Default is strict for a gateway (no inline scripts).
	ContentSecurityPolicy string

	// HSTS max-age in seconds. 0 disables HSTS.
	HSTSMaxAge int

	// Disable HSTS includeSubDomains (default true).
	HSTSNoSubdomains bool

	// Disable HSTS preload (default false).
	HSTSNoPreload bool

	// Frame options: "DENY", "SAMEORIGIN", or "".
	FrameOptions string

	// Referrer policy. Default "strict-origin-when-cross-origin".
	ReferrerPolicy string

	// Permissions policy features. Default disables camera/mic/geolocation.
	PermissionsPolicy string

	// Explicitly override X-Content-Type-Options (default "nosniff").
	ContentTypeOptions string

	// Remove the Server header entirely.
	StripServerHeader bool
}

// DefaultSecurityConfig returns production-safe defaults.
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		ContentSecurityPolicy: "default-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self';",
		HSTSMaxAge:            31536000, // 1 year
		HSTSNoSubdomains:      false,
		HSTSNoPreload:         false,
		FrameOptions:          "DENY",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		PermissionsPolicy:     "camera=(), microphone=(), geolocation=(), payment=(), usb=(), magnetometer=(), gyroscope=()",
		ContentTypeOptions:    "nosniff",
		StripServerHeader:     true,
	}
}

// SecurityHeadersMiddleware returns a middleware that sets security headers.
func SecurityHeadersMiddleware(cfg SecurityConfig) func(http.Handler) http.Handler {
	// Pre-compute static headers
	hsts := ""
	if cfg.HSTSMaxAge > 0 {
		hsts = "max-age=" + strconv.Itoa(cfg.HSTSMaxAge)
		if !cfg.HSTSNoSubdomains {
			hsts += "; includeSubDomains"
		}
		if !cfg.HSTSNoPreload {
			hsts += "; preload"
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if enableSec, ok := session.AppEnableSecurityHeadersFromCtx(r.Context()); ok && !enableSec {
				next.ServeHTTP(w, r)
				return
			}

			// Prevent MIME-type sniffing
			if cfg.ContentTypeOptions != "" {
				w.Header().Set("X-Content-Type-Options", cfg.ContentTypeOptions)
			}

			// Clickjacking protection
			if cfg.FrameOptions != "" {
				w.Header().Set("X-Frame-Options", cfg.FrameOptions)
			}

			// Referrer control
			if cfg.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)
			}

			// CSP
			if cfg.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
			}

			// Permissions Policy (modern replacement for Feature-Policy)
			if cfg.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", cfg.PermissionsPolicy)
			}

			// HSTS (only over HTTPS)
			if hsts != "" && r.TLS != nil {
				w.Header().Set("Strict-Transport-Security", hsts)
			}
			// w.Header().Set("Content-")

			// Strip server fingerprint
			if cfg.StripServerHeader {
				w.Header().Del("Server")
			}

			next.ServeHTTP(w, r)
		})
	}
}