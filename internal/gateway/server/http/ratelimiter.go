package http_proxy

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
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
	Rate  float64       // tokens per second
	Burst int           // bucket capacity (max burst)
}

type RateLimiterConfig struct {
	Redis     *redis.Client
	IP        RateLimit   // anonymous / pre-session traffic
	User      RateLimit   // authenticated traffic
	KeyPrefix string      // Redis key prefix (default "ratelimit")
	NowFunc   func() time.Time // for tests; nil → time.Now
}

func DefaultRateLimiterConfig(rdb *redis.Client) RateLimiterConfig {
	return RateLimiterConfig{
		Redis:     rdb,
		IP:        RateLimit{Rate: 10, Burst: 20},   // 10/s sustained, burst 20
		User:      RateLimit{Rate: 100, Burst: 150}, // 100/s sustained, burst 150
		KeyPrefix: "ratelimit",
	}
}

// RateLimiterMiddleware returns the rate-limiting handler.
func RateLimiterMiddleware(cfg RateLimiterConfig, logger *zap.Logger) func(http.Handler) http.Handler {
	// Atomic token-bucket via Lua. Returns {allowed, tokens_remaining}.
	const luaTokenBucket = `
		local key = KEYS[1]
		local rate = tonumber(ARGV[1])
		local capacity = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		local cost = tonumber(ARGV[4])

		local bucket = redis.call('HMGET', key, 'tokens', 'last')
		local tokens = capacity
		local last = now

		if bucket[1] then
			tokens = tonumber(bucket[1])
			last = tonumber(bucket[2])
		end

		local delta = math.max(0, now - last)
		tokens = math.min(capacity, tokens + delta * rate)

		local allowed = 0
		if tokens >= cost then
			tokens = tokens - cost
			allowed = 1
		end

		redis.call('HMSET', key, 'tokens', tokens, 'last', now)
		redis.call('EXPIRE', key, math.ceil(capacity / rate) + 1)
		return {tostring(allowed), tostring(tokens)}
	`

	script := redis.NewScript(luaTokenBucket)

	nowFn := cfg.NowFunc
	if nowFn == nil {
		nowFn = time.Now
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isInternalPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Skip if limits are zero/disabled
			var limit RateLimit
			var key string

			sess := session.IdentityFromCtx(r.Context())
			if sess != nil {
				limit = cfg.User
				key = sess.TenantId
			} else {
				clientIP := middleware.GetClientIP(r.Context())
				if clientIP == "" {
					http.Error(w, "unable to determine client", http.StatusBadRequest)
					return
				}
				limit = cfg.IP
				key = fmt.Sprintf("%s:ip:%s", cfg.KeyPrefix, clientIP)
			}

			if limit.Rate <= 0 || limit.Burst <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			now := nowFn()
			nowSec := float64(now.UnixNano()) / 1e9

			res, err := script.Run(r.Context(), cfg.Redis, []string{key},
				limit.Rate,
				limit.Burst,
				nowSec,
				1, // cost per request
			).Result()

			if err != nil {
				// Fail open: log error but do not drop traffic
				logger.Warn("rate limiter redis error",
					zap.String("error", err.Error()),
					zap.String("key", key),
				)
				next.ServeHTTP(w, r)
				return
			}

			vals := res.([]interface{})
			allowed := vals[0].(string) == "1"
			tokensFloat, _ := strconv.ParseFloat(vals[1].(string), 64)
			remaining := int64(math.Floor(tokensFloat))

			// Rate-limit headers (standard de-facto format)
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit.Burst))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

			if remaining < int64(limit.Burst) {
				resetAt := nowSec + float64(int64(limit.Burst) - remaining)/limit.Rate
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
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}


func isInternalPath(path string) bool {
	return path == "/health" || path == "/logout" ||
		strings.HasPrefix(path, "/_auth/")
}