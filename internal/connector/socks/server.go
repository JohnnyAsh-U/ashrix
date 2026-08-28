package socks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/flow"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
	"log/slog"
)

type Identity struct {
	AppID string
}

type OpenRequest struct {
	Identity    Identity
	Destination ConnectRequest
}

type TunnelStream interface {
	io.ReadWriteCloser
}

type Tunnel interface {
	Open(
		ctx context.Context,
		req OpenRequest,
	) (TunnelStream, error)
}

const (
	DefaultAuthTimeout = 10 * time.Second
	DefaultIdleTimeout = 30 * time.Minute
)

type Server struct {
	Addr string

	Credentials *CredentialStore
	Tunnel      transport.Session

	Logger *slog.Logger

	AuthTimeout time.Duration
	IdleTimeout time.Duration

	wg sync.WaitGroup
}

func NewServer(addr string, credentials *CredentialStore, tunnel transport.Session, logger *slog.Logger) *Server {
	return &Server{
		Addr:        addr,
		Credentials: credentials,
		Tunnel:      tunnel,
		Logger:      logger,
		AuthTimeout: DefaultAuthTimeout,
		IdleTimeout: DefaultIdleTimeout,
	}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	if s.Credentials == nil {
		return errors.New("SOCKS server: nil credential store")
	}

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.Addr, err)
	}

	defer ln.Close()

	s.Logger.Info("SOCKS5 server listening", "addr", s.Addr)

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()

		if err != nil {
			select {
			case <-ctx.Done():
				s.wg.Wait()
				return nil
			default:
			}
			s.Logger.Debug("SOCKS accept error", "error", err)
			continue
		}

		s.wg.Add(1)

		go func() {
			defer s.wg.Done()
			s.handleConnection(ctx, conn)
		}()
	}
}

