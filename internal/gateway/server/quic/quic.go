package quic

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/router"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/flow"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
	"go.uber.org/zap"
)

type QUICServer struct {
	listener *quic.Listener
	addr     string
	tlsConf  *tls.Config
	log      *zap.Logger
	registry *registry.Registry
	router   *router.Router
}

func NewQUICServer(cfg *config.Config, tlsConfig *tls.Config, log *zap.Logger, registry *registry.Registry, rtr *router.Router) *QUICServer {
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

	s.log.Info("Gateway QUIC Server starting", zap.String("addr", s.addr))

	for {
		conn, err := s.listener.Accept(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // Clean shutdown via context
			}
			s.log.Error("QUIC accept error", zap.Error(err))
			continue
		}

		s.log.Debug("QUIC connection accepted", zap.String("remote_addr", conn.RemoteAddr().String()))

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
			s.log.Error("panic in handleQUICConnection", zap.Any("panic", r))
			conn.CloseWithError(500, "internal server error due to panic")
		}
	}()

	connectorID, err := extractConnectorIDFromCert(conn)
	if err != nil {
		s.log.Warn("Quic Connection with unparsable identity - closing", zap.Error(err))
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
		s.log.Warn("Quic tunnel attempted with revoked cert", zap.String("connector_id", connectorID))
		conn.CloseWithError(3, "Certificate is revoked")
		return
	}

	if _, exists := s.registry.GetByConnectorID(connectorID); !exists {
		s.log.Warn("QUIC tunnel attempted before a management registration", zap.String("connector_id", connectorID))
		conn.CloseWithError(2, "register management plane first")
		return
	}

	tunnelSession := &quicTunnelSession{conn: conn}
	s.registry.AttachTunnel(connectorID, tunnelSession, cert, "quic")
	defer s.registry.DetachTunnel(connectorID, tunnelSession)

	s.log.Info("Tunnel plane attached via quic", zap.String("Connector_id", connectorID))

	// Run accept loop for incoming streams from this connector (App -> App flows)
	for {
		stream, err := conn.AcceptStream(conn.Context())
		if err != nil {
			if conn.Context().Err() != nil {
				break
			}
			s.log.Debug("Stop accepting streams from connector (session closed)", zap.String("connector_id", connectorID))
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
		s.log.Error("failed to read opening frame from connector", zap.String("connector_id", srcConnectorID), zap.Error(err))
		return
	}

	var streamFrame proto.StreamFrame
	if err := frame.DecodeFrame(payload, &streamFrame); err != nil {
		s.log.Error("failed to decode opening frame from connector", zap.String("connector_id", srcConnectorID), zap.Error(err))
		return
	}

	startTime := time.Now()
	flowID := streamFrame.RequestId
	if flowID == "" {
		flowID = uuid.NewString()
	}

	var flowType flow.FlowType
	if streamFrame.FlowType == proto.FlowType_APP_TO_APP {
		flowType = flow.FlowAppToApp
	} else {
		flowType = flow.FlowUserToApp
	}

	// SECURITY RULE: Overwrite/set the source connector ID to the authenticated srcConnectorID!
	req := flow.OpenRequest{
		Version:  1,
		FlowID:   flowID,
		FlowType: flowType,
		Protocol: flow.ProtocolTCP,
		Source: flow.Endpoint{
			Type:        flow.EndpointApp,
			PrincipalID: srcConnectorID, // AUTHORITATIVE identity from cert!
		},
		Destination: flow.Endpoint{
			Type:  flow.EndpointApp,
			AppID: streamFrame.AppId,
		},
		SocksUsername: streamFrame.SocksUsername,
		SocksPassword: streamFrame.SocksPassword,
	}

	destStream, err := s.router.Route(ctx, req)
	if err != nil {
		s.log.Warn("routing/policy rejected for incoming connector stream",
			zap.String("flow_id", flowID),
			zap.String("src_connector", srcConnectorID),
			zap.String("dest_app", streamFrame.AppId),
			zap.Error(err),
		)
		rejectFrame := proto.StreamFrame{
			RequestId: flowID,
			Method:    "OPEN_ERR",
		}
		_ = frame.WriteFrame(srcStream, &rejectFrame)
		return
	}
	defer destStream.Close()

	ackFrame := proto.StreamFrame{
		RequestId: flowID,
		Method:    "OPEN_OK",
	}
	if err := frame.WriteFrame(srcStream, &ackFrame); err != nil {
		s.log.Error("failed to write ACK frame to source connector", zap.String("flow_id", flowID), zap.Error(err))
		return
	}

	s.log.Info("Logical flow routed successfully, starting Relay",
		zap.String("flow_id", flowID),
		zap.String("src_connector", srcConnectorID),
		zap.String("dest_app", streamFrame.AppId),
	)

	result := flow.Relay(ctx, srcStream, destStream)

	terminationReason := "completed"
	if result.Err != nil {
		terminationReason = result.Err.Error()
	}

	s.log.Info("flow termination audit",
		zap.String("flow_id", flowID),
		zap.String("flow_type", string(req.FlowType)),
		zap.String("source_connector", srcConnectorID),
		zap.String("destination_app", streamFrame.AppId),
		zap.Duration("duration", time.Since(startTime)),
		zap.Int64("bytes_a_to_b", result.BytesAToB),
		zap.Int64("bytes_b_to_a", result.BytesBToA),
		zap.String("termination_reason", terminationReason),
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
