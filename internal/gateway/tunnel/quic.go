package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/quic-go/quic-go"
)

type QUICServer struct {
	listener *quic.Listener
	addr     string
	tlsConf  *tls.Config
	log      *slog.Logger
	registry *Registry
}

func NewQUICServer(cfg *config.Config, tlsConfig *tls.Config, log *slog.Logger, registry *Registry) *QUICServer {

	quicTLS := tlsConfig.Clone()

	// Connector authentication.
	quicTLS.ClientAuth =
		tls.RequireAndVerifyClientCert

	// QUIC has its own ALPN.
	quicTLS.NextProtos = []string{
		"ashrix-quic-v1",
	}

	return &QUICServer{
		addr:     fmt.Sprintf("%s", cfg.Listeners.QUIC.Address),
		tlsConf:  quicTLS,
		log:      log,
		registry: registry,
	}
}

func (s *QUICServer) Start(ctx context.Context) error {
	quicConfig := &quic.Config{
		KeepAlivePeriod:    15 * time.Second,
		MaxIdleTimeout:     30 * time.Second,
		MaxIncomingStreams: 1000,
	}
	listener, err := quic.ListenAddr(s.addr, s.tlsConf, quicConfig)
	if err != nil {
		return fmt.Errorf("failed to listen on QUIC port %s: %w", s.addr, err)
	}
	s.listener = listener

	s.log.Info("Gateway QUIC Server starting", slog.String("addr", s.addr))

	for {
		conn, err := s.listener.Accept(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // Clean shutdown via context
			}
			s.log.Error("QUIC accept error", slog.Any("error", err))
			continue
		}

		s.log.Info("QUIC connection accepted", slog.String("remote_addr", conn.RemoteAddr().String()))

		go s.handleQUICConnection(ctx, conn)
	}
}

func (s *QUICServer) Stop() {
	s.log.Info("Gateway QUIC Server stopping")
	if s.listener != nil {
		s.listener.Close()
	}
}

func (s *QUICServer) handleQUICConnection(ctx context.Context, conn *quic.Conn) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("panic in handleQUICConnection", slog.Any("panic", r))
			conn.CloseWithError(500, "internal server error due to panic")
		}
	}()

	connectorID, err := extractConnectorIDFromCert(conn)
	if err != nil {
		s.log.Warn("Quic Connection with unparsable identity - closing", slog.Any("error", err))
		conn.CloseWithError(1, "Invalid identity")
		return
	}

	tlsState := conn.ConnectionState().TLS
	if len(tlsState.PeerCertificates) == 0 {
		conn.CloseWithError(1, "Invalid identity")
		return
	}
	cert := tlsState.PeerCertificates[0]


	tunnelSession := &quicTunnelSession{conn: conn}
	s.registry.AttachTunnel(connectorID, tunnelSession, cert, "quic", []*proto.ConnectorApps{{Id: "s3",Name: "s3",Upstream: "localhost:9000"}})
	defer s.registry.DetachTunnel(connectorID, tunnelSession)

	s.log.Info("Tunnel plane attached via quic", slog.String("Connector_id", connectorID))

	<-ctx.Done()

	s.log.Info("QUIC Tunnel plane detached", slog.String("Connector_id", connectorID))
}


func extractConnectorIDFromCert(conn *quic.Conn) (string, error) {
	tlsState := conn.ConnectionState().TLS
	if len(tlsState.PeerCertificates) == 0 {
		return "", fmt.Errorf("no peer certificates found")
	}
	for _, cert := range tlsState.PeerCertificates {
		if connectorID := cert.Subject.CommonName; connectorID != "" {
			return connectorID, nil
		}
	}
	return "", fmt.Errorf("peer certificate missing CommonName")
}
