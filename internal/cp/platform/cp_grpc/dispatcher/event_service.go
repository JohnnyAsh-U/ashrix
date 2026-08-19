package dispatcher

import (
	"context"
	"fmt"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type EventService struct {
	queries *store.Queries
}

func NewEventService(
	queries *store.Queries,
) *EventService {
	return &EventService{
		queries: queries,
	}
}

func (s *EventService) CreateEvent(
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

	tx, err := s.queries.NewTx(ctx)
	if err != nil {
		return GatewayEvent{}, err
	}

	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)

	seq, err := q.AllocateGatewayEventSeq(
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

	err = q.InsertGatewayEvent(
		ctx,
		store.InsertGatewayEventParams{
			GatewayID: gatewayID,

			Seq: seq,

			EventID: pgtype.UUID{
				Bytes: eventID,
				Valid: true,
			},

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

	err = q.EnsureGatewayEventsAck(
		ctx,
		gatewayID,
	)

	if err != nil {
		return GatewayEvent{}, fmt.Errorf(
			"ensure gateway ACK: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return GatewayEvent{}, fmt.Errorf(
			"commit gateway event: %w",
			err,
		)
	}

	return GatewayEvent{
		GatewayID:    job.GatewayID,
		Seq:          seq,
		EventID:      eventID.String(),
		Command:      job.Type,
		DeliveryMode: mode,
		Payload:      payload,
	}, nil
}