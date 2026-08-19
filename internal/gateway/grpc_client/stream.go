package gateway_grpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/version"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	// "github.com/cenkalti/backoff/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// StreamManager maintains a single bidi stream to the control plane.
// On any break it reconnects, sends a fresh Hello payload, and resumes.
type StreamManager struct {
	cm          *ConnectionManager
	policyStore *store.BoltStore
	cfg         *config.Config
	// onAuthError func(ctx context.Context) error // e.g. pki.PreflightRenew

	pki *crypto.GatewayPKI
	log *zap.Logger

	registry    *registry.Registry
	redisClient *redis.Client
	session     *session.SessionManager

	mu     sync.RWMutex
	stream pb.ControlPlaneService_ConnectClient
	active bool
	sendCh chan *pb.GatewayEnvelope
}

func NewStreamManager(
	cm *ConnectionManager,
	policyStore *store.BoltStore,
	cfg *config.Config,
	log *zap.Logger,
	sendQueue int,
	pki *crypto.GatewayPKI,
	// onAuthError func(ctx context.Context) error,
	reg *registry.Registry,
	redisClient *redis.Client,
	session *session.SessionManager,
) *StreamManager {
	if sendQueue <= 0 {
		sendQueue = 64
	}

	return &StreamManager{
		cm:          cm,
		policyStore: policyStore,
		cfg:         cfg,
		pki: pki,
		log:         log,
		sendCh:      make(chan *pb.GatewayEnvelope, sendQueue),
		registry:    reg,
		redisClient: redisClient,
		session:     session,
	}
}

// Run blocks forever. It is the only function that should call runSession.
func (sm *StreamManager) Run(ctx context.Context) {
	// b := backoff.NewExponentialBackOff()
	// b.MaxInterval = 1 * time.Minute
	// b.MaxElapsedTime = 30 * time.Second // retry forever

	// Max times before exit
	// maxRetries := 3
	// const retryInterval = 10 * time.Second
	const (
		retryInterval  = 30 * time.Second
		recoveryWindow = 24 * time.Hour
	)

	recoveryDeadline := time.Now().Add(recoveryWindow)

	for {
		// Don't attempt anything after the recovery window.
		if time.Now().After(recoveryDeadline) {
			sm.log.Error(
				"control plane unavailable for recovery window — shutting down",
				zap.Duration("recovery_window", recoveryWindow),
			)
			return
		}

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

		// fmt.Println(status.FromError(err))

		// ── Auth/cert error path ─────────────────────────────────────
		if sm.isAuthError(err) {
			sm.log.Warn("TLS/auth error detected — attempting cert renewal")
			// renewCtx, renewCancel := context.WithTimeout(ctx, 30*time.Second)
			renewErr := sm.pki.RenewNow()
			// renewCancel()


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
					// b.Reset()
					continue
				}
			}
		}

		// ---------------------------------------------------------
		// Wait before next attempt
		// ---------------------------------------------------------

		remaining := time.Until(recoveryDeadline)

		if remaining <= 0 {
			sm.log.Error(
				"control plane recovery window expired — shutting down",
			)
			return
		}

		wait := min(remaining, retryInterval)

		sm.log.Warn(
			"control plane unavailable — waiting before retry",
			zap.Duration("retry_after", wait),
			zap.Duration("recovery_remaining", remaining),
		)

		timer := time.NewTimer(wait)

		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return

		case <-timer.C:
		}
	}
}

