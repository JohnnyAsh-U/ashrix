package http_proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdentityHeadersReplaceUntrustedHeaders(t *testing.T) {
	incoming := make(http.Header)
	incoming["x-ashrix-user-id"] = []string{"attacker"}
	incoming.Set("X-Ashrix-User-Email", "spoofed@example.com")
	incoming.Set("X-Other", "preserved")

	outbound := identityHeaders(incoming, "user-123", "user@example.com", "app-456", "request-789")

	assert.Equal(t, "user-123", outbound.Get("X-Ashrix-User-ID"))
	assert.Equal(t, "user@example.com", outbound.Get("X-Ashrix-User-Email"))
	assert.Equal(t, "app-456", outbound.Get("X-Ashrix-App-ID"))
	assert.Equal(t, "request-789", outbound.Get("X-Ashrix-Request-ID"))
	assert.Equal(t, "preserved", outbound.Get("X-Other"))
	assert.Empty(t, incoming.Get("X-Ashrix-User-ID"))
}

func TestSecurityHeadersMiddleware_DefaultConfig(t *testing.T) {
	cfg := DefaultSecurityConfig()
	mw := SecurityHeadersMiddleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("HTTPS request gets HSTS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"))
		assert.Equal(t, "strict-origin-when-cross-origin", rr.Header().Get("Referrer-Policy"))
		assert.Contains(t, rr.Header().Get("Strict-Transport-Security"), "max-age=31536000")
		assert.Contains(t, rr.Header().Get("Strict-Transport-Security"), "includeSubDomains")
		assert.Contains(t, rr.Header().Get("Strict-Transport-Security"), "preload")
		assert.NotEmpty(t, rr.Header().Get("Content-Security-Policy"))
		assert.NotEmpty(t, rr.Header().Get("Permissions-Policy"))
		assert.Empty(t, rr.Header().Get("Server"))
	})

	t.Run("HTTP request skips HSTS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Header().Get("Strict-Transport-Security"))
		// Other headers still present
		assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"))
	})
}

func TestSecurityHeadersMiddleware_CustomConfig(t *testing.T) {
	cfg := SecurityConfig{
		ContentSecurityPolicy: "default-src 'none'",
		HSTSMaxAge:            0, // disabled
		FrameOptions:          "SAMEORIGIN",
		ReferrerPolicy:        "no-referrer",
		PermissionsPolicy:     "camera=(self)",
		ContentTypeOptions:    "",
		StripServerHeader:     false,
	}
	mw := SecurityHeadersMiddleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "custom-server")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, "default-src 'none'", rr.Header().Get("Content-Security-Policy"))
	assert.Empty(t, rr.Header().Get("Strict-Transport-Security"))
	assert.Equal(t, "SAMEORIGIN", rr.Header().Get("X-Frame-Options"))
	assert.Equal(t, "no-referrer", rr.Header().Get("Referrer-Policy"))
	assert.Equal(t, "camera=(self)", rr.Header().Get("Permissions-Policy"))
	assert.Empty(t, rr.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "custom-server", rr.Header().Get("Server")) // not stripped
}

func TestSecurityHeadersMiddleware_NoCSP(t *testing.T) {
	cfg := DefaultSecurityConfig()
	cfg.ContentSecurityPolicy = ""
	mw := SecurityHeadersMiddleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Empty(t, rr.Header().Get("Content-Security-Policy"))
}

func TestSecurityHeadersMiddleware_HSTSWithoutSubdomains(t *testing.T) {
	cfg := DefaultSecurityConfig()
	cfg.HSTSNoSubdomains = true
	mw := SecurityHeadersMiddleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	assert.Contains(t, hsts, "max-age=")
	assert.NotContains(t, hsts, "includeSubDomains")
	assert.Contains(t, hsts, "preload")
}
