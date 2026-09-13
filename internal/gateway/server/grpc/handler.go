package grpc

import (
	"context"
	"crypto/x509"
	"fmt"
	"log/slog"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
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

type recvResult struct {
	env *gen.ConnectorGatewayEnvelope
	err error
}

// Server implements the connector-facing management gRPC service.
// Depends on the SAME Registry and PendingCommands instances as
// cpstream.Handler — this is the wiring point.
type Server struct {
	gen.UnimplementedConnectorServiceServer

	registry *registry.Registry
	log      *slog.Logger
}

func New(
	reg *registry.Registry,
	log *slog.Logger,
) *Server {
	return &Server{
		registry: reg,
		log:      log,
	}
}

// Connect handles one connector's management stream for its entire
// connected lifetime. One goroutine per connector, managed by gRPC.
func (s *Server) Connect(stream gen.ConnectorService_ConnectServer) error {
	ctx, cancel := context.WithCancel(stream.Context())
    defer cancel()
	
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

	// 3. HARD SECURITY CHECK: Bind cert identity (SAN or CN) to requested connector_id
	certConnectorID, err := extractConnectorIDFromCert(cert)
	if err != nil {
		s.log.Warn("invalid certificate identity structure", slog.Any("error", err))
		return status.Error(codes.Unauthenticated, "failed to parse certificate identity")
	}

	if certConnectorID != connectorID {
		s.log.Error("mTLS identity spoofing attempt blocked",
			slog.String("hello_connector_id", connectorID),
			slog.String("cert_connector_id", certConnectorID),
		)
		return status.Error(codes.PermissionDenied, "mTLS identity mismatch")
	}

	// Check if the connector is in the authorized connectors list
	if _, authorized := s.registry.IsConnectorAuthorized(connectorID); !authorized {
		return status.Error(codes.PermissionDenied, "connector not authorized")
	}

	session := &registry.ManagementSession{
		Stream: stream,
		Cancel: cancel,
	}

	s.log.Info("connector connecting",
		slog.String("connector_id", connectorID),
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
	entry := s.registry.AttachManagement(connectorID, hello.TenantId, apps, session, cert, "active")
	defer s.registry.DetachManagement(connectorID, session)


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

	s.log.Info("Management plane attached", slog.String("Connector_id", connectorID), slog.Bool("Tunnel already attached", entry.TunnelSession != nil))

	// 6. Spawn dedicated receive worker to keep stream.Recv() from blocking context cancellation
	msgChan := make(chan recvResult, 1)
	go func() {
		for {
			env, err := stream.Recv()
			select {
			case msgChan <- recvResult{env: env, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	// ── Normal receive loop ──────────────────────────────────────────
	// 7. Main Event Loop
	for {
		select {
		case <-ctx.Done():
			s.log.Info("Management stream context terminated", slog.String("connector_id", connectorID))
			return status.Error(codes.Canceled, "connection terminated by gateway")

		case res := <-msgChan:
			if res.err != nil {
				s.log.Info("Connector network stream closed",
					slog.String("connector_id", connectorID),
					slog.Any("err",res.err),
				)
				return nil
			}
			s.handleConnectorMessage(connectorID, res.env)
		}
	}
}

func (s *Server) handleConnectorMessage(connectorID string, env *gen.ConnectorGatewayEnvelope) {
	switch p := env.Payload.(type) {
	case *gen.ConnectorGatewayEnvelope_Heartbeat:
		if p.Heartbeat != nil && len(p.Heartbeat.AppHealth) > 0 {
			s.registry.UpdateAppHealth(connectorID, p.Heartbeat.AppHealth, p.Heartbeat.ActiveStreams)
		} else {
			s.registry.UpdateHeartbeat(connectorID, p.Heartbeat.ActiveStreams)
		}
		s.log.Debug("heartbeat received",
			slog.String("connector_id", connectorID),
			slog.Int64("seq", p.Heartbeat.Seq),
			slog.Int64("activeStreams", p.Heartbeat.ActiveStreams),
		)
	default:
		s.log.Debug("unhandled connector message")
	}
}

func generateSessionID() string {
	// Placeholder — use a real UUID library in production
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

func extractConnectorIDFromCert(cert *x509.Certificate) (string, error) {
	if cert.Subject.CommonName != "" {
		return cert.Subject.CommonName, nil
	}
	if len(cert.DNSNames) > 0 {
		return cert.DNSNames[0], nil
	}
	return "", fmt.Errorf("no valid CommonName or SAN DNSName found in certificate")
}