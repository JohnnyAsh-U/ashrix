package app

import (
	"context"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Repository interface {
	Create(ctx context.Context, params store.CreateAppParams) (store.App, error)
	GetByIDAndOrg(ctx context.Context, params store.GetAppByIDAndOrgParams) (store.App, error)
	GetByID(ctx context.Context, appID uuid.UUID) (store.App, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]store.App, error)
	Update(ctx context.Context, params store.UpdateAppParams) (store.App, error)
	UpdateAppHealthStatus(ctx context.Context, params store.UpdateAppHealthStatusParams) (store.App, error)
	Delete(ctx context.Context, params store.DeleteAppParams) (store.App, error)
	ListAppsWithDetailsByOrg(ctx context.Context, orgID uuid.UUID) ([]store.ListAppsWithDetailsByOrgRow, error)
	CountAppTrafficToday(ctx context.Context, appID pgtype.UUID) (int64, error)
	ListPoliciesByAppResource(ctx context.Context, params store.ListPoliciesByAppResourceParams) ([]store.Policy, error)
	CountPolicyUserSubjectsByApp(ctx context.Context, params store.CountPolicyUserSubjectsByAppParams) (int64, error)
	CountPolicyGroupSubjectsByApp(ctx context.Context, params store.CountPolicyGroupSubjectsByAppParams) (int64, error)
	MarkOfflineStaleAppStatus(ctx context.Context, minutes string) (int64, error)
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

func (r *postgresRepository) GetByID(ctx context.Context, appID uuid.UUID) (store.App, error) {
	return r.q.GetAppByID(ctx, appID)
}

func (r *postgresRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]store.App, error) {
	return r.q.ListAppsByOrg(ctx, orgID)
}

func (r *postgresRepository) Update(ctx context.Context, params store.UpdateAppParams) (store.App, error) {
	return r.q.UpdateApp(ctx, params)
}

func (r *postgresRepository) UpdateAppHealthStatus(ctx context.Context, params store.UpdateAppHealthStatusParams) (store.App, error) {
	return r.q.UpdateAppHealthStatus(ctx, params)
}

func (r *postgresRepository) Delete(ctx context.Context, params store.DeleteAppParams) (store.App, error) {
	return r.q.DeleteApp(ctx, params)
}

func (r *postgresRepository) ListAppsWithDetailsByOrg(ctx context.Context, orgID uuid.UUID) ([]store.ListAppsWithDetailsByOrgRow, error) {
	return r.q.ListAppsWithDetailsByOrg(ctx, orgID)
}

func (r *postgresRepository) CountAppTrafficToday(ctx context.Context, appID pgtype.UUID) (int64, error) {
	return r.q.CountAppTrafficToday(ctx, appID)
}

func (r *postgresRepository) ListPoliciesByAppResource(ctx context.Context, params store.ListPoliciesByAppResourceParams) ([]store.Policy, error) {
	return r.q.ListPoliciesByAppResource(ctx, params)
}

func (r *postgresRepository) CountPolicyUserSubjectsByApp(ctx context.Context, params store.CountPolicyUserSubjectsByAppParams) (int64, error) {
	return r.q.CountPolicyUserSubjectsByApp(ctx, params)
}

func (r *postgresRepository) CountPolicyGroupSubjectsByApp(ctx context.Context, params store.CountPolicyGroupSubjectsByAppParams) (int64, error) {
	return r.q.CountPolicyGroupSubjectsByApp(ctx, params)
}

func (r *postgresRepository) MarkOfflineStaleAppStatus(ctx context.Context, minutes string) (int64, error) {
	return r.q.MarkOfflineStaleApp(ctx, minutes)
}
