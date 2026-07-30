package config

import (
    "context"
    "fmt"
    "time"
    "github.com/redis/go-redis/v9"
)

type RedisStore struct {
    client *redis.Client
}

func NewRedisStore(ctx context.Context,cfg *Config) (*RedisStore, error) {
	opts, err := redis.ParseURL(cfg.RedisAddr)
	if err != nil {
		return nil, fmt.Errorf("Invalid redis Url: %w", err)
	}

	if cfg.RedisPassword != ""{
		opts.Password = cfg.RedisPassword
	}

	opts.PoolSize = 20
	// opts.MinIdleConns = poolSize / 5
	opts.MaxRetries = 3
	opts.DialTimeout =  5 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second
	opts.PoolTimeout = 4 * time.Second
	client := redis.NewClient(opts)


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