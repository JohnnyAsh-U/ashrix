package logs

import (
	// "context"

	"context"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	// "github.com/google/uuid"
)

type Repository interface {
	BulkCreateAccessLogs(ctx context.Context, params []store.BulkCreateAccessLogsParams) (int64, error)
}

type postgresRepository struct {
	q *store.Queries
}

func NewPostgresRepository(q *store.Queries) Repository {
	return &postgresRepository{q: q}
}


func (p *postgresRepository) BulkCreateAccessLogs(ctx context.Context, params []store.BulkCreateAccessLogsParams) (int64, error) {
	return p.q.BulkCreateAccessLogs(ctx, params)
}
