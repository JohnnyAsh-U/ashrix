package dispatcher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/cp_grpc/registry"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CommandType string

const (
	CmdRotateGatewayCert CommandType = "ROTATE_GATEWAY_CERT"
	CmdRevokeGatewayCert CommandType = "REVOKE_GATEWAY_CERT"
	CmdRevokeGateway     CommandType = "REVOKE_GATEWAY"
	CmdDrainGateway      CommandType = "DRAIN_GATEWAY"

	CmdRotateConnectorCert CommandType = "ROTATE_CONNECTOR_CERT"
	CmdRevokeConnectorCert CommandType = "REVOKE_CONNECTOR_CERT"
	CmdRevokeConnector     CommandType = "REVOKE_CONNECTOR"

	CmdRevokeUserSession CommandType = "REVOKE_USER_SESSION"
	CmdCrlSync           CommandType = "CRL_SYNC"
	CmdConnectorSync     CommandType = "CONNECTOR_SYNC"
)

type DeliveryMode string
const (
	DeliveryAction   DeliveryMode = "ACTION"
	DeliverySnapshot DeliveryMode = "SNAPSHOT"
)

func DeliveryModeForCommand(cmd CommandType) DeliveryMode {
	switch cmd {
	case CmdCrlSync, CmdConnectorSync:
		return DeliverySnapshot
	default:
		return DeliveryAction
	}
}

type CommandJob struct {
	Type CommandType

	GatewayID string

	ConnectorID string

	SessionID string

	RevokedSerialNumbers []string

	ConnectorInfo []*gen.ConnectorInfo
}




type CommandDispatcher interface {
	Dispatch(job CommandJob) bool
	Start(ctx context.Context)
}

type BoundedDispatcher struct {
	registry    *registry.GatewayRegistry
	jobQueue    chan CommandJob
	workerCount int
	logger      *slog.Logger
}

func NewBoundedDispatcher(reg *registry.GatewayRegistry, queueSize, workerCount int, logger *slog.Logger) *BoundedDispatcher {
	return &BoundedDispatcher{
		registry:    reg,
		jobQueue:    make(chan CommandJob, queueSize),
		workerCount: workerCount,
		logger:      logger,
	}
}

// Start initializes the fixed worker pool. Run this on application startup.
func (d *BoundedDispatcher) Start(ctx context.Context) {
	for i := 0; i < d.workerCount; i++ {
		workerID := i
		go d.worker(ctx, workerID)
	}
}

// Dispatch non-blockingly attempts to enqueue a job.
// Returns false if the queue is at full capacity (backpressure handling).
func (d *BoundedDispatcher) Dispatch(job CommandJob) bool {
	select {
	case d.jobQueue <- job:
		return true
	default:
		d.logger.Warn("Command queue full, dropping command",
			"cmd", job.Type,
			"gateway_id", job.GatewayID,
		)
		return false
	}
}

func (d *BoundedDispatcher) worker(ctx context.Context, id int) {
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-d.jobQueue:
			if !ok {
				return
			}
			d.processJob(job)
		}
	}
}

func (d *BoundedDispatcher) processJob(job CommandJob) {
	conn, exists := d.registry.GetConnection(job.GatewayID)
	if !exists {
		return
	}

	// Create an isolated context to bound delivery execution time
	execCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var envelope *gen.CPEnvelope

	switch job.Type {
	case CmdRotateGatewayCert:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_RotateGatewayCert{
				RotateGatewayCert: &gen.RotateGatewayCertCmd{},
			},
		})
	case CmdRevokeGatewayCert:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_RevokeGatewayCert{
				RevokeGatewayCert: &gen.RevokeGatewayCertCmd{},
			},
		})
	case CmdRevokeGateway:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_RevokeGateway{
				RevokeGateway: &gen.RevokeGatewayCmd{},
			},
		})
	case CmdDrainGateway:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_DrainGateway{
				DrainGateway: &gen.DrainGatewayCmd{},
			},
		})
	case CmdRotateConnectorCert:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_RotateConnectorCert{
				RotateConnectorCert: &gen.RotateConnectorCertCmd{
					ConnectorId: job.ConnectorID,
				},
			},
		})
	case CmdRevokeConnectorCert:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_RevokeConnectorCert{
				RevokeConnectorCert: &gen.RevokeConnectorCertCmd{
					ConnectorId: job.ConnectorID,
				},
			},
		})
	case CmdRevokeConnector:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_RevokeConnector{
				RevokeConnector: &gen.RevokeConnectorCmd{
					ConnectorId: job.ConnectorID,
				},
			},
		})
	case CmdRevokeUserSession:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_RevokeSession{
				RevokeSession: &gen.RevokeSessionCmd{
					SessionId: job.SessionID,
				},
			},
		})
	case CmdCrlSync:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_CrlSync{
				CrlSync: &gen.CrlSyncCmd{
					RevokedSerialNumbers: job.RevokedSerialNumbers,
				},
			},
		})
	case CmdConnectorSync:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_ConnectorSync{
				ConnectorSync: &gen.ConnectorSyncCmd{
					Connectors: job.ConnectorInfo,
				},
			},
		})
	}

	if err := conn.SendWithTimeout(execCtx, envelope); err != nil {
		d.logger.Error("Failed to deliver command to gateway",
			"gateway_id", job.GatewayID,
			"error", err,
		)
	}

	if job.Type == CmdRevokeGateway {
		conn.Cancel()
	}
}

func buildEnvelope(cmdPayload *gen.Command) *gen.CPEnvelope {
	return &gen.CPEnvelope{
		SentAt: timestamppb.Now(),
		Payload: &gen.CPEnvelope_Cmd{
			Cmd: cmdPayload,
		},
	}
}


func SendEventAndWaitAck(
	ctx context.Context,
	conn *registry.GatewayConn,
	event store.GatewayEvent,
) error {

	cmd, err :=
		GatewayEventToCmd(event)

	if err != nil {
		return err
	}

	ackCh :=
		conn.RegisterAck(event.Seq)

	envelope := &gen.CPEnvelope{
		// Seq: event.Seq,

		// EventId: event.EventID.String(),

		SentAt: timestamppb.Now(),

		Payload: &gen.CPEnvelope_Cmd{
			Cmd: cmd,
		},
	}

	if err := conn.Send(envelope); err != nil {

		conn.ResolveAck(
			event.Seq,
			err,
		)

		return fmt.Errorf(
			"send event %d: %w",
			event.Seq,
			err,
		)
	}

	select {

	case err := <-ackCh:
		return err

	case <-ctx.Done():
		return ctx.Err()
	}
}