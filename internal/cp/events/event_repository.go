package events

import (
	"context"
	"fmt"

	// "fmt"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/google/uuid"

	// "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// -----------------------------------------------------------
// REPOSITORY INTERFACE
// -----------------------------------------------------------

type Repository interface {
	CreateEvent(ctx context.Context, job CommandJob) (GatewayEvent, error)
	GetLatestGatewayEventSeq(ctx context.Context, gatewayID uuid.UUID) (int64, error)
	GetLastAckedSeqForGateway(ctx context.Context, gatewayID uuid.UUID) (int64, error)
	AckGatewayEvents(ctx context.Context, params store.AckGatewayEventsParams) error
	ListGatewayEventsAfter(ctx context.Context, params store.ListGatewayEventsAfterParams) ([]store.GatewayEvent, error)
	CompactGatewayEvents(params []store.GatewayEvent) []store.GatewayEvent
}


type postgresRepository struct {
	db      *pgxpool.Pool
	queries *store.Queries
}

func NewRepository(queries *store.Queries, db *pgxpool.Pool ) Repository {
	return &postgresRepository{
		queries: queries,
		db:      db,
	}
}

func (s *postgresRepository) CreateEvent(
	ctx context.Context,
	job CommandJob,
) (GatewayEvent, error) {

	gatewayID, err := uuid.Parse(job.GatewayID)
	if err != nil {
		return GatewayEvent{}, fmt.Errorf(
			"invalid gateway ID: %w",
			err,
		)
	}

	payload, err := MarshalCommand(job)
	if err != nil {
		return GatewayEvent{}, err
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return GatewayEvent{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// qtx := r.queries.WithTx(tx)

	qtx := s.queries.WithTx(tx)
	
	seq, err := qtx.AllocateGatewayEventSeq(
		ctx,
		gatewayID,
	)
	if err != nil {
		return GatewayEvent{}, fmt.Errorf(
			"allocate gateway event sequence: %w",
			err,
		)
	}

	eventID := uuid.New()

	mode := DeliveryModeForCommand(job.Type)

	err = qtx.InsertGatewayEvent(
		ctx,
		store.InsertGatewayEventParams{
			GatewayID: gatewayID,

			Seq: int64(seq),

			EventID: eventID,

			Command: string(job.Type),

			DeliveryMode: string(mode),

			Payload: payload,
		},
	)

	if err != nil {
		return GatewayEvent{}, fmt.Errorf(
			"insert gateway event: %w",
			err,
		)
	}

	// err = q.EnsureGatewayEventsAck(
	// 	ctx,
	// 	gatewayID,
	// )

	// if err != nil {
	// 	return GatewayEvent{}, fmt.Errorf(
	// 		"ensure gateway ACK: %w",
	// 		err,
	// 	)
	// }

	if err := tx.Commit(ctx); err != nil {
		return GatewayEvent{}, fmt.Errorf(
			"commit gateway event: %w",
			err,
		)
	}

	return GatewayEvent{
		GatewayID:    job.GatewayID,
		Seq:          int64(seq),
		EventID:      eventID.String(),
		Command:      job.Type,
		DeliveryMode: mode,
		Payload:      payload,
	}, nil
}

func (s *postgresRepository) GetLatestGatewayEventSeq(ctx context.Context, gatewayID uuid.UUID) (int64, error) {
	return s.queries.GetLatestGatewayEventSeq(ctx, gatewayID)
}

func (s *postgresRepository) GetLastAckedSeqForGateway(ctx context.Context, gatewayID uuid.UUID) (int64, error) {
	return s.queries.GetLastAckedSeqForGateway(ctx, gatewayID)
}

func (s *postgresRepository) AckGatewayEvents(ctx context.Context, params store.AckGatewayEventsParams) error {
	return s.queries.AckGatewayEvents(ctx, params)
}

func (s *postgresRepository) ListGatewayEventsAfter(ctx context.Context, params store.ListGatewayEventsAfterParams) ([]store.GatewayEvent, error) {
	return s.queries.ListGatewayEventsAfter(ctx, params)
}

func (s *postgresRepository) CompactGatewayEvents(events []store.GatewayEvent) []store.GatewayEvent {

	if len(events) <= 1 {
		return events
	}

	latest := make(map[string]int)

	for i, event := range events {

		if event.DeliveryMode !=
			string(DeliverySnapshot) {

			continue
		}

		latest[event.Command] = i
	}

	result := make(
		[]store.GatewayEvent,
		0,
		len(events),
	)

	for i, event := range events {

		if event.DeliveryMode ==
			string(DeliverySnapshot) {

			latestIndex, ok :=
				latest[event.Command]

			if ok && latestIndex != i {
				continue
			}
		}

		result = append(result, event)
	}

	return result
}
