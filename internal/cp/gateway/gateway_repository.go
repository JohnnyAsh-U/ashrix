package gateway

import (
	"context"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/google/uuid"
)

type Repository interface {
	CreateGateway(ctx context.Context, params store.CreateGatewayParams) (store.Gateway, error)
	ListGatewayByOrg(ctx context.Context, orgID uuid.UUID) ([]store.Gateway, error) // Updated signature
	ListActiveGatewaysByOrg(ctx context.Context, orgID uuid.UUID) ([]store.Gateway, error)
	ReCreateGateway(ctx context.Context, params store.ReCreateGatewayParams) (store.Gateway, error)
	GetGatewayByTokenHash(ctx context.Context, token string) (store.Gateway, error)

	GetActiveGatewayByID(ctx context.Context, id uuid.UUID) (store.Gateway, error)

	GetGatewayByID(ctx context.Context, id uuid.UUID) (store.Gateway, error)
	EnrollGatewayUsingTokenHash(ctx context.Context, tokenHash string) (store.Gateway, error)

	UpdateGatewayHeartBeat(ctx context.Context, id uuid.UUID) (store.Gateway, error)

	UpdateGatewayBinaryVersion(ctx context.Context, params store.UpdateGatewayBinaryVersionParams) (store.Gateway, error)

	RevokeGateway(ctx context.Context, params store.RevokeGatewayParams) (store.Gateway, error)

	UpdateGatewayStatus(ctx context.Context, params store.UpdateGatewayStatusParams) (store.Gateway, error)

	CreateGatewayEvent(ctx context.Context, params store.CreateGatewayEventParams) (store.GatewayEvent, error)

	CreateGatewayEventAck(ctx context.Context, params store.CreateGatewayEventAcksParams) (store.GatewayEventsAck, error)

	GetLastEventSeqForGateway(ctx context.Context, gatewayID uuid.UUID) (int64, error)

	GetLatestGatewayEventsByCommand(ctx context.Context, params store.GetLatestGatewayEventsByCommandParams) ([]store.GatewayEvent, error)

	GetLastAckedSeqForGateway(ctx context.Context, gatewayID uuid.UUID) (int64, error)

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

func (r *postgresRepository) ListActiveGatewaysByOrg(ctx context.Context, orgID uuid.UUID) ([]store.Gateway, error) {
	return r.q.ListActiveGatewaysByOrg(ctx, orgID)
}

func (r *postgresRepository) ReCreateGateway(ctx context.Context, params store.ReCreateGatewayParams) (store.Gateway, error) {
	return r.q.ReCreateGateway(ctx, params)
}





func (r *postgresRepository) GetActiveGatewayByID(ctx context.Context, id uuid.UUID) (store.Gateway, error) {
	return r.q.GetActiveGatewayByID(ctx, id)
}

func (r *postgresRepository) GetGatewayByID(ctx context.Context, id uuid.UUID) (store.Gateway, error) {
	return r.q.GetGatewayByID(ctx, id)
}

func (r *postgresRepository) EnrollGatewayUsingTokenHash(ctx context.Context, tokenHash string) (store.Gateway, error) {
	return r.q.EnrollGateway(ctx, tokenHash)
}


func (r *postgresRepository) GetGatewayByTokenHash(ctx context.Context, token string) (store.Gateway, error) {
	return r.q.GetGatewayByTokenHash(ctx, token)
}

func (r *postgresRepository) UpdateGatewayHeartBeat(ctx context.Context, id uuid.UUID) (store.Gateway, error) {
	return r.q.UpdateGatewayHeartbeat(ctx, id)
}

func (r *postgresRepository) UpdateGatewayBinaryVersion(ctx context.Context, params store.UpdateGatewayBinaryVersionParams) (store.Gateway, error) {
	return r.q.UpdateGatewayBinaryVersion(ctx, params)
}

func (r *postgresRepository) RevokeGateway(ctx context.Context, params store.RevokeGatewayParams) (store.Gateway, error) {
	return r.q.RevokeGateway(ctx, params)
}

func (r *postgresRepository) UpdateGatewayStatus(ctx context.Context, params store.UpdateGatewayStatusParams) (store.Gateway, error) {
	return r.q.UpdateGatewayStatus(ctx, params)
}

func (r *postgresRepository) CreateGatewayEvent(ctx context.Context, params store.CreateGatewayEventParams) (store.GatewayEvent, error) {
	return r.q.CreateGatewayEvent(ctx, params)
}

func (r *postgresRepository) CreateGatewayEventAck(ctx context.Context, params store.CreateGatewayEventAcksParams) (store.GatewayEventsAck, error) {
	return r.q.CreateGatewayEventAcks(ctx, params)
}

func (r *postgresRepository) GetLastEventSeqForGateway(ctx context.Context, gatewayID uuid.UUID) (int64, error) {
	return r.q.GetLastEventSeqForGateway(ctx, gatewayID)
}

func (r *postgresRepository) GetLastAckedSeqForGateway(ctx context.Context, gatewayID uuid.UUID) (int64, error) {
	return r.q.GetLastAckedSeqForGateway(ctx, gatewayID)
}

func (r *postgresRepository) GetLatestGatewayEventsByCommand(ctx context.Context, params store.GetLatestGatewayEventsByCommandParams) ([]store.GatewayEvent, error) {
	return r.q.GetLatestGatewayEventsByCommand(ctx, params)
}


