package cp_grpc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/dispatcher"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/registry"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"

	"github.com/google/uuid"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *cpServer) reconcileGateway(
	ctx context.Context,
	conn *registry.GatewayConn,
) error {

	gatewayID, err :=
		uuid.Parse(conn.GatewayID)

	if err != nil {
		return fmt.Errorf(
			"invalid gateway ID: %w",
			err,
		)
	}

	// --------------------------------------------
	// 1. Get authoritative cursor from CP
	// --------------------------------------------

	lastAck, err :=
		s.gatewayRepo.GetLastAckedSeqForGateway(
			ctx,
			gatewayID,
		)

	if err != nil {

		s.log.Error(
			"failed to get gateway event ACK",
			slog.String(
				"gateway_id",
				conn.GatewayID,
			),
			slog.String(
				"error",
				err.Error(),
			),
		)

		return err
	}

	// --------------------------------------------
	// 2. Get every event after cursor
	// --------------------------------------------

	events, err :=
		s.gatewayRepo.ListGatewayEventsAfter(
			ctx,
			store.ListGatewayEventsAfterParams{
				GatewayID: gatewayID,
				Seq:       lastAck,
			},
		)

	if err != nil {
		return fmt.Errorf(
			"load gateway events: %w",
			err,
		)
	}

	if len(events) == 0 {
		return nil
	}

	// --------------------------------------------
	// 3. Compact complete snapshots
	// --------------------------------------------

	events =
		dispatcher.CompactGatewayEvents(
			events,
		)

	// --------------------------------------------
	// 4. Send in original sequence order
	// --------------------------------------------

	for _, event := range events {

		cmd, err :=
			dispatcher.GatewayEventToCmd(
				event,
			)

		if err != nil {
			return fmt.Errorf(
				"convert event %d: %w",
				event.Seq,
				err,
			)
		}

		envelope := &proto.CPEnvelope{
			Seq: event.Seq,

			EventId: event.EventID.String(),

			SentAt: timestamppb.Now(),

			Payload: &proto.CPEnvelope_Cmd{
				Cmd: cmd,
			},
		}

		if err := conn.Send(envelope); err != nil {
			return fmt.Errorf(
				"send event %d: %w",
				event.Seq,
				err,
			)
		}
	}

	return nil
}