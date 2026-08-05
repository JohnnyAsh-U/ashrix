package gateway_grpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"

	"github.com/cenkalti/backoff/v4"
	// "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StreamManager maintains a single bidi stream to the control plane.
// On any break it reconnects, sends a fresh Hello payload, and resumes.
type StreamManager struct {
	cm        *ConnectionManager
	makeHello func() *pb.GatewayEnvelope // factory: fresh epoch/nonce every reconnect
	// onMessage func(*pb.CPEnvelope)
	onAuthError func(ctx context.Context) error // e.g. pki.PreflightRenew

	pki *crypto.GatewayPKI
	log *zap.Logger

	registry *registry.Registry

	mu     sync.RWMutex
	stream pb.ControlPlaneService_ConnectClient
	active bool
	sendCh chan *pb.GatewayEnvelope
}

func NewStreamManager(
	cm *ConnectionManager,
	makeHello func() *pb.GatewayEnvelope,
	// onMessage func(*pb.CPEnvelope),
	log *zap.Logger,
	sendQueue int,
	onAuthError func(ctx context.Context) error,
) *StreamManager {
	if sendQueue <= 0 {
		sendQueue = 64
	}
	// if log == nil {
	// 	log = zap.NewNop()
	// }
	return &StreamManager{
		cm:        cm,
		makeHello: makeHello,
		// onMessage: onMessage,
		onAuthError: onAuthError,
		log:         log,
		sendCh:      make(chan *pb.GatewayEnvelope, sendQueue),
	}
}

// Run blocks forever. It is the only function that should call runSession.
func (sm *StreamManager) Run(ctx context.Context) {
	b := backoff.NewExponentialBackOff()
	b.MaxInterval = 30 * time.Second
	b.MaxElapsedTime = 0 // retry forever

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := sm.runSession(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			sm.log.Warn("stream session ended", zap.Error(err))
		}

		sm.setStream(nil, false)

		// ── Auth/cert error path ─────────────────────────────────────
		if sm.isAuthError(err) && sm.onAuthError != nil {
			sm.log.Warn("TLS/auth error detected — attempting cert renewal")

			renewCtx, renewCancel := context.WithTimeout(ctx, 30*time.Second)
			renewErr := sm.onAuthError(renewCtx)
			renewCancel()

			if renewErr != nil {
				sm.log.Warn("cert renewal failed — retrying with backoff", zap.Error(renewErr))
			} else {
				sm.log.Info("cert renewed — forcing connection refresh")
				// Tear down the old gRPC connection so the next dial
				// performs a fresh TLS handshake with the new cert.
				if closeErr := sm.cm.Close(); closeErr != nil {
					sm.log.Debug("old conn close error", zap.Error(closeErr))
				}
				if refreshErr := sm.cm.RefreshConnection(ctx); refreshErr != nil {
					sm.log.Error("failed to refresh connection after renewal", zap.Error(refreshErr))
				} else {
					sm.log.Info("reconnected with renewed cert — retrying stream immediately")
					b.Reset()
					continue
				}
			}
		}

		wait := b.NextBackOff()
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

func (sm *StreamManager) runSession(ctx context.Context) error {
	conn := sm.cm.CurrentConn()
	if conn == nil {
		return errors.New("no control plane connection")
	}

	client := pb.NewControlPlaneServiceClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("open bidi stream: %w", err)
	}

	// ── 1. Hello payload (synchronous, must be first) ─────────────
	// The server binds session state (identity, routing, rate-limits)
	// to this specific stream. No other message may precede this.
	if err := stream.Send(sm.makeHello()); err != nil {
		return fmt.Errorf("hello payload: %w", err)
	}

	// ── 2. Publish stream only after hello succeeds ───────────────
	// HTTP handlers may now enqueue messages via Send().
	sm.setStream(stream, true)

	// ── 3. Concurrent send / recv loops ───────────────────────────
	sessCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go sm.heartbeater(ctx)

	errCh := make(chan error, 2)
	go func() { errCh <- sm.sendLoop(sessCtx, stream) }()
	go func() { errCh <- sm.recvLoop(sessCtx, stream) }()

	// If either direction breaks, tear down the entire session.
	err = <-errCh
	cancel()
	<-errCh // drain the other side
	return err
}

