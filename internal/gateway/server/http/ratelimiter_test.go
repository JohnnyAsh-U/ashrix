package http_proxy

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http/web"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiterMiddleware_AllowThenDeny(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Now()
	config := RateLimiterConfig{
		IP:        RateLimit{Rate: 1, Burst: 1},
		KeyPrefix: "test",
		NowFunc:   func() time.Time { return now },
	}
	handler := withClientIP(RateLimiterMiddleware(config, logger, web.NewErrorHandler())(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	))

	serve := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		req.Header.Set("X-Real-IP", "203.0.113.25")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder
	}

	first := serve()
	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, "1", first.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "0", first.Header().Get("X-RateLimit-Remaining"))
	assert.Equal(t, http.StatusTooManyRequests, serve().Code)
	now = now.Add(1100 * time.Millisecond)
	assert.Equal(t, http.StatusOK, serve().Code)
}

func TestRateLimiterMiddleware_ExemptPaths(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := RateLimiterConfig{IP: RateLimit{Rate: 0, Burst: 0}, KeyPrefix: "test"}
	handler := withClientIP(RateLimiterMiddleware(config, logger, web.NewErrorHandler())(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	))

	for _, path := range []string{"/_ashrix/health", "/_ashrix/static/app.js"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Real-IP", "203.0.113.25")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		assert.Equal(t, http.StatusOK, recorder.Code, "path %s should be exempt", path)
	}
}

func TestRateLimiterMiddleware_SeparatesAppsForSameIP(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := RateLimiterConfig{
		IP:        RateLimit{Rate: 1, Burst: 1},
		KeyPrefix: "test",
	}
	handler := withClientIP(RateLimiterMiddleware(config, logger, web.NewErrorHandler())(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	))
	serve := func(appID string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		req.Header.Set("X-Real-IP", "203.0.113.25")
		ctx := context.WithValue(req.Context(), session.AppID, appID)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req.WithContext(ctx))
		return recorder.Code
	}

	assert.Equal(t, http.StatusOK, serve("app-one"))
	assert.Equal(t, http.StatusTooManyRequests, serve("app-one"))
	assert.Equal(t, http.StatusOK, serve("app-two"))
}

func withClientIP(handler http.Handler) http.Handler {
	return chimiddleware.ClientIPFromHeader("X-Real-IP")(handler)
}
