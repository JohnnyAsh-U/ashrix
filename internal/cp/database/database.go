package database

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/config"
	"log/slog"
)

func Connect(ctx context.Context, cfg *config.Config, log *slog.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	poolConfig.MaxConns = cfg.DatabaseMaxConn
	poolConfig.MinConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)

	if err != nil {
		return nil, err
	}

	//Test Connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	log.Info("DB Connected successfully")

	return pool, nil
}
