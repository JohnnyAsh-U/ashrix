package identity

import (
	"context"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/google/uuid"
)

type Repository interface {
	CreateTenantIdentityConfig(ctx context.Context, params store.CreateIDPConfigParams) (store.IdpConfig, error)
	GetIdentityConfigByID(ctx context.Context, id uuid.UUID) (store.IdpConfig, error)
	UpdateIdentityConfig(ctx context.Context, params store.UpdateIDPConfigParams) (store.IdpConfig, error)
	DeleteIdentityConfig(ctx context.Context, params store.DeleteIDPConfigParams) (store.IdpConfig, error)
	ListIdentityConfigsForTenant(ctx context.Context, tenantID uuid.UUID) ([]store.IdpConfig, error)
	ListAppIdPConfigs(ctx context.Context, id uuid.UUID) ([]store.IdpConfig, error)
	ListAppsByOrg(ctx context.Context, orgID uuid.UUID) ([]store.App, error)
		GetOrgByID(ctx context.Context, id uuid.UUID) (store.Org, error)

}

type postgresRepository struct {
	q *store.Queries
}

func NewPostgresRepository(q *store.Queries) Repository {
	return &postgresRepository{q: q}
}
func (r *postgresRepository) CreateTenantIdentityConfig(ctx context.Context, params store.CreateIDPConfigParams) (store.IdpConfig, error) {
	return r.q.CreateIDPConfig(ctx, params)
}

func (r *postgresRepository) GetIdentityConfigByID(ctx context.Context, id uuid.UUID) (store.IdpConfig, error) {
	return r.q.GetIDPConfigByID(ctx, id)
}

func (r *postgresRepository) UpdateIdentityConfig(ctx context.Context, params store.UpdateIDPConfigParams) (store.IdpConfig, error) {
	return r.q.UpdateIDPConfig(ctx, params)
}

func (r *postgresRepository) DeleteIdentityConfig(ctx context.Context, params store.DeleteIDPConfigParams) (store.IdpConfig, error) {
	return r.q.DeleteIDPConfig(ctx, params)
}

func (r *postgresRepository) ListIdentityConfigsForTenant(ctx context.Context, tenantID uuid.UUID) ([]store.IdpConfig, error) {
	return r.q.ListIDPConfigsByOrg(ctx, tenantID)
}

func (r *postgresRepository) ListAppIdPConfigs(ctx context.Context, tenantID uuid.UUID) ([]store.IdpConfig, error) {
	return r.q.ListAppIdPs(ctx, tenantID)
}

func (r *postgresRepository) ListAppsByOrg(ctx context.Context, orgID uuid.UUID) ([]store.App, error) {
	return r.q.ListAppsByOrg(ctx, orgID)
}

func (r *postgresRepository) GetOrgByID(ctx context.Context, id uuid.UUID) (store.Org, error) {
	return r.q.GetOrgByID(ctx, id)
}

