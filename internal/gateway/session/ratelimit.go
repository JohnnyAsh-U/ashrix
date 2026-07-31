package session

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)



type RedisLimiter struct {
	client *redis.Client
	rate int
	window time.Duration
}

func NewRedisLimiter(client *redis.Client, rate int, window time.Duration) *RedisLimiter {
	return &RedisLimiter{client:client, rate:rate, window: window}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string) bool {
	redisKey := fmt.Sprintf("rateliimit:%s", key)
	pipe := l.client.Pipeline()
	incr := pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, l.window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false
	}
	return incr.Val() <= int64(l.rate)
}