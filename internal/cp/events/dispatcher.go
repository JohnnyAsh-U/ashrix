package events

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/registry"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GatewayDispatcher struct {
	repo     Repository
	registry *registry.GatewayRegistry

	wakeMu sync.Mutex

	wakeups map[string]chan struct{}

	logger *slog.Logger
}

func NewGatewayDispatcher(
	repo Repository,
	reg *registry.GatewayRegistry,
	logger *slog.Logger,
) *GatewayDispatcher {

	return &GatewayDispatcher{
		repo:     repo,
		registry: reg,
		logger:   logger,
		wakeups: make(
			map[string]chan struct{},
		),
	}
}

func (d *GatewayDispatcher) Wakeup(
	gatewayID string,
) {

	d.wakeMu.Lock()

	ch, exists := d.wakeups[gatewayID]

	if !exists {
		ch = make(chan struct{}, 1)
		d.wakeups[gatewayID] = ch

		go d.worker(gatewayID, ch)
	}

	d.wakeMu.Unlock()

	select {
	case ch <- struct{}{}:
	default:
	}
}

func (d *GatewayDispatcher) worker(
	gatewayID string,
	wakeup <-chan struct{},
) {

	for range wakeup {

		conn, exists :=
			d.registry.GetConnection(
				gatewayID,
			)

		if !exists {
			continue
		}

		ctx, cancel :=
			context.WithTimeout(
				context.Background(),
				30*time.Second,
			)
		err := d.deliverPending(ctx, conn)

		cancel()

		if err != nil {

			d.logger.Error(
				"gateway event delivery failed",
				slog.String(
					"gateway_id",
					gatewayID,
				),
				slog.String(
					"error",
					err.Error(),
				),
			)
		}
	}
}

func (d *GatewayDispatcher) deliverPending(
	ctx context.Context,
	conn *registry.GatewayConn,
) error {

	gatewayID, err :=
		uuid.Parse(conn.GatewayID)

	if err != nil {
		return err
	}

	lastAck, err :=
		d.repo.GetLastAckedSeqForGateway(
			ctx,
			gatewayID,
		)
	if err != nil {
		return err
	}

	events, err :=
		d.repo.ListGatewayEventsAfter(
			ctx,
			store.ListGatewayEventsAfterParams{
				GatewayID: gatewayID,
				Seq:       lastAck,
			},
		)

	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}
	events = d.repo.CompactGatewayEvents(events)

	for _, event := range events {

		cmd, err := GatewayEventToCmd(event)

		if err != nil {
			return err
		}

		cmd.Seq = event.Seq
		cmd.EventId = event.EventID.String()

		envelope :=
			&proto.CPEnvelope{
				SentAt:  timestamppb.Now(),
				Payload: &proto.CPEnvelope_Cmd{
					Cmd: cmd,
				},
			}

		if err := conn.Send(
			envelope,
		); err != nil {
			return err
		}
	}

	return nil
}

// job := dispatcher.CommandJob{
// 	Type:       dispatcher.CmdRevokeConnector,
// 	GatewayID:  gatewayID,
// 	ConnectorID: connectorID,
// }

// event, err :=
// 	eventService.CreateEvent(
// 		ctx,
// 		job,
// 	)

// if err != nil {
// 	return err
// }

// gatewayDispatcher.Wakeup(
// 	gatewayID,
// )
