package http_proxy

import (
	// "io"
	// "log/slog"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/alicebob/miniredis/v2"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	// "go.uber.org/zap/zapcore"
)

func TestRateLimiterMiddleware_AllowThenDeny(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	logger, _ := zap.NewDevelopment()

	now := time.Now()
	cfg := RateLimiterConfig{
		Redis:     rdb,
		IP:        RateLimit{Rate: 1, Burst: 1}, // 1 req/sec, burst 1
		User:      RateLimit{Rate: 100, Burst: 100},
		KeyPrefix: "test",
		NowFunc:   func() time.Time { return now },
	}

	mw := RateLimiterMiddleware(cfg, logger)

	handler := withClientIP(
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)

	// 1st request: consumes the only token
	req1 := httptest.NewRequest(http.MethodGet, "/api/data", nil)

	req1.Header.Set("X-Real-IP", "203.0.113.25")

	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	assert.Equal(t, http.StatusOK, rr1.Code)
	assert.Equal(t, "1", rr1.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "0", rr1.Header().Get("X-RateLimit-Remaining"))

	// 2nd request: bucket empty → 429
	req2 := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req2.Header.Set("X-Real-IP", "203.0.113.25")

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
	assert.NotEmpty(t, rr2.Header().Get("Retry-After"))

	// Advance 1.1s: bucket refilled
	now = now.Add(1100 * time.Millisecond)
	req3 := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req3.Header.Set("X-Real-IP", "203.0.113.25")

	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)

	assert.Equal(t, http.StatusOK, rr3.Code)
}

func TestRateLimiterMiddleware_UserTier(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	// logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	logger, _ := zap.NewDevelopment()

	cfg := RateLimiterConfig{
		Redis:     rdb,
		IP:        RateLimit{Rate: 1, Burst: 1},
		User:      RateLimit{Rate: 5, Burst: 5}, // generous user limit
		KeyPrefix: "test",
	}

	mw := RateLimiterMiddleware(cfg, logger)

	handler := withClientIP(
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)

	// Inject session into context (simulates SessionMiddleware upstream)
	baseReq := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	baseReq.Header.Set("X-Real-IP", "203.0.113.25")

	sess := proto.NormalizedIdentity{UserId: "user-123", TenantId: "t1"}
	ctx := context.WithValue(
		baseReq.Context(),
		session.Identity,
		&sess,
	)
	// ctx := context.WithValue(baseReq.Context(), "Identity", &sess)
	req := baseReq.WithContext(ctx)

	// Fire 5 requests: all should pass (user burst = 5)
	for i := range 5 {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code, "request %d failed", i+1)
		rem, _ := strconv.Atoi(rr.Header().Get("X-RateLimit-Remaining"))
		assert.Equal(t, 4-i, rem)
	}

	// 6th request: exhausted
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestRateLimiterMiddleware_ExemptPaths(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	// logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	logger, _ := zap.NewDevelopment()

	cfg := RateLimiterConfig{
		Redis:     rdb,
		IP:        RateLimit{Rate: 0, Burst: 0}, // zero = disabled, but exempt should skip anyway
		KeyPrefix: "test",
	}

	mw := RateLimiterMiddleware(cfg, logger)
	handler := withClientIP(
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)

	for _, path := range []string{"/healths", "/logout", "/_auth/callback"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Real-IP", "203.0.113.25")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "path %s should be exempt", path)
	}
}

func TestRateLimiterMiddleware_FailOpen(t *testing.T) {
	// Point Redis at a dead port to simulate outage
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	// logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	logger, _ := zap.NewDevelopment()

	cfg := RateLimiterConfig{
		Redis:     rdb,
		IP:        RateLimit{Rate: 1, Burst: 1},
		KeyPrefix: "test",
	}

	mw := RateLimiterMiddleware(cfg, logger)
	handler := withClientIP(
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("X-Real-IP", "203.0.113.25")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should NOT block traffic when Redis is down
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRateLimiterMiddleware_IPFallbackWhenNoSession(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	// logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	logger, _ := zap.NewDevelopment()

	cfg := RateLimiterConfig{
		Redis:     rdb,
		IP:        RateLimit{Rate: 2, Burst: 2},
		User:      RateLimit{Rate: 100, Burst: 100},
		KeyPrefix: "test",
	}

	mw := RateLimiterMiddleware(cfg, logger)
	handler := withClientIP(
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("X-Real-IP", "203.0.113.25")

	// No session in context → falls back to IP limit (2 req burst)

	for range 2 {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func withClientIP(handler http.Handler) http.Handler {
	return chimiddleware.ClientIPFromHeader("X-Real-IP")(handler)
}
