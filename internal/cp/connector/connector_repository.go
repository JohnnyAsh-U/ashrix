package connector

import (
	"context"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Repository interface {
	CreateConnector(ctx context.Context, params store.CreateConnectorParams) (store.Connector, error)
	ListConnectorByOrg(ctx context.Context, orgID uuid.UUID) ([]store.Connector, error) // Updated signature
	ListActiveConnectorsByGateway(ctx context.Context, gatewayID uuid.UUID) ([]store.Connector, error)
	ReCreateConnector(ctx context.Context, params store.ReCreateConnectorParams) (store.Connector, error)
	GetConnectorByTokenHash(ctx context.Context, token string) (store.Connector, error)
	GetConnectorByID(ctx context.Context, id uuid.UUID) (store.Connector, error)

	GetConnectorApps(ctx context.Context, id pgtype.UUID) ([]store.App, error)
	EnrollConnectorUsingTokenHash(ctx context.Context, tokenHash string) (store.Connector, error)

	UpdateConnectorStatus(ctx context.Context, params store.UpdateConnectorStatusParams) (store.Connector, error)

	RevokeConnector(ctx context.Context, params store.RevokeConnectorParams) (store.Connector, error)
}

type postgresRepository struct {
	q *store.Queries
}

func NewPostgresRepository(q *store.Queries) Repository {
	return &postgresRepository{q: q}
}

func (r *postgresRepository) CreateConnector(ctx context.Context, params store.CreateConnectorParams) (store.Connector, error) {
	return r.q.CreateConnector(ctx, params)
}

func (r *postgresRepository) ListConnectorByOrg(ctx context.Context, orgID uuid.UUID) ([]store.Connector, error) {
	return r.q.ListConnectorsByOrg(ctx, orgID)
}

func (r *postgresRepository) ListActiveConnectorsByGateway(ctx context.Context, gatewayID uuid.UUID) ([]store.Connector, error) {
	return r.q.ListActiveConnectorsByGateway(ctx, gatewayID)
}

func (r *postgresRepository) ReCreateConnector(ctx context.Context, params store.ReCreateConnectorParams) (store.Connector, error) {
	return r.q.ReCreateConnector(ctx, params)
}

func (r *postgresRepository) GetConnectorByID(ctx context.Context, id uuid.UUID) (store.Connector, error) {
	return r.q.GetConnectorByID(ctx, id)
}

func (r *postgresRepository) GetConnectorByTokenHash(ctx context.Context, tokenHash string) (store.Connector, error) {
	return r.q.GetConnectorByTokenHash(ctx, tokenHash)
}

func (r *postgresRepository) GetConnectorApps(ctx context.Context, id pgtype.UUID) ([]store.App, error) {
	return r.q.ListAppsByConnector(ctx, id)
}

func (r *postgresRepository) EnrollConnectorUsingTokenHash(ctx context.Context, tokenHash string) (store.Connector, error) {
	return r.q.EnrollConnector(ctx, tokenHash)
}

func (r *postgresRepository) UpdateConnectorStatus(ctx context.Context, params store.UpdateConnectorStatusParams) (store.Connector, error) {
	return r.q.UpdateConnectorStatus(ctx, params)
}

func (r *postgresRepository) RevokeConnector(ctx context.Context, params store.RevokeConnectorParams) (store.Connector, error) {
	return r.q.RevokeConnector(ctx, params)
}

