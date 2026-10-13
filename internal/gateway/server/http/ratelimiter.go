package http_proxy

import (
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http/web"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
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
	Redis          *redis.Client
	IP             RateLimit        // anonymous / pre-session traffic
	User           RateLimit        // authenticated traffic
	CallbackIP     int              // auth callback requests per minute per IP
	CallbackGlobal int              // auth callback requests per minute gateway-wide
	CallbackScope  string           // gateway identity for shared-Redis deployments
	KeyPrefix      string           // Redis key prefix (default "ratelimit")
	NowFunc        func() time.Time // for tests; nil → time.Now
}

func DefaultRateLimiterConfig(rdb *redis.Client) RateLimiterConfig {
	return RateLimiterConfig{
		Redis:          rdb,
		IP:             RateLimit{Rate: 10, Burst: 20},   // 10/s sustained, burst 20
		User:           RateLimit{Rate: 100, Burst: 150}, // 100/s sustained, burst 150
		CallbackIP:     10,
		CallbackGlobal: 100,
		CallbackScope:  "gateway",
		KeyPrefix:      "ratelimit",
	}
}

// RateLimiterMiddleware returns the rate-limiting handler.
func RateLimiterMiddleware(cfg RateLimiterConfig, logger *slog.Logger, errorHandler *web.ErrorHandler) func(http.Handler) http.Handler {
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
	const luaCallbackWindow = `
		local now = tonumber(ARGV[1])
		local member = ARGV[2]
		local window = tonumber(ARGV[3])
		local limits = {tonumber(ARGV[4]), tonumber(ARGV[5])}
		local counts = {}
		local allowed = 1
		local retryAfter = 0

		for i = 1, 2 do
			redis.call('ZREMRANGEBYSCORE', KEYS[i], '-inf', now - window)
			counts[i] = redis.call('ZCARD', KEYS[i])
			if counts[i] >= limits[i] then
				allowed = 0
				local oldest = redis.call('ZRANGE', KEYS[i], 0, 0, 'WITHSCORES')
				local wait = tonumber(oldest[2]) + window - now
				if wait > retryAfter then
					retryAfter = wait
				end
			end
		end

		if allowed == 1 then
			for i = 1, 2 do
				redis.call('ZADD', KEYS[i], now, member)
				redis.call('PEXPIRE', KEYS[i], window * 2)
			end
		end
		return {tostring(allowed), tostring(counts[1]), tostring(counts[2]), tostring(retryAfter)}
	`
	callbackScript := redis.NewScript(luaCallbackWindow)

	nowFn := cfg.NowFunc
	if nowFn == nil {
		nowFn = time.Now
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/_ashrix/auth/callback" {
				clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
				if err != nil || clientIP == "" {
					errorHandler.ErrorPage(w, http.StatusBadRequest, "Bad Request", "Unable to determine client IP address.", "")
					return
				}

				ipLimit := cfg.CallbackIP
				globalLimit := cfg.CallbackGlobal
				if ipLimit <= 0 || globalLimit <= 0 {
					next.ServeHTTP(w, r)
					return
				}

				now := nowFn()
				nowMillis := now.UnixMilli()
				windowMillis := int64(time.Minute / time.Millisecond)
				callbackScope := cfg.CallbackScope
				if callbackScope == "" {
					callbackScope = "gateway"
				}
				keys := []string{
					fmt.Sprintf("%s:callback:{%s}:ip:%s", cfg.KeyPrefix, callbackScope, clientIP),
					fmt.Sprintf("%s:callback:{%s}:global", cfg.KeyPrefix, callbackScope),
				}
				res, err := callbackScript.Run(r.Context(), cfg.Redis, keys,
					nowMillis,
					uuid.NewString(),
					windowMillis,
					ipLimit,
					globalLimit,
				).Result()
				if err != nil {
					logger.Warn("callback rate limiter redis error", slog.Any("error", err), slog.String("ip", clientIP))
					next.ServeHTTP(w, r)
					return
				}

				vals := res.([]interface{})
				allowed := vals[0].(string) == "1"
				ipCount, _ := strconv.Atoi(vals[1].(string))
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(ipLimit))
				remaining := ipLimit - ipCount
				if allowed {
					remaining--
				}
				w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(max(0, remaining)))
				if !allowed {
					retryMillis, _ := strconv.ParseInt(vals[3].(string), 10, 64)
					retryAfter := int64(math.Ceil(float64(retryMillis) / 1000))
					if retryAfter < 1 {
						retryAfter = 1
					}
					w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
					errorHandler.ErrorPage(w, http.StatusTooManyRequests, "Rate Limit Exceeded", "You have exceeded the authentication callback rate limit. Please try again later.", "")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			appIsPublic := session.AppIsPublicFromCtx(r.Context())
			if isInternalPath(r.URL.Path) || appIsPublic {
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
					errorHandler.ErrorPage(w, http.StatusBadRequest, "Bad Request", "Unable to determine client IP address.", "")
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
					slog.String("error", err.Error()),
					slog.String("key", key),
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
	return path == "/_ashrix/health" || path == "/_ashrix/logout" ||
		strings.HasPrefix(path, "/_ashrix/auth/") || strings.HasPrefix(path, "/_ashrix/static/")
}
