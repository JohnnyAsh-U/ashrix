package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
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
	log      *zap.Logger
}

func New(
	reg *registry.Registry,
	log *zap.Logger,
) *Server {
	return &Server{
		registry: reg,
		log:      log,
	}
}

// Connect handles one connector's management stream for its entire
// connected lifetime. One goroutine per connector, managed by gRPC.
func (s *Server) Connect(stream gen.ConnectorService_ConnectServer) error {
	// 1. Extract peer cert and gateway ID
	peerInfo, ok := peer.FromContext(stream.Context())
	if !ok {
		return status.Error(codes.Unauthenticated, "no peer info")
	}
	tlsInfo, ok := peerInfo.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return status.Error(codes.Unauthenticated, "no TLS Info")
	}
	if len(tlsInfo.State.PeerCertificates) == 0 {
		return status.Error(codes.Unauthenticated, "no peer certificates")
	}
	cert := tlsInfo.State.PeerCertificates[0]

	//Check if crl is revoked
	if s.registry.IsCrlRevoked(cert.SerialNumber.String()){
		return status.Error(codes.Unauthenticated, "certificate is revoked")
	}


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

	// Check if the connector is in the authorized connectors list
	if _, authorized := s.registry.IsConnectorAuthorized(connectorID); !authorized {
		return status.Error(codes.PermissionDenied, "connector not authorized")
	}

	s.log.Info("connector connecting",
		zap.String("connector_id", connectorID),
	)


	apps := make([]*gen.ConnectorApps, len(hello.Apps))
	for i, a := range hello.Apps {
		apps[i] = &gen.ConnectorApps{
			Id:        a.Id,
			Protocol:  a.Protocol,
			Subdomain: a.Subdomain,
			Name:      a.Name,
			Upstream:  a.Upstream,
			IsPublic:  a.IsPublic,
		}
	}

	//Attach management - does not touch tunnel fields per correct registry
	entry := s.registry.AttachManagement(connectorID, hello.TenantId, apps, stream, cert, "active")
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
