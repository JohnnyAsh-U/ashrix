package tunnel

import (
	"context"
	"crypto/tls"

	// "crypto/x509"

	// "crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	ashrixerrors "github.com/JohnnyAsh-U/ashrix-api/internal/errors"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/pki"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/quic-go/quic-go"
)

type ConnectorSession struct {
	ConnectorID string
	CertSerial  string
	Conn        *quic.Conn
	// Apps           []*pb.ConnectorApps
	MgmtStream        *quic.Stream
	LastSeen          time.Time
	tunnelAttached    bool
	tunnelConnectedAt time.Time
	ActiveStreams     int64
}

type QuicTunnel struct {
	listener   *quic.Listener
	mu         sync.RWMutex
	connectors map[string]*ConnectorSession // connector_id → entry
	routing    map[string]string            // subdomain → connector_id
	logger     *slog.Logger
}

func NewQuicTunnel(config *config.Config, tlsConfig *tls.Config, logger *slog.Logger) (*QuicTunnel, error) {
	listener, err := quic.ListenAddr(
		config.Listeners.QUIC.Address,
		tlsConfig,
		&quic.Config{
			KeepAlivePeriod:    10 * time.Second,
			MaxIdleTimeout:     30 * time.Second,
			MaxIncomingStreams: 1000,
		},
	)
	if err != nil {
		return nil, err
	}

	// Initialize the connectors
	connectorsList := make(map[string]*ConnectorSession, len(config.Connectors))

	for _, connector := range config.Connectors {
		connectorsList[connector.ID] = &ConnectorSession{
			ConnectorID: connector.ID,
			CertSerial:  connector.CertificateSerial,
		}
	}

	// Initialize the app routing to connector
	appRouting := make(map[string]string, len(config.Applications))

	for _, app := range config.Applications {
		appRouting[app.Domain] = app.Connector
	}

	return &QuicTunnel{
		listener:   listener,
		connectors: connectorsList,
		routing:    appRouting,
		logger:     logger,
	}, nil
}

func (s *QuicTunnel) Start(ctx context.Context) error {
	s.logger.Info("tunnel edge server listening", "addr", s.listener.Addr().String())
	for {
		conn, err := s.listener.Accept(ctx)
		if err != nil {
			return err
		}

		go func(c *quic.Conn) {
			if err := s.handleIncomingConnection(ctx, c); err != nil {
				s.logger.Warn("client session error", "err", err)
			}
		}(conn)
	}
}

func (s *QuicTunnel) handleIncomingConnection(ctx context.Context, conn *quic.Conn) error {
	// 1. Accept first incoming stream as Management Stream
	mgmtStream, err := (*conn).AcceptStream(ctx)
	if err != nil {
		return fmt.Errorf("failed to accept management stream: %w", err)
	}
	defer mgmtStream.Close()

	// Read registration message
	var regMsg proto.ControlMessage
	if err := json.NewDecoder(mgmtStream).Decode(&regMsg); err != nil || regMsg.MsgType != proto.MsgType_REGISTER {
		conn.CloseWithError(quic.ApplicationErrorCode(ashrixerrors.CodeNoCert), "bad register")
		return err
	}

	s.mu.RLock()
	connector, exists := s.connectors[regMsg.ConnectorId]
	s.mu.RUnlock()

	if !exists {
		err := ashrixerrors.UnknownConnector(regMsg.ConnectorId)
		s.logger.Error("unknown connector", "err", err)
		conn.CloseWithError(quic.ApplicationErrorCode(err.Code), err.Msg)
		return err
	}

	if err := s.VerifyConnectorConn(conn, regMsg.ConnectorId, connector.CertSerial); err != nil {
		return err
	}

	entry := s.AttachTunnel(regMsg.ConnectorId, conn, mgmtStream)

	s.logger.Info("connector registered successfully", "connector_id", regMsg.ConnectorId, "apps", regMsg.ConnectorApps)

	defer func() {
		s.DetachTunnel(regMsg.ConnectorId, conn)
		s.logger.Info("connector unregistered", "connector_id", regMsg.ConnectorId)
	}()

	// 2. Read heartbeats on management stream
	decoder := json.NewDecoder(mgmtStream)
	for {
		var msg proto.ControlMessage
		if err := decoder.Decode(&msg); err != nil {
			return err
		}

		if msg.MsgType == proto.MsgType_HEARTBEAT {
			entry.LastSeen = time.Now()
			s.logger.Debug("heartbeat received and acked", "connector_id", entry.ConnectorID)
		}
	}
}

