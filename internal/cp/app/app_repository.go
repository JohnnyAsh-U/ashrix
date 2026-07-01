package app

import (
	"context"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, params store.CreateAppParams) (store.App, error)
	GetByIDAndOrg(ctx context.Context, params store.GetAppByIDAndOrgParams) (store.App, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]store.App, error)
	Update(ctx context.Context, params store.UpdateAppParams) (store.App, error)
	Delete(ctx context.Context, params store.DeleteAppParams) (store.App, error)
}

type postgresRepository struct {
	q *store.Queries
}

func NewPostgresRepository(q *store.Queries) Repository {
	return &postgresRepository{q: q}
}

func (r *postgresRepository) Create(ctx context.Context, params store.CreateAppParams) (store.App, error) {
	return r.q.CreateApp(ctx, params)
}

func (r *postgresRepository) GetByIDAndOrg(ctx context.Context, params store.GetAppByIDAndOrgParams) (store.App, error) {
	return r.q.GetAppByIDAndOrg(ctx, params)
}

func (r *postgresRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]store.App, error) {
	return r.q.ListAppsByOrg(ctx, orgID)
}

func (r *postgresRepository) Update(ctx context.Context, params store.UpdateAppParams) (store.App, error) {
	return r.q.UpdateApp(ctx, params)
}

func (r *postgresRepository) Delete(ctx context.Context, params store.DeleteAppParams) (store.App, error) {
	return r.q.DeleteApp(ctx, params)
}
