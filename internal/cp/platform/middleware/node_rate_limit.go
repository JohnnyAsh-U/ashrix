package middleware

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/dto"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	maxNodeAttemptsPerHour = 5
	maxIPAttemptsPerMinute = 20
	maxRateLimitBodyBytes  = 1 << 20
)

type NodeIDResolver func(*http.Request, []byte) string

func NodeAttemptRateLimit(client *redis.Client, logger *slog.Logger, nodeType string, resolveNodeID NodeIDResolver) func(http.Handler) http.Handler {
	const slidingWindowScript = `
		local now = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - window)
		local count = redis.call('ZCARD', KEYS[1])
		if count >= limit then
			local oldest = redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')
			return {0, tonumber(oldest[2]) + window - now}
		end
		redis.call('ZADD', KEYS[1], now, ARGV[4])
		redis.call('PEXPIRE', KEYS[1], window)
		return {1, 0}
	`
	script := redis.NewScript(slidingWindowScript)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil || ip == "" {
				dto.SendError(w, dto.NewBadRequestError("Unable to determine source IP address"))
				return
			}

			now := time.Now().UnixMilli()
			if limited, retryAfter, err := checkRateLimit(r, client, script,
				fmt.Sprintf("cp:attempts:ip:%s", ip), now, time.Minute, maxIPAttemptsPerMinute,
			); err != nil {
				logger.Error("CP rate limiter unavailable", "scope", "ip", "error", err)
				dto.SendError(w, dto.NewAppError(http.StatusServiceUnavailable, dto.CodeInternal, "Rate limiter unavailable", nil))
				return
			} else if limited {
				sendRateLimitError(w, retryAfter, "Too many requests from this IP address")
				return
			}

			var body []byte
			if r.Body != nil {
				body, err = io.ReadAll(io.LimitReader(r.Body, maxRateLimitBodyBytes+1))
				if err != nil {
					dto.SendError(w, dto.NewBadRequestError("Unable to read request body"))
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}

			nodeID := resolveNodeID(r, body)
			if nodeID != "" {
				key := fmt.Sprintf("cp:attempts:node:%s:%s", nodeType, nodeID)
				limited, retryAfter, err := checkRateLimit(r, client, script, key, now, time.Hour, maxNodeAttemptsPerHour)
				if err != nil {
					logger.Error("CP rate limiter unavailable", "scope", "node", "node_type", nodeType, "error", err)
					dto.SendError(w, dto.NewAppError(http.StatusServiceUnavailable, dto.CodeInternal, "Rate limiter unavailable", nil))
					return
				}
				if limited {
					sendRateLimitError(w, retryAfter, "Too many requests for this node")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func checkRateLimit(r *http.Request, client *redis.Client, script *redis.Script, key string, now int64, window time.Duration, limit int) (bool, int64, error) {
	result, err := script.Run(r.Context(), client, []string{key},
		now,
		window.Milliseconds(),
		limit,
		uuid.NewString(),
	).Result()
	if err != nil {
		return false, 0, err
	}
	values := result.([]interface{})
	allowed := fmt.Sprint(values[0]) == "1"
	retryMillis, _ := strconv.ParseInt(fmt.Sprint(values[1]), 10, 64)
	return !allowed, retryMillis, nil
}

func sendRateLimitError(w http.ResponseWriter, retryMillis int64, message string) {
	retryAfter := (retryMillis + 999) / 1000
	if retryAfter < 1 {
		retryAfter = 1
	}
	w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
	dto.SendError(w, dto.NewAppError(http.StatusTooManyRequests, dto.CodeRateLimited, message, nil))
}
