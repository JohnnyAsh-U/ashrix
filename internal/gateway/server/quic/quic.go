package quic

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/quic-go/quic-go"
	"go.uber.org/zap"
)

type QUICServer struct {
	listener *quic.Listener
	addr     string
	tlsConf  *tls.Config
	log      *zap.Logger
	registry *registry.Registry
}

func NewQUICServer(cfg *config.Config, tlsConfig *tls.Config, log *zap.Logger, registry *registry.Registry) *QUICServer {
	// Enforce mTLS by requiring client certificates
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	return &QUICServer{
		addr:     fmt.Sprintf(":%s", cfg.QUICPort),
		tlsConf:  tlsConfig,
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

	s.log.Info("Gateway QUIC Server starting", zap.String("addr", s.addr))

	for {
		conn, err := s.listener.Accept(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // Clean shutdown via context
			}
			s.log.Error("QUIC accept error", zap.Error(err))
			// Usually accept errors in QUIC are temporary or relate to a specific connection.
			continue
		}

		s.log.Debug("QUIC connection accepted", zap.String("remote_addr", conn.RemoteAddr().String()))

		// TODO: Spawn a goroutine to handle the connection streams
		go s.handleQUICConnection(ctx, conn)
		// _ = conn // suppress unused variable error for now
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
			s.log.Error("panic in handleQUICConnection", zap.Any("panic", r))
			conn.CloseWithError(500, "internal server error due to panic")
		}
	}()

	//mTLS already happened at QUIC handshake
	// Cert tells us connector_id extract from the certs
	connectorID, err := extractConnectorIDFromCert(conn)
	if err != nil {
		s.log.Warn("Quic Connection with unparsable identity - closing", zap.Error(err))
		conn.CloseWithError(1, "Invalid identity")
		return
	}

	//Get the cert from the quic handshake
	tlsState := conn.ConnectionState().TLS
	if len(tlsState.PeerCertificates) == 0 {
		conn.CloseWithError(1, "Invalid identity")
		return
	}
	cert := tlsState.PeerCertificates[0]

	//Check if crl is revoked
	if s.registry.IsCrlRevoked(cert.Subject.SerialNumber) {
		s.log.Warn("Quic tunnel attempted with revoked cert", zap.String("connector_id", connectorID))
		conn.CloseWithError(3, "Certificate is revoked")
		return
	}

	//The connector must already have an active management
	// registration - tunnel cannot attach standalone. This closes the gap from scenario A more strictly than
	// routability alone:
	// we refust to event attach a tunnel for an unknown connector_id, not just refuse to route traffec to it
	if _, exists := s.registry.GetByConnectorID(connectorID); !exists {
		s.log.Warn("QUIC tunnel attempted  before a management registration", zap.String("connector_id", connectorID))
		conn.CloseWithError(2, "register management plane first")
		return
	}

	tunnelSession := &quicTunnelSession{conn: conn}
	s.registry.AttachTunnel(connectorID, tunnelSession, cert, "quic")
	defer s.registry.DetachTunnel(connectorID)

	s.log.Info("Tunnel plane attached via quic", zap.String("Connector_id", connectorID))
	<-conn.Context().Done()

	s.log.Info("Quic tunnel closed", zap.String("connector_id", connectorID))
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
