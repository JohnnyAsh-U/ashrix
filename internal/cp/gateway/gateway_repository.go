package gateway

import (
	"context"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/google/uuid"
)

type Repository interface {
	CreateGateway(ctx context.Context, params store.CreateGatewayParams) (store.Gateway, error)
	ListGatewayByOrg(ctx context.Context, orgID uuid.UUID) ([]store.Gateway, error) // Updated signature
	ReCreateGateway(ctx context.Context, params store.ReCreateGatewayParams) (store.Gateway, error)
	GetGatewayByTokenHash(ctx context.Context, token string) (store.Gateway, error)

	CreateGatewayCert(ctx context.Context, params store.RegisterCompCertParams) (store.ComponentCertificate, error)


	GetGatewayByID(ctx context.Context, id uuid.UUID) (store.Gateway, error)
	EnrollGatewayUsingTokenHash(ctx context.Context, token string) (store.Gateway, error)

	UpdateGatewayHeartBeat(ctx context.Context, params store.UpdateGatewayHeartbeatParams) (store.Gateway, error)

	RevokeGateway(ctx context.Context, params store.RevokeGatewayParams) (store.Gateway, error)

	// New methods
	RevokeGatewayCert(ctx context.Context, params store.RevokeCompCertParams) (store.ComponentCertificate, error)
}

type postgresRepository struct {
	q *store.Queries
}

func NewPostgresRepository(q *store.Queries) Repository {
	return &postgresRepository{q: q}
}

func (r *postgresRepository) CreateGateway(ctx context.Context, params store.CreateGatewayParams) (store.Gateway, error) {
	return r.q.CreateGateway(ctx, params)
}

func (r *postgresRepository) ListGatewayByOrg(ctx context.Context, orgID uuid.UUID) ([]store.Gateway, error) {
	return r.q.ListGatewaysByOrg(ctx, orgID)
}


func (r *postgresRepository) ReCreateGateway(ctx context.Context, params store.ReCreateGatewayParams) (store.Gateway, error) {
	return r.q.ReCreateGateway(ctx, params)
}





func (r *postgresRepository) GetGatewayByID(ctx context.Context, id uuid.UUID) (store.Gateway, error) {
	return r.q.GetGatewayByID(ctx, id)
}

func (r *postgresRepository) EnrollGatewayUsingTokenHash(ctx context.Context, token string) (store.Gateway, error) {
	return r.q.EnrollGateway(ctx, token)
}

func (r *postgresRepository) GetGatewayByTokenHash(ctx context.Context, token string) (store.Gateway, error) {
	return r.q.GetGatewayByTokenHash(ctx, token)
}

func (r *postgresRepository) UpdateGatewayHeartBeat(ctx context.Context, params store.UpdateGatewayHeartbeatParams) (store.Gateway, error) {
	return r.q.UpdateGatewayHeartbeat(ctx, params)
}

func (r *postgresRepository) RevokeGateway(ctx context.Context, params store.RevokeGatewayParams) (store.Gateway, error) {
	return r.q.RevokeGateway(ctx, params)
}

func (r *postgresRepository) RevokeGatewayCert(ctx context.Context, params store.RevokeCompCertParams) (store.ComponentCertificate, error) {
	return r.q.RevokeCompCert(ctx, params)
}

func (r *postgresRepository) CreateGatewayCert(ctx context.Context, params store.RegisterCompCertParams) (store.ComponentCertificate, error) {
	return r.q.RegisterCompCert(ctx, params)
}