func (r *QuicTunnel) GetConnectorSessionBySubdomain(subdomain string) (*ConnectorSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connectorID, ok := r.routing[subdomain]
	if !ok {
		return nil, false
	}
	entry := r.connectors[connectorID]
	return entry, entry != nil
}

func (e *QuicTunnel) IsConnectorTunnelAttached(connectorID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	entry, ok := e.connectors[connectorID]
	return ok && entry.Conn != nil
}

func (r *QuicTunnel) GetConnectorTunnel(connectorID string) (*quic.Conn, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.connectors[connectorID]
	if !ok || entry.Conn == nil {
		return nil, false
	}
	return entry.Conn, true
}

func (r *QuicTunnel) AttachTunnel(
	connectorID string,
	quicConn *quic.Conn,
	mgmtStream *quic.Stream,
) *ConnectorSession {
	r.mu.Lock()

	entry, ok := r.connectors[connectorID]

	if !ok {
		r.mu.Unlock()
		return nil
	}

	oldConn := entry.Conn
	entry.tunnelConnectedAt = time.Now()
	entry.tunnelAttached = true
	entry.Conn = quicConn
	entry.tunnelAttached = true
	entry.MgmtStream = mgmtStream
	entry.LastSeen = entry.tunnelConnectedAt
	snapshot := *entry
	r.mu.Unlock()

	if oldConn != nil && oldConn != quicConn {
		_ = oldConn.CloseWithError(0, "connector session replaced")
	}

	return &snapshot
}

// DetachTunnel is called when the QUIC connection dies.
func (r *QuicTunnel) DetachTunnel(connectorID string, quicConn *quic.Conn) {
	r.mu.Lock()

	entry, ok := r.connectors[connectorID]
	if !ok || entry.Conn != quicConn {
		// Ignore stale detach from a displaced session
		r.mu.Unlock()
		return
	}

	mgmtStream := entry.MgmtStream
	entry.Conn = nil
	entry.tunnelAttached = false
	entry.MgmtStream = nil
	r.mu.Unlock()
	if mgmtStream != nil {
		_ = mgmtStream.Close()
	}
}

func (s *QuicTunnel) Stop() {
	s.logger.Info("Gateway QUIC Server stopping")
	if s.listener != nil {
		s.listener.Close()
	}
}

func (r *QuicTunnel) VerifyConnectorConn(quicConn *quic.Conn, connectorID string, serialNumber string) error {
	// Get peer cdrt from quic connection
	tlsState := quicConn.ConnectionState().TLS
	if len(tlsState.PeerCertificates) == 0 {
		r.logger.Error("No Client cert on QUIC Conn")
		err := ashrixerrors.NoCert(nil)
		_ = quicConn.CloseWithError(quic.ApplicationErrorCode(err.Code), err.Msg)
		return err
	}

	leaf := tlsState.PeerCertificates[0]
	CertconnectorID := leaf.Subject.CommonName
	if CertconnectorID == "" {
		r.logger.Error("Cert ConnectorID missing")
		err := ashrixerrors.IDMismatch(leaf.Subject.CommonName, connectorID)
		_ = quicConn.CloseWithError(quic.ApplicationErrorCode(err.Code), err.Msg)
		return err
	}

	if connectorID != CertconnectorID {
		r.logger.Error("ConnectorID Mismatch")
		err := ashrixerrors.IDMismatch(leaf.Subject.CommonName, connectorID)
		_ = quicConn.CloseWithError(quic.ApplicationErrorCode(err.Code), err.Msg)
		return err
	}

	if len(leaf.URIs) != 1 || leaf.URIs[0].String() != pki.ConnectorIdentityURI(connectorID) {
		err := ashrixerrors.IDMismatch("certificate URI identity", connectorID)
		_ = quicConn.CloseWithError(quic.ApplicationErrorCode(err.Code), err.Msg)
	}

	// serialNumberNormalized := pki.SerialString(leaf.SerialNumber)

	expectedSerial, err := pki.NormalizeSerial(serialNumber)
	if err != nil {
		serialErr := ashrixerrors.SerialMismatch(serialNumber, pki.SerialString(leaf.SerialNumber))
		_ = quicConn.CloseWithError(quic.ApplicationErrorCode(serialErr.Code), serialErr.Msg)
		return serialErr
	}
	actualSerial, err := pki.NormalizeSerial(pki.SerialString(leaf.SerialNumber))

	if err != nil || actualSerial != expectedSerial {
		r.logger.Error("Connector Serial Number Mismatch")
		serialErr := ashrixerrors.SerialMismatch(pki.SerialString(leaf.SerialNumber), serialNumber)
		_ = quicConn.CloseWithError(quic.ApplicationErrorCode(serialErr.Code), serialErr.Msg)
		return serialErr
	}

	return nil
}
