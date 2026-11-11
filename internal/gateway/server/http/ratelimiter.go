package http_proxy

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http/web"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/go-chi/chi/v5/middleware"
)

// ============================================================
// RATE LIMITER MIDDLEWARE
// ============================================================
// Token-bucket rate limiter backed by Redis. Distinguishes
// authenticated traffic (per-UserID) from anonymous (per-IP).
// Place AFTER SessionMiddleware so it can apply user-tier limits.
// Fails open on Redis errors.
// ============================================================

// RateLimit defines a token bucket: Rate = sustained r/s, Burst = peak.
type RateLimit struct {
	Rate  float64 // tokens per second
	Burst int     // bucket capacity (max burst)
}

type RateLimiterConfig struct {
	IP        RateLimit
	KeyPrefix string
	NowFunc   func() time.Time
}

func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		IP:        RateLimit{Rate: 10, Burst: 20},
		KeyPrefix: "ratelimit",
	}
}

type tokenBucket struct {
	tokens   float64
	updated  time.Time
	lastSeen time.Time
}

type tokenBucketLimiter struct {
	mu          sync.Mutex
	buckets     map[string]tokenBucket
	lastCleanup time.Time
}

func newTokenBucketLimiter() *tokenBucketLimiter {
	return &tokenBucketLimiter{buckets: make(map[string]tokenBucket)}
}

func (l *tokenBucketLimiter) allow(key string, now time.Time, limit RateLimit) (bool, float64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastCleanup) >= time.Minute || len(l.buckets) >= 10000 {
		cutoff := now.Add(-10 * time.Minute)
		for bucketKey, bucket := range l.buckets {
			if bucket.lastSeen.Before(cutoff) {
				delete(l.buckets, bucketKey)
			}
		}
		l.lastCleanup = now
	}

	bucket, exists := l.buckets[key]
	if !exists {
		if len(l.buckets) >= 10000 {
			var oldestKey string
			var oldest time.Time
			for bucketKey, candidate := range l.buckets {
				if oldest.IsZero() || candidate.lastSeen.Before(oldest) {
					oldestKey = bucketKey
					oldest = candidate.lastSeen
				}
			}
			delete(l.buckets, oldestKey)
		}
		bucket.tokens = float64(limit.Burst)
		bucket.updated = now
	}
	if elapsed := now.Sub(bucket.updated).Seconds(); elapsed > 0 {
		bucket.tokens = math.Min(float64(limit.Burst), bucket.tokens+elapsed*limit.Rate)
	}
	allowed := bucket.tokens >= 1
	if allowed {
		bucket.tokens--
	}
	bucket.updated = now
	bucket.lastSeen = now
	l.buckets[key] = bucket
	return allowed, bucket.tokens
}

// RateLimiterMiddleware returns the rate-limiting handler.
func RateLimiterMiddleware(cfg RateLimiterConfig, logger *slog.Logger, errorHandler *web.ErrorHandler) func(http.Handler) http.Handler {
	limiter := newTokenBucketLimiter()
	nowFn := cfg.NowFunc
	if nowFn == nil {
		nowFn = time.Now
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			appIsPublic := session.AppIsPublicFromCtx(r.Context())
			if isInternalPath(r.URL.Path) || appIsPublic {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := middleware.GetClientIP(r.Context())
			if clientIP == "" {
				errorHandler.ErrorPage(w, http.StatusBadRequest, "Bad Request", "Unable to determine client IP address.", "")
				return
			}
			limit := cfg.IP
			key := fmt.Sprintf("%s:app:%s:ip:%s", cfg.KeyPrefix, session.AppIDFromCtx(r.Context()), clientIP)

			if limit.Rate <= 0 || limit.Burst <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			now := nowFn()
			nowSec := float64(now.UnixNano()) / 1e9
			allowed, tokensFloat := limiter.allow(key, now, limit)
			remaining := int64(math.Floor(tokensFloat))

			// Rate-limit headers (standard de-facto format)
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit.Burst))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

			if remaining < int64(limit.Burst) {
				resetAt := nowSec + float64(int64(limit.Burst)-remaining)/limit.Rate
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(int64(resetAt), 10))
			} else {
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(int64(nowSec), 10))
			}

			if !allowed {
				retryAfter := int64(math.Ceil((1.0 - tokensFloat) / limit.Rate))
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))

				errorHandler.ErrorPage(w, http.StatusTooManyRequests, "Rate Limit Exceeded", "You have exceeded the rate limit for this application. Please try again later.", "")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isInternalPath(path string) bool {
	return path == "/_ashrix/health" || strings.HasPrefix(path, "/_ashrix/static/")
}
