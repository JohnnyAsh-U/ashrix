package socks

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/flow"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Server struct {
	addr        string
	username    string
	password    string
	session     transport.Session
	log         *zap.Logger
	listener    net.Listener
	mu          sync.Mutex
	activeFlows sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewServer(addr, username, password string, session transport.Session, log *zap.Logger) *Server {
	return &Server{
		addr:     addr,
		username: username,
		password: password,
		session:  session,
		log:      log,
	}
}

func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("socks5 listen failed: %w", err)
	}
	s.listener = l
	s.log.Info("SOCKS5 Server started", zap.String("addr", s.addr))

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return nil
			default:
				s.log.Debug("SOCKS5 accept error or listener closed")
				return nil
			}
		}

		s.activeFlows.Add(1)
		go func() {
			defer s.activeFlows.Done()
			s.handleConnection(conn)
		}()
	}
}

func (s *Server) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.listener != nil {
		s.listener.Close()
	}
	s.activeFlows.Wait()
	s.log.Info("SOCKS5 Server stopped")
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	if err := s.handshake(conn); err != nil {
		s.log.Warn("SOCKS5 handshake failed", zap.String("client", conn.RemoteAddr().String()), zap.Error(err))
		return
	}

	dest, err := s.readRequest(conn)
	if err != nil {
		s.log.Warn("SOCKS5 read request failed", zap.String("client", conn.RemoteAddr().String()), zap.Error(err))
		return
	}

	err = s.routeToGateway(conn, dest)
	if err != nil {
		s.log.Warn("SOCKS5 routing failed", zap.String("client", conn.RemoteAddr().String()), zap.String("dest", dest), zap.Error(err))
		s.sendReply(conn, 0x01)
	}
}

func (s *Server) handshake(conn net.Conn) error {
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}

	if buf[0] != 0x05 {
		return fmt.Errorf("unsupported SOCKS version: 0x%02x", buf[0])
	}

	nmethods := buf[1]
	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}

	hasUserPass := false
	for _, m := range methods {
		if m == 0x02 {
			hasUserPass = true
			break
		}
	}

	if !hasUserPass {
		conn.Write([]byte{0x05, 0xff})
		return errors.New("client does not support username/password auth")
	}

	if _, err := conn.Write([]byte{0x05, 0x02}); err != nil {
		return err
	}

	subBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, subBuf); err != nil {
		return err
	}

	if subBuf[0] != 0x01 {
		return fmt.Errorf("unsupported subnegotiation version: 0x%02x", subBuf[0])
	}

	userLen := subBuf[1]
	userBytes := make([]byte, userLen)
	if _, err := io.ReadFull(conn, userBytes); err != nil {
		return err
	}

	passLenBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, passLenBuf); err != nil {
		return err
	}

	passLen := passLenBuf[0]
	passBytes := make([]byte, passLen)
	if _, err := io.ReadFull(conn, passBytes); err != nil {
		return err
	}

	if string(userBytes) != s.username || string(passBytes) != s.password {
		conn.Write([]byte{0x01, 0x01})
		return fmt.Errorf("auth failure: invalid credentials for user %s", string(userBytes))
	}

	if _, err := conn.Write([]byte{0x01, 0x00}); err != nil {
		return err
	}

	return nil
}

func (s *Server) readRequest(conn net.Conn) (string, error) {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return "", err
	}

	if buf[0] != 0x05 {
		return "", fmt.Errorf("unsupported request version: 0x%02x", buf[0])
	}

	if buf[1] != 0x01 {
		return "", fmt.Errorf("unsupported request command: 0x%02x", buf[1])
	}

	var host string
	switch buf[3] {
	case 0x01:
		ipBuf := make([]byte, 4)
		if _, err := io.ReadFull(conn, ipBuf); err != nil {
			return "", err
		}
		host = net.IP(ipBuf).String()
	case 0x03:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", err
		}
		length := lenBuf[0]
		domainBuf := make([]byte, length)
		if _, err := io.ReadFull(conn, domainBuf); err != nil {
			return "", err
		}
		host = string(domainBuf)
	case 0x04:
		ipBuf := make([]byte, 16)
		if _, err := io.ReadFull(conn, ipBuf); err != nil {
			return "", err
		}
		host = net.IP(ipBuf).String()
	default:
		return "", fmt.Errorf("unsupported address type: 0x%02x", buf[3])
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", err
	}
	port := binary.BigEndian.Uint16(portBuf)

	return fmt.Sprintf("%s:%d", host, port), nil
}

func (s *Server) routeToGateway(clientConn net.Conn, destination string) error {
	destHost, _, err := net.SplitHostPort(destination)
	if err != nil {
		destHost = destination
	}

	qStream, err := s.session.OpenStream(s.ctx)
	if err != nil {
		return fmt.Errorf("failed to open stream to gateway: %w", err)
	}
	defer qStream.Close()

	flowID := uuid.NewString()
	envelope := proto.StreamFrame{
		RequestId:     flowID,
		AppId:         destHost,
		StreamType:    proto.RequestType_HTTP_REQUEST,
		FlowType:      proto.FlowType_APP_TO_APP,
		SocksUsername: s.username,
		SocksPassword: s.password,
	}

	if err := frame.WriteFrame(qStream, &envelope); err != nil {
		return fmt.Errorf("failed to send opening frame to gateway: %w", err)
	}

	payload, err := frame.ReadFrame(qStream)
	if err != nil {
		return fmt.Errorf("failed to read response frame from gateway: %w", err)
	}

	var respFrame proto.StreamFrame
	if err := frame.DecodeFrame(payload, &respFrame); err != nil {
		return fmt.Errorf("failed to decode response frame: %w", err)
	}

	if respFrame.Method != "OPEN_OK" {
		return fmt.Errorf("gateway rejected open request: status=%s", respFrame.Method)
	}

	if err := s.sendReply(clientConn, 0x00); err != nil {
		return fmt.Errorf("failed to send SOCKS success reply: %w", err)
	}

	srcFlowStream := &tcpStreamAdapter{Conn: clientConn, ctx: s.ctx}
	destFlowStream := &qStreamAdapter{Stream: qStream, ctx: s.ctx}

	s.log.Info("Relaying SOCKS flow", zap.String("flow_id", flowID), zap.String("dest", destination))
	relayRes := flow.Relay(s.ctx, srcFlowStream, destFlowStream)
	return relayRes.Err
}

func (s *Server) sendReply(conn net.Conn, rep byte) error {
	reply := []byte{
		0x05,
		rep,
		0x00,
		0x01,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00,
	}
	_, err := conn.Write(reply)
	return err
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
