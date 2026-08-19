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


	CreateUserSessionForGateway(ctx context.Context, params store.CreateUserSessionForGatewayParams) (store.UserSession, error)
	GetUserSessionByID(ctx context.Context, id uuid.UUID) (store.UserSession, error)
	GetUserSessionsByOrgAndUser(ctx context.Context, params store.GetUserActiveSessionParams) ([]store.UserSession, error)
	RevokeUserSession(ctx context.Context, id uuid.UUID) (store.UserSession, error)

	CreateAppIdPMapping(ctx context.Context, params store.AddAppIdpMappingParams) (store.AppIdpMapping, error)
	DeleteAppIdpMapping(ctx context.Context, params store.DeleteAppIdpMappingParams) (store.AppIdpMapping, error)
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


func (r *postgresRepository) CreateUserSessionForGateway(ctx context.Context,  params store.CreateUserSessionForGatewayParams) (store.UserSession, error) {
	return r.q.CreateUserSessionForGateway(ctx, params)
}



func (r *postgresRepository) GetUserSessionsByOrgAndUser(ctx context.Context,  params store.GetUserActiveSessionParams) ([]store.UserSession, error) {
	return r.q.GetUserActiveSession(ctx, params)
}



func (r *postgresRepository) RevokeUserSession(ctx context.Context,  id uuid.UUID) (store.UserSession, error) {
	return r.q.RevokeActiveUserSession(ctx, id)
}

func (r *postgresRepository) GetUserSessionByID(ctx context.Context, id uuid.UUID) (store.UserSession, error) {
	return r.q.GetUserSessionByID(ctx, id)
}


func (r *postgresRepository) CreateAppIdPMapping(ctx context.Context, params store.AddAppIdpMappingParams) (store.AppIdpMapping, error) {
	return r.q.AddAppIdpMapping(ctx, params)
}

func (r *postgresRepository) DeleteAppIdpMapping(ctx context.Context, params store.DeleteAppIdpMappingParams) (store.AppIdpMapping, error) {
	return r.q.DeleteAppIdpMapping(ctx, params)
}




