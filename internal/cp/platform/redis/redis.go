package redis

import (
    "context"
    "fmt"
    "time"
    "github.com/redis/go-redis/v9"
)

type RedisStore struct {
    client *redis.Client
}

func NewRedisStore(ctx context.Context, addr, password string, db, poolSize int) (*RedisStore, error) {
    client := redis.NewClient(&redis.Options{
        Addr:         addr,
        Password:     password,
        DB:           db,
        PoolSize:     poolSize,
        MinIdleConns: poolSize / 5,
        MaxRetries:   3,
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
        PoolTimeout:  4 * time.Second,
    })

    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("failed to connect to redis: %w", err)
    }

    return &RedisStore{client: client}, nil
}

func (r *RedisStore) Close() error {
    return r.client.Close()
}

func (r *RedisStore) Client() *redis.Client {
    return r.client
}