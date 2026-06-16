package org

import (
	"context"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, params store.CreateOrgParams) (store.Org, error)
	GetByID(ctx context.Context, id uuid.UUID) (store.Org, error)
	GetBySlug(ctx context.Context, slug string) (store.Org, error)
	UpdateName(ctx context.Context, params store.UpdateOrgNameParams) (store.Org, error)
	SetCustomDomain(ctx context.Context, params store.SetOrgCustomDomainParams) (store.Org, error)
	VerifyCustomDomain(ctx context.Context, params store.VerifyOrgCustomDomainParams) (store.Org, error)
	Delete(ctx context.Context, id uuid.UUID) (store.Org, error)
}

type postgresRepository struct {
	q *store.Queries
}

func NewPostgresRepository(q *store.Queries) Repository {
	return &postgresRepository{q: q}
}

func (r *postgresRepository) Create(ctx context.Context, params store.CreateOrgParams) (store.Org, error) {
	return r.q.CreateOrg(ctx, params)
}

func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (store.Org, error) {
	return r.q.GetOrgByID(ctx, id)
}

func (r *postgresRepository) GetBySlug(ctx context.Context, slug string) (store.Org, error) {
	return r.q.GetOrgBySlug(ctx, slug)
}

func (r *postgresRepository) UpdateName(ctx context.Context, params store.UpdateOrgNameParams) (store.Org, error) {
	return r.q.UpdateOrgName(ctx, params)
}

func (r *postgresRepository) SetCustomDomain(ctx context.Context, params store.SetOrgCustomDomainParams) (store.Org, error) {
	return r.q.SetOrgCustomDomain(ctx, params)
}

func (r *postgresRepository) VerifyCustomDomain(ctx context.Context, params store.VerifyOrgCustomDomainParams) (store.Org, error) {
	return r.q.VerifyOrgCustomDomain(ctx, params)
}

func (r *postgresRepository) Delete(ctx context.Context, id uuid.UUID) (store.Org,error) {
	return r.q.DeleteOrg(ctx, id)
}	