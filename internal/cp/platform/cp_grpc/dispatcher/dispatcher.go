package dispatcher

import (
	"context"
	"log/slog"
	"time"

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

type CommandJob struct {
	Type      CommandType
	GatewayID string
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
			d.processJob(job, "")
		}
	}
}

func (d *BoundedDispatcher) processJob(job CommandJob, SessionId string) {
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
				RotateConnectorCert: &gen.RotateConnectorCertCmd{},
			},
		})
	case CmdRevokeConnectorCert:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_RevokeConnectorCert{
				RevokeConnectorCert: &gen.RevokeConnectorCertCmd{},
			},
		})
	case CmdRevokeConnector:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_RevokeConnector{
				RevokeConnector: &gen.RevokeConnectorCmd{},
			},
		})
	case CmdRevokeUserSession:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_RevokeSession{
				RevokeSession: &gen.RevokeSessionCmd{
					SessionId: SessionId,
				},
			},
		})
	case CmdCrlSync:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_CrlSync{
				CrlSync: &gen.CrlSyncCmd{},
			},
		})
	case CmdConnectorSync:
		envelope = buildEnvelope(&gen.Command{
			Payload: &gen.Command_ConnectorSync{
				ConnectorSync: &gen.ConnectorSyncCmd{},
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