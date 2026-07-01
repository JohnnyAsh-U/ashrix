package gateway_grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type StreamHandler struct {
	gatewayID string
	stream    proto.ControlPlaneService_ConnectClient
	pki       *crypto.GatewayPKI
	log       *zap.Logger

	registry *registry.Registry

	//outbound queue - everything gateway sends to CP
	outbound chan *proto.GatewayEnvelope
	stopCh   chan struct{}
}

func NewStreamHandler(
	gatewayID string,
	stream proto.ControlPlaneService_ConnectClient,
	pki *crypto.GatewayPKI,
	reg *registry.Registry,
	log *zap.Logger,
) *StreamHandler {
	return &StreamHandler{
		gatewayID: gatewayID,
		stream:    stream,
		pki:       pki,
		registry: reg,
		log:       log,
		outbound:  make(chan *proto.GatewayEnvelope, 512),
		stopCh:    make(chan struct{}),
	}
}

func (h *StreamHandler) Run(ctx context.Context) error {
	// Start sender goroutine — reads from outbound channel, writes to stream
	go h.sender(ctx)

	// // Start heartbeat goroutine
	go h.heartbeater(ctx)

	// // Receive loop — this is the main goroutine
	return h.receiver(ctx)
}

func (h *StreamHandler) sender(ctx context.Context) {
	for {
		select {
		case env := <-h.outbound:
			env.GatewayId = h.gatewayID
			env.SentAt = timestamppb.Now()

			if err := h.stream.Send(env); err != nil {
				h.log.Error("stream send error", zap.Error(err))
				return
			}
		case <-h.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Send enqueues a message to CP. Non-blocking — drops if queue is full.
// Use for access logs and metrics. For critical messages use SendPriority.
func (h *StreamHandler) Send(env *proto.GatewayEnvelope) {
	select {
	case h.outbound <- env:
	default:
		h.log.Warn("outbound queue full — dropping message",
			zap.String("type", fmt.Sprintf("%T", env.Payload)),
		)
	}
}

// ── Receiver — CP → Gateway ───────────────────────────────────────────────────

func (h *StreamHandler) receiver(ctx context.Context) error {
	for {
		msg, err := h.stream.Recv()
		if err != nil {
			return fmt.Errorf("stream recv error: %w", err)
		}

		// Normal and HIGH priority handled in a goroutine
		// so the receiver loop never blocks
		go h.handleMessage(ctx, msg)
	}
}

func (h *StreamHandler) handleMessage(ctx context.Context, msg *proto.CPEnvelope) {
	switch p := msg.Payload.(type) {

	case *proto.CPEnvelope_HelloAck:
		h.log.Info("hello acknowledged by CP",
			zap.String("server_version", p.HelloAck.ServerVersion),
			zap.Bool("needs_policy", p.HelloAck.NeedsPolicy),
		)
		// CP will push bundles immediately if NeedsPolicy/NeedsTrust/NeedsCRL are true

	case *proto.CPEnvelope_TrustBundle:
		h.log.Info("trust bundle received",
			zap.String("version", p.TrustBundle.Version),
		)
		// if err := h.pki.ApplyTrustBundle(p.TrustBundle); err != nil {
		//     h.log.Error("failed to apply trust bundle", zap.Error(err))
		//     h.ack(msg.MessageId, "trust", p.TrustBundle.Version, false, err.Error())
		//     return
		// }
		h.ack(msg.NodeId, proto.BundleType_BUNDLE_TYPE_POLICY, p.TrustBundle.Version, true, "")

		// case *pb.CPEnvelope_RotationCmd:
		//     h.log.Info("rotation command received",
		//         zap.String("reason", p.RotationCmd.Reason),
		//         zap.Time("rotate_by", p.RotationCmd.RotateBy.AsTime()),
		//     )
		//     if err := h.pki.VerifySignature(p.RotationCmd.Signature, p.RotationCmd); err != nil {
		//         h.log.Error("rotation command signature invalid — ignoring",
		//             zap.Error(err),
		//         )
		//         return
		//     }
		//     go func() {
		//         if err := h.pki.RenewNow(); err != nil {
		//             h.log.Error("forced rotation failed", zap.Error(err))
		//         }
		//     }()
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (h *StreamHandler) ack(
	messageID string, bundleType proto.BundleType, version string,
	applied bool,
	errMsg string,
) {
	h.Send(&proto.GatewayEnvelope{
		Payload: &proto.GatewayEnvelope_BundleAck{
			BundleAck: &proto.BundleAck{
				MessageId:  messageID,
				BundleType: bundleType,
				Version:    version,
				Applied:    applied,
				Error:      errMsg,
			},
		},
	})
}

// ── Heartbeat ─────────────────────────────────────────────────────────────────

func (h *StreamHandler) heartbeater(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	var seq int64

	for {
		select {
		case <-ticker.C:
			seq++
			h.Send(&proto.GatewayEnvelope{
				Payload: &proto.GatewayEnvelope_Heartbeat{
					Heartbeat: &proto.HeartbeatMessage{
						Seq:               seq,
						ActiveConnections: 3,
						ActiveSessions:    3,
						ConnectorCount:    4,
					},
				},
			})
		case <-h.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}




// // handleSuspendCommand is the exact flow traced in our conversation:
// // look up in shared registry, relay if connected, otherwise no-op
// // (reconciliation on reconnect handles the offline case correctly).
// func (h *Handler) handleSuspendCommand(cmd *pb.SuspendCommand) {
// 	if err := h.pki.VerifySignature(cmd.Signature, cmd); err != nil {
// 		h.log.Error("suspend command signature invalid — ignoring",
// 			zap.Error(err))
// 		return
// 	}

// 	connectorID := cmd.ConnectorId

// 	// Update registry's view of truth regardless of connection state.
// 	// This matters for policy checks even if relay fails below.
// 	h.registry.SetState(connectorID, "suspended")

// 	entry, connected := h.registry.GetByConnectorID(connectorID)
// 	if !connected {
// 		h.log.Info("connector offline — state recorded, will apply on reconnect",
// 			zap.String("connector_id", connectorID))

// 		// Best-effort: queue it too, in case connector reconnects
// 		// within this gateway's lifetime before a full reconciliation
// 		// cycle would otherwise catch it.
// 		h.pending.Enqueue(connectorID, &pb.GatewayEnvelope{
// 			Payload: &pb.GatewayEnvelope_SuspendCmd{SuspendCmd: cmd},
// 		})
// 		return
// 	}

// 	err := entry.ManagementStream.Send(&pb.GatewayEnvelope{
// 		Payload: &pb.GatewayEnvelope_SuspendCmd{SuspendCmd: cmd},
// 	})
// 	if err != nil {
// 		h.log.Error("failed to relay suspend to connector",
// 			zap.String("connector_id", connectorID),
// 			zap.Error(err))
// 		// Stream write failed — connector's stream is likely dying.
// 		// Do NOT retry here. Let the connector server's own recv
// 		// loop detect the dead stream, unregister, and let the
// 		// connector's natural reconnect trigger reconciliation.
// 		return
// 	}

// 	h.log.Info("suspend command relayed",
// 		zap.String("connector_id", connectorID))
// }

// func (h *Handler) handleResumeCommand(cmd *pb.ResumeCommand) {
// 	if err := h.pki.VerifySignature(cmd.Signature, cmd); err != nil {
// 		h.log.Error("resume command signature invalid — ignoring",
// 			zap.Error(err))
// 		return
// 	}

// 	connectorID := cmd.ConnectorId
// 	h.registry.SetState(connectorID, "active")

// 	entry, connected := h.registry.GetByConnectorID(connectorID)
// 	if !connected {
// 		h.pending.Enqueue(connectorID, &pb.GatewayEnvelope{
// 			Payload: &pb.GatewayEnvelope_ResumeCmd{ResumeCmd: cmd},
// 		})
// 		return
// 	}

// 	if err := entry.ManagementStream.Send(&pb.GatewayEnvelope{
// 		Payload: &pb.GatewayEnvelope_ResumeCmd{ResumeCmd: cmd},
// 	}); err != nil {
// 		h.log.Error("failed to relay resume to connector", zap.Error(err))
// 	}
// }