func (sm *StreamManager) sendLoop(ctx context.Context, stream pb.ControlPlaneService_ConnectClient) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-sm.sendCh:
			if err := stream.Send(msg); err != nil {
				return err
			}
		}
	}
}

func (sm *StreamManager) recvLoop(ctx context.Context, stream pb.ControlPlaneService_ConnectClient) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		// Normal and HIGH priority handled in a goroutine
		// so the receiver loop never blocks
		go sm.handleMessage(ctx, msg)
	}
}

func (h *StreamManager) heartbeater(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	var seq int64

	for {
		select {
		case <-ticker.C:
			seq++
			h.Send(&pb.GatewayEnvelope{
				Payload: &pb.GatewayEnvelope_Heartbeat{
					Heartbeat: &pb.HeartbeatMessage{
						Seq:               seq,
						ActiveConnections: 3,
						ActiveSessions:    3,
						ConnectorCount:    4,
					},
				},
			})
		// case <-h.stop:
		// 	return
		case <-ctx.Done():
			return
		}
	}
}

func (h *StreamManager) handleMessage(ctx context.Context, msg *pb.CPEnvelope) {
	switch p := msg.Payload.(type) {

	case *pb.CPEnvelope_HelloAck:
		h.log.Info("hello acknowledged by CP",
			zap.String("server_version", p.HelloAck.ServerVersion),
			zap.Bool("needs_policy", p.HelloAck.NeedsPolicy),
		)
		// CP will push bundles immediately if NeedsPolicy/NeedsTrust/NeedsCRL are true

	case *pb.CPEnvelope_PolicyBundle:
		h.log.Info("policy bundle received",
			zap.String("version", "3"),
		)
		// if err := h.pki.ApplyTrustBundle(p.TrustBundle); err != nil {
		//     h.log.Error("failed to apply trust bundle", zap.Error(err))
		//     h.ack(msg.MessageId, "trust", p.TrustBundle.Version, false, err.Error())
		//     return
		// }
		// h.ack("kd", pb.BundleTypes_BUNDLE_TYPE_POLICY,"9", true, "")

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

	default:
		h.log.Debug("unhandled message type", zap.String("type", fmt.Sprintf("%T", p)))

	}
}

func (sm *StreamManager) isAuthError(err error) bool {
	if err == nil {
		return false
	}

	// gRPC status errors
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unauthenticated:
			return true
		case codes.Unavailable:
			// TLS handshake failures often surface as Unavailable
			msg := strings.ToLower(st.Message())
			return strings.Contains(msg, "certificate") ||
				strings.Contains(msg, "tls") ||
				strings.Contains(msg, "handshake") ||
				strings.Contains(msg, "bad certificate")
		}
		return false
	}
	// Raw errors from the transport / crypto/tls
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "certificate") ||
		strings.Contains(errStr, "tls handshake") ||
		strings.Contains(errStr, "bad certificate") ||
		strings.Contains(errStr, "x509")
}

// Send is safe for concurrent HTTP handlers or background goroutines.
// It returns immediately; the message is queued for the next active stream.
// If the queue is full (backpressure or disconnection), it fails fast.
func (sm *StreamManager) Send(msg *pb.GatewayEnvelope) error {
	select {
	case sm.sendCh <- msg:
		return nil
	default:
		return status.Error(codes.ResourceExhausted, "control plane stream backpressure")
	}
}

func (sm *StreamManager) IsActive() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.active
}

func (sm *StreamManager) setStream(s pb.ControlPlaneService_ConnectClient, active bool) {
	sm.mu.Lock()
	sm.stream = s
	sm.active = active
	sm.mu.Unlock()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// func (h *StreamManager) ack(
// 	messageID string, bundleType pb.BundleTypes, version string,
// 	applied bool,
// 	errMsg string,
// ) {
// 	h.Send(&pb.GatewayEnvelope{
// 		Payload: &pb.GatewayEnvelope_BundleAck{
// 			BundleAck: &pb.BundleAck{
// 				MessageId:  messageID,
// 				BundleType: bundleType,
// 				Version:    version,
// 				Applied:    applied,
// 				Error:      errMsg,
// 			},
// 		},
// 	})
// }

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
