package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
)

// CPStateClient is what this server needs to ask CP for a
// connector's authoritative current state at registration time.
// Interface — allows testing without a real CP connection.
type CPStateClient interface {
	GetConnectorState(ctx context.Context, connectorID string) (*ConnectorState, error)
}

type ConnectorState struct {
	State         string // "active" or "suspended"
	SuspendReason string
}


// Server implements the connector-facing management gRPC service.
// Depends on the SAME Registry and PendingCommands instances as
// cpstream.Handler — this is the wiring point.
type Server struct {
	gen.UnimplementedConnectorServiceServer

	registry *registry.Registry
	pending  *registry.PendingCommands
	log      *zap.Logger
}

func New(
	reg *registry.Registry,
	pending *registry.PendingCommands,
	log *zap.Logger,
) *Server {
	return &Server{
		registry: reg,
		pending:  pending,
		log:      log,
	}
}


// Connect handles one connector's management stream for its entire
// connected lifetime. One goroutine per connector, managed by gRPC.
func (s *Server) Connect(stream gen.ConnectorService_ConnectServer) error {
	// ── Registration: first message must be Hello ──────────────────
	envelope, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("recv hello: %w", err)
	}

	hello := envelope.GetHello()
	if hello == nil {
		return fmt.Errorf("first message must be ConnectorHello")
	}

	connectorID := hello.ConnectorId

	s.log.Info("connector connecting",
		zap.String("connector_id", connectorID),
	)

	// TODO: validate hello.Token against CP before proceeding.
	// Skipping here for brevity — this MUST be added before
	// this touches anything with real traffic. An unauthenticated
	// registration here means anyone who can reach this gRPC port
	// can register as any connector_id they choose and receive
	// traffic meant for a real tenant's app. This is a critical gap,
	// not a nice-to-have — flag it in your own tracking.

	// ── Reconciliation: ask CP for current authoritative state ─────
	// This is the pattern from our earlier conversation — never
	// trust local assumptions about whether this connector is
	// suspended. Ask fresh, every single time.
	// ctx := stream.Context()
	// state, bool := s.registry.Get(connectorID)

	apps := make([]*gen.AppDef, len(hello.Apps))
	for i, a := range hello.Apps{
		apps[i] = &gen.AppDef{
			Id: a.Id, Url: a.Addr, Proto: a.Proto, Subdomain: a.Subdomain,
		}
	}

	//Attach management - does not touch tunnel fields per correct registry
	entry := s.registry.AttachManagement(connectorID,apps,stream, "active")
	defer s.registry.DetachManagement(connectorID)


	// Send HelloAck
	if err := stream.Send(&gen.GatewayConnectorEnvelope{
		Payload: &gen.GatewayConnectorEnvelope_HelloAck{
			HelloAck: &gen.ConnectorHelloAck{
				SessionId:     generateSessionID(),
				ServerVersion: "1.0.0",
			},
		},
	}); err != nil {
		return fmt.Errorf("send hello ack: %w", err)
	}


	s.log.Info("Management plane attached", zap.String("Connector_id", connectorID), zap.Bool("Tunnel already attached", entry.TunnelSession != nil))

	// If CP says this connector should be suspended, apply
	// immediately — before accepting any further traffic.
	// if state.State == "suspended" {
	// 	s.log.Warn("connector registering into SUSPENDED state",
	// 		zap.String("connector_id", connectorID),
	// 		zap.String("reason", state.SuspendReason))

	// 	stream.Send(&pb.GatewayEnvelope{
	// 		Payload: &pb.GatewayEnvelope_SuspendCmd{
	// 			SuspendCmd: &pb.SuspendCommand{
	// 				ConnectorId: connectorID,
	// 				Reason:      state.SuspendReason,
	// 			},
	// 		},
	// 	})
	// }

	// ── Drain any pending commands queued while offline ─────────────
	// for _, pendingEnv := range s.pending.Drain(connectorID) {
	// 	if err := stream.Send(pendingEnv); err != nil {
	// 		s.log.Warn("failed to deliver pending command",
	// 			zap.String("connector_id", connectorID),
	// 			zap.Error(err))
	// 	}
	// }

	// s.log.Info("connector registered",
	// 	zap.String("connector_id", connectorID),
	// 	zap.String("state", state.State))

	// ── Normal receive loop ──────────────────────────────────────────
	for {
		env, err := stream.Recv()
		if err != nil {
			s.log.Info("connector disconnected",
				zap.String("connector_id", connectorID),
				zap.Error(err))
			return nil // clean disconnect — not an error worth propagating
		}

		s.handleConnectorMessage(connectorID, env)
	}
}

func (s *Server) handleConnectorMessage(connectorID string, env *gen.ConnectorGatewayEnvelope) {
	switch p := env.Payload.(type) {
	case *gen.ConnectorGatewayEnvelope_Heartbeat:
		s.registry.UpdateHeartbeat(connectorID)
		// all := s.registry.All()
		// fmt.Println(s.registry.GetByConnectorID(connectorID))
		s.log.Debug("heartbeat received",
			zap.String("connector_id", connectorID),
			zap.Int64("seq", p.Heartbeat.Seq))
	default:
		s.log.Debug("unhandled connector message")
	}
}

func generateSessionID() string {
	// Placeholder — use a real UUID library in production
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