func (sm *StreamManager) runSession(ctx context.Context) error {
	sessCtx, cancel := context.WithCancel(ctx)
	defer cancel()

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
	if err := stream.Send(sm.makeHello(ctx)); err != nil {
		return fmt.Errorf("hello payload: %w", err)
	}

	// ── 2. Publish stream only after hello succeeds ───────────────
	// HTTP handlers may now enqueue messages via Send().
	sm.setStream(stream, true)

	go sm.heartbeater(sessCtx)

	errCh := make(chan error, 2)
	go func() { errCh <- sm.sendLoop(sessCtx, stream) }()

	//Worker Pools => To be implemented
	go func() { errCh <- sm.recvLoop(sessCtx, stream) }()

	// When CP comes up verify the connections
	sm.registry.VerifyGatewayConnections()

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
						GatewayId: h.cfg.GatewayID,
						ActiveConnections: int32(h.registry.ActiveSessions()),
						ActiveSessions:    int32(h.registry.ActiveSessions()),
						ConnectorCount:    int32(h.registry.ActiveConnectors()),
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
		h.log.Info("Hello Acknowledged, Gateway Syncing", zap.Time("server_time", p.HelloAck.ServerTime.AsTime()))

	case *pb.CPEnvelope_PolicyBundle:
		h.log.Info("policy bundle received",
			zap.String("version", "3"),
		)

	case *pb.CPEnvelope_Cmd:
		h.log.Info("command received from CP", zap.String("type", fmt.Sprintf("%T", p.Cmd.Payload)))
		switch cmd := p.Cmd.Payload.(type) {
		case *pb.Command_RevokeSession:
			h.log.Info("revoking session in Redis", zap.String("session_id", cmd.RevokeSession.SessionId))
			if h.redisClient != nil {
				if err := h.session.RevokeByCPSession(ctx, cmd.RevokeSession.SessionId); err != nil{
					h.CommandStatusUpdate(p.Cmd.CmdId, false, err.Error())
				} else {
					h.CommandStatusUpdate(p.Cmd.CmdId, true, "")
				}
			}
			h.log.Info("sent status update", zap.String("session_id", cmd.RevokeSession.SessionId))

		case *pb.Command_RevokeConnector:
			h.log.Info("revoking connector; cutting connection", zap.String("connector_id", cmd.RevokeConnector.ConnectorId))
			h.cutConnectorConnection(cmd.RevokeConnector.ConnectorId)
			h.CommandStatusUpdate(p.Cmd.CmdId, true, "")

		case *pb.Command_RevokeConnectorCert:
			h.log.Info("revoking connector cert; cutting connection", zap.String("connector_id", cmd.RevokeConnectorCert.ConnectorId))
			h.cutConnectorConnection(cmd.RevokeConnectorCert.ConnectorId)
			h.CommandStatusUpdate(p.Cmd.CmdId, true, "")

		case *pb.Command_RotateConnectorCert:
			h.log.Info("rotating connector cert; cutting connection", zap.String("connector_id", cmd.RotateConnectorCert.ConnectorId))
			h.cutConnectorConnection(cmd.RotateConnectorCert.ConnectorId)
			h.CommandStatusUpdate(p.Cmd.CmdId, true, "")

		case *pb.Command_CrlSync:
			h.log.Info("received CRL sync", zap.Int("revoked_certs_count", len(cmd.CrlSync.RevokedSerialNumbers)))
			h.registry.SetCrlEntries(cmd.CrlSync.RevokedSerialNumbers)
			h.CommandStatusUpdate(p.Cmd.CmdId, true, "")

		case *pb.Command_ConnectorSync:
			h.log.Info("received authorized connectors list sync", zap.Int("connectors_count", len(cmd.ConnectorSync.Connectors)))
			statusMap := make(map[string]string)
			for _, c := range cmd.ConnectorSync.Connectors {
				statusMap[c.Id] = c.Status
			}
			h.registry.SetAuthorizedConnectors(statusMap)
			h.CommandStatusUpdate(p.Cmd.CmdId, true, "")


		case *pb.Command_RotateGatewayCert:
			h.log.Warn("gateway certificate rotation requested")
			// if h.onAuthError != nil {
			// 	go func() {
			// 		_ = h.onAuthError(ctx)
			// 	}()
			// }

		case *pb.Command_RevokeGatewayCert:
			h.log.Error("gateway certificate revoked - reconnecting")
			h.CommandStatusUpdate(p.Cmd.CmdId, true, "")
			_ = h.cm.Close()

		case *pb.Command_RevokeGateway:
			h.log.Error("gateway revoked - terminating connection")
			h.CommandStatusUpdate(p.Cmd.CmdId, true, "")
			_ = h.cm.Close()

		case *pb.Command_DrainGateway:
			h.log.Warn("gateway entering draining state")
			h.CommandStatusUpdate(p.Cmd.CmdId, true, "")
		}

	default:
		h.log.Debug("unhandled message type", zap.String("type", fmt.Sprintf("%T", p)))

	}
}

func (h *StreamManager) cutConnectorConnection(connectorID string) {
	if entry, ok := h.registry.GetByConnectorID(connectorID); ok {
		if entry.ManagementSession != nil {
			_ = entry.ManagementSession.Stream.Send(&pb.GatewayConnectorEnvelope{
				Payload: &pb.GatewayConnectorEnvelope_Reject{
					Reject: &pb.ConnectorReject{
						Reason:    "connector revoked or rotated by CP",
						Permanent: true,
					},
				},
			})
			//Close management stream
			h.registry.ForceCloseConnector(connectorID)
		}
		h.registry.DetachTunnel(connectorID, entry.TunnelSession)
		h.registry.DetachManagement(connectorID, entry.ManagementSession)
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

func (h *StreamManager) CommandStatusUpdate(
	cmdID string, 
	hasApplied bool,
	errMsg string,
) {
	h.Send(&pb.GatewayEnvelope{
		Payload: &pb.GatewayEnvelope_CmdAck{
			CmdAck: &pb.CmdAck{
				CmdId:             cmdID,
				GatewayId:         h.cfg.GatewayID,
				Success:           hasApplied,
				Error:             errMsg,
				TimeStamp:         timestamppb.Now(),
			},
		},
	})
}


func (sm *StreamManager) makeHello(ctx context.Context) *pb.GatewayEnvelope {
	policyVersion, err := sm.policyStore.GetCheckpoint(ctx)
	if err != nil {
		sm.log.Error("failed to get policy version", zap.Error(err))
	}

	return &pb.GatewayEnvelope{
		GatewayId: sm.cfg.GatewayID,
		Payload: &pb.GatewayEnvelope_Hello{
			Hello: &pb.HelloMessage{
				GatewayId:     sm.cfg.GatewayID,
				TenantId:      sm.cfg.TenantId,
				PolicyVersion: policyVersion.LastBundleVersion,
				BinaryVersion: version.GetGatewayVersion(),
			},
		},
	}
}