func (s *Server) handleConnection(parent context.Context, conn net.Conn) error {
	defer conn.Close()

	remote := conn.RemoteAddr().String()

	// --------------------------------------------------
	// Authentication deadline
	// --------------------------------------------------

	authTimeout := s.AuthTimeout

	if authTimeout <= 0 {
		authTimeout = DefaultAuthTimeout
	}

	if err := conn.SetDeadline(time.Now().Add(authTimeout)); err != nil {
		s.Logger.Debug("set auth deadline", "remote", remote, "error", err)
		return fmt.Errorf("%s: set auth deadline: %v", remote, err)
	}

	// --------------------------------------------------
	// SOCKS5 method negotiation
	// --------------------------------------------------

	if err := negotiate(conn, conn); err != nil {
		s.Logger.Info("SOCKS negotiation failed", "remote", remote, "error", err)
		return fmt.Errorf("%s: SOCKS negotiation failed: %v", remote, err)
	}

	// --------------------------------------------------
	// Authentication
	// --------------------------------------------------

	appID, password, err := readUserPass(conn)

	if err != nil {
		_ = writeAuthReply(conn, false)
		s.Logger.Info("invalid authentication frame", "remote", remote, "error", err)
		return fmt.Errorf("%s: invalid authentication frame: %v", remote, err)
	}

	authCtx, cancel := context.WithTimeout(parent, authTimeout)
	defer cancel()

	if err := s.Credentials.Authenticate(authCtx, appID, password); err != nil {
		_ = writeAuthReply(conn, false)
		s.Logger.Info("authentication failed for app", "remote", remote, "app_id", appID)
		return fmt.Errorf("%s: authentication failed for app=%q", remote, appID)
	}

	if err := writeAuthReply(conn, true); err != nil {
		return fmt.Errorf("%s: authentication failed for app=%q", remote, appID)
	}

	// Don't keep the password alive longer than necessary.
	password = ""

	// --------------------------------------------------
	// SOCKS CONNECT
	// --------------------------------------------------

	req, err := readConnectRequest(conn)
	if err != nil {
		_ = writeConnectReply(conn, ReplyGeneralFailure)
		s.Logger.Info("invalid CONNECT", "remote", remote, "error", err)
		return fmt.Errorf("%s: invalid CONNECT: %v", remote, err)
	}
	s.Logger.Info("CONNECT request received", "remote", remote, "app_id", appID, "addr", req.Address())

	// --------------------------------------------------
	// Authentication is complete.
	// Remove deadline before long-lived tunnel.
	// --------------------------------------------------

	if err := conn.SetDeadline(time.Time{}); err != nil {
		s.Logger.Debug("clear deadline", "remote", remote, "error", err)
		return fmt.Errorf("%s: clear deadline: %v",remote, err)
	}

	// --------------------------------------------------
	// Open Ashrix QUIC stream.
	//
	// IMPORTANT:
	// The Open() implementation writes the Ashrix
	// OPEN frame and waits for the gateway response.
	// --------------------------------------------------

	openCtx := parent


	qStream, err := s.Tunnel.OpenStream(openCtx)
	if err != nil {
		s.Logger.Error("failed to open stream to gateway", "error", err)
		return fmt.Errorf("failed to open stream to gateway: %w", err)
	}
	defer qStream.Close()

	flowID := uuid.NewString()
	sessionID := uuid.NewString()
	
	envelope := proto.StreamFrame{
		RequestId:     flowID,
		SourceId: appID,
		SessionId: sessionID,
		DestAppName: req.Host,
		StreamType:    proto.RequestType_HTTP_REQUEST,
		FlowType:      proto.FlowType_APP_TO_APP,
	}

	if err := frame.WriteFrame(qStream, &envelope); err != nil {
		return fmt.Errorf("failed to send opening frame to gateway: %w", err)
	}


	// --------------------------------------------------
	// Gateway accepted the stream.
	// Now tell SOCKS client that CONNECT succeeded.
	// --------------------------------------------------

	payload, err := frame.ReadFrame(qStream)
	if err != nil {
		return fmt.Errorf("failed to read response frame from gateway: %w", err)
		
	}

	var respFrame proto.StreamFrame
	if err := frame.DecodeFrame(payload, &respFrame); err != nil {
		return fmt.Errorf("failed to decode response frame: %w", err)
	}

	fmt.Println(&respFrame)

	if respFrame.Method != "OPEN_OK" {
		writeConnectReply(conn, ReplyNotAllowed)
		return fmt.Errorf("gateway rejected open request: status=%s", respFrame.Method)
	}


	if err := writeConnectReply(conn,ReplySucceeded); err != nil {
		return fmt.Errorf("failed to send SOCKS success reply: %w", err)
	}

	// --------------------------------------------------
	// Application traffic
	// --------------------------------------------------

	srcFlowStream := &tcpStreamAdapter{Conn: conn, ctx: parent}
	destFlowStream := &qStreamAdapter{Stream: qStream, ctx: parent}

	s.Logger.Info("Relaying SOCKS flow", "flow_id", flowID)
	relayRes := flow.Relay(parent, srcFlowStream, destFlowStream)
	return relayRes.Err
}

type tcpStreamAdapter struct {
	net.Conn
	ctx context.Context
}

func (t *tcpStreamAdapter) CloseRead() error {
	if tc, ok := t.Conn.(*net.TCPConn); ok {
		return tc.CloseRead()
	}
	return nil
}

func (t *tcpStreamAdapter) CloseWrite() error {
	if tc, ok := t.Conn.(*net.TCPConn); ok {
		return tc.CloseWrite()
	}
	return nil
}

func (t *tcpStreamAdapter) Context() context.Context {
	if t.ctx != nil {
		return t.ctx
	}
	return context.Background()
}

type qStreamAdapter struct {
	transport.Stream
	ctx context.Context
}

func (q *qStreamAdapter) CloseRead() error {
	return nil
}

func (q *qStreamAdapter) Context() context.Context {
	if q.ctx != nil {
		return q.ctx
	}
	return context.Background()
}
