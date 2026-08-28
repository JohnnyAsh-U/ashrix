package quic

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/router"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/flow"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/quic-go/quic-go"
)

type QUICServer struct {
	listener *quic.Listener
	addr     string
	tlsConf  *tls.Config
	log      *slog.Logger
	registry *registry.Registry
	router   *router.Router
}

func NewQUICServer(cfg *config.Config, tlsConfig *tls.Config, log *slog.Logger, registry *registry.Registry, rtr *router.Router) *QUICServer {
	// Enforce mTLS by requiring client certificates
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	return &QUICServer{
		addr:     fmt.Sprintf(":%s", cfg.QUICPort),
		tlsConf:  tlsConfig,
		log:      log,
		registry: registry,
		router:   rtr,
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

		s.log.Debug("QUIC connection accepted", slog.String("remote_addr", conn.RemoteAddr().String()))

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

	if s.registry.IsCrlRevoked(cert.Subject.SerialNumber) {
		s.log.Warn("Quic tunnel attempted with revoked cert", slog.String("connector_id", connectorID))
		conn.CloseWithError(3, "Certificate is revoked")
		return
	}

	if _, exists := s.registry.GetByConnectorID(connectorID); !exists {
		s.log.Warn("QUIC tunnel attempted before a management registration", slog.String("connector_id", connectorID))
		conn.CloseWithError(2, "register management plane first")
		return
	}

	tunnelSession := &quicTunnelSession{conn: conn}
	s.registry.AttachTunnel(connectorID, tunnelSession, cert, "quic")
	defer s.registry.DetachTunnel(connectorID, tunnelSession)

	s.log.Info("Tunnel plane attached via quic", slog.String("Connector_id", connectorID))

	// Run accept loop for incoming streams from this connector (App -> App flows)
	for {
		stream, err := conn.AcceptStream(conn.Context())
		if err != nil {
			if conn.Context().Err() != nil {
				break
			}
			s.log.Debug("Stop accepting streams from connector (session closed)", slog.String("connector_id", connectorID))
			break
		}

		go s.handleIncomingConnectorStream(conn.Context(), connectorID, conn, stream)
	}
}

func (s *QUICServer) handleIncomingConnectorStream(ctx context.Context, srcConnectorID string, conn *quic.Conn, quicStream *quic.Stream) {
	defer quicStream.Close()

	srcStream := &quicStreamAdapter{stream: quicStream, ctx: ctx}

	payload, err := frame.ReadFrame(srcStream)
	if err != nil {
		s.log.Error("failed to read opening frame from connector", slog.String("connector_id", srcConnectorID), slog.Any("error", err))
		return
	}

	var streamFrame proto.StreamFrame
	if err := frame.DecodeFrame(payload, &streamFrame); err != nil {
		s.log.Error("failed to decode opening frame from connector", slog.String("connector_id", srcConnectorID), slog.Any("error", err))
		return
	}

	startTime := time.Now()

	destStream, err := s.router.Route(ctx, &streamFrame, false)
	if err != nil {
		s.log.Warn("routing/policy rejected for incoming connector stream",
			slog.String("flow_id", streamFrame.RequestId),
			slog.String("src_connector", srcConnectorID),
			slog.String("dest_app", streamFrame.DestAppName),
			slog.Any("error", err),
		)
		rejectFrame := proto.StreamFrame{
			RequestId: streamFrame.RequestId,
			Method:    "OPEN_ERR",
		}
		_ = frame.WriteFrame(srcStream, &rejectFrame)
		return
	}
	defer destStream.Close()

	ackFrame := proto.StreamFrame{
		RequestId: streamFrame.RequestId,
		Method:    "OPEN_OK",
	}
	if err := frame.WriteFrame(srcStream, &ackFrame); err != nil {
		s.log.Error("failed to write ACK frame to source connector", slog.String("flow_id", streamFrame.RequestId), slog.Any("error", err))
		return
	}

	s.log.Info("Logical flow routed successfully, starting Relay",
		slog.String("flow_id", streamFrame.RequestId),
		slog.String("src_connector", srcConnectorID),
		slog.String("dest_app", streamFrame.DestAppId),
	)

	result := flow.Relay(ctx, srcStream, destStream)

	terminationReason := "completed"
	if result.Err != nil {
		terminationReason = result.Err.Error()
	}

	s.log.Info("flow termination audit",
		slog.String("flow_id", streamFrame.RequestId),
		slog.String("flow_type", string(streamFrame.FlowType)),
		slog.String("source_connector", srcConnectorID),
		slog.String("destination_app", streamFrame.DestAppId),
		slog.Duration("duration", time.Since(startTime)),
		slog.Int64("bytes_a_to_b", result.BytesAToB),
		slog.Int64("bytes_b_to_a", result.BytesBToA),
		slog.String("termination_reason", terminationReason),
	)
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
