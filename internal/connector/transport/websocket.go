// internal/connector/transport/websocket.go
// WebSocket fallback — used only when QUIC and gRPC are both blocked.
// Typically by DPI stripping HTTP/2 headers.
package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
)

type wsSession struct {
	session *yamux.Session
	log     *slog.Logger
	done    chan struct{}
	once    sync.Once
}

// DialWebSocket connects via WebSocket + yamux.
// Last resort — only when QUIC and gRPC both fail.
func DialWebSocket(
	ctx context.Context,
	url string,
	connectorID string,
	token string,
	tlsConfig *tls.Config,
	log *slog.Logger,
) (Session, error) {
	log.Info("trying WebSocket transport", "url", url)

	dialer := websocket.Dialer{
		TLSClientConfig:  tlsConfig,
		HandshakeTimeout: 10 * time.Second,
	}

	// Auth at HTTP level before WebSocket upgrade
	headers := http.Header{}
	headers.Set("X-Ashrix-Connector-ID", connectorID)
	headers.Set("X-Ashrix-Token", token)

	wsConn, _, err := dialer.DialContext(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("WebSocket dial: %w", err)
	}

	netConn := &wsNetConn{conn: wsConn}

	session, err := yamux.Client(netConn, &yamux.Config{
		AcceptBacklog:          256,
		EnableKeepAlive:        true,
		KeepAliveInterval:      15 * time.Second,
		ConnectionWriteTimeout: 10 * time.Second,
		MaxStreamWindowSize:    512 * 1024,
		LogOutput:              io.Discard,
	})
	if err != nil {
		wsConn.Close()
		return nil, fmt.Errorf("yamux session: %w", err)
	}

	log.Info("WebSocket+yamux session established", "url", url)

	s := &wsSession{
		session: session,
		log:     log,
		done:    make(chan struct{}),
	}

	go func() {
		<-session.CloseChan()
		s.once.Do(func() { close(s.done) })
	}()

	return s, nil
}

func (s *wsSession) OpenStream(ctx context.Context) (Stream, error) {
	stream, err := s.session.Open()
	if err != nil {
		return nil, fmt.Errorf("yamux open stream: %w", err)
	}
	return &yamuxStream{stream: stream}, nil
}

func (s *wsSession) AcceptStream(ctx context.Context) (Stream, error) {
	stream, err := s.session.Accept()
	if err != nil {
		return nil, fmt.Errorf("yamux accept stream: %w", err)
	}
	return &yamuxStream{stream: stream}, nil
}

func (s *wsSession) Close() error {
	s.once.Do(func() { close(s.done) })
	return s.session.Close()
}

func (s *wsSession) Done() <-chan struct{}  { return s.done }
func (s *wsSession) TransportName() string  { return "websocket" }

// yamuxStream wraps yamux.Stream
type yamuxStream struct{ stream net.Conn }

func (s *yamuxStream) Read(b []byte) (int, error)  { return s.stream.Read(b) }
func (s *yamuxStream) Write(b []byte) (int, error) { return s.stream.Write(b) }
func (s *yamuxStream) Close() error                { return s.stream.Close() }
func (s *yamuxStream) CloseWrite() error           { return s.stream.Close() }

// wsNetConn adapts WebSocket to net.Conn for yamux
type wsNetConn struct {
	conn   *websocket.Conn
	reader io.Reader
	mu     sync.Mutex
}

func (c *wsNetConn) Read(b []byte) (int, error) {
	if c.reader == nil {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return 0, err
		}
		c.reader = &byteReader{data: msg}
	}
	n, err := c.reader.Read(b)
	if err == io.EOF {
		c.reader = nil
		return n, nil
	}
	return n, err
}

func (c *wsNetConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *wsNetConn) Close() error                       { return c.conn.Close() }
func (c *wsNetConn) SetDeadline(t time.Time) error      { return nil }
func (c *wsNetConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *wsNetConn) SetWriteDeadline(t time.Time) error { return nil }
func (c *wsNetConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (c *wsNetConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(b []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(b, r.data[r.pos:])
	r.pos += n
	return n, nil
}
