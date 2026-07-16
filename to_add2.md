// EARLIER VERSION — HAS A BUG
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
		return n, nil  // ← BUG: swallows EOF, returns nil error
	}
	return n, err
}



// internal/connector/transport/ws_netconn.go
package transport

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// wsNetConn adapts a message-oriented gorilla WebSocket connection
// into a byte-oriented net.Conn, which is what yamux requires.
//
// The core problem: WebSocket delivers discrete messages.
// yamux (and any Read/Write-based protocol) expects a continuous
// byte stream where a Write() of N bytes may be split across
// multiple Read() calls, or multiple Writes may be coalesced into
// one Read() — exactly TCP's semantics. This type bridges that gap.
type wsNetConn struct {
	conn *websocket.Conn

	// readBuf holds leftover bytes from the CURRENT WebSocket
	// message that haven't been consumed yet. A single WebSocket
	// message frequently contains more bytes than one Read() call
	// asks for — those extra bytes must be held here, not discarded,
	// or data is silently lost.
	readBuf *bytes.Reader
	readMu  sync.Mutex

	// writeMu serializes writes — gorilla's WebSocket connection
	// is explicitly NOT safe for concurrent writes from multiple
	// goroutines (documented in gorilla/websocket itself). yamux's
	// own internals already serialize writes through a single
	// sender in most usage patterns, but we enforce it here too
	// as defense in depth — cheap insurance against a future
	// caller that doesn't respect that assumption.
	writeMu sync.Mutex

	closeOnce sync.Once
	closed    chan struct{}
}

func newWSNetConn(conn *websocket.Conn) *wsNetConn {
	return &wsNetConn{
		conn:   conn,
		closed: make(chan struct{}),
	}
}

// Read implements io.Reader / net.Conn.Read correctly:
// - Serves from the buffered remainder of the current WS message first
// - Blocks on ReadMessage() only when the buffer is exhausted
// - NEVER returns (0, nil) — always makes progress or returns an error
func (c *wsNetConn) Read(b []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	for {
		// Serve remaining bytes from the current message, if any
		if c.readBuf != nil && c.readBuf.Len() > 0 {
			n, _ := c.readBuf.Read(b) // bytes.Reader.Read never returns
			                          // an error here except io.EOF,
			                          // and we already checked Len() > 0
			return n, nil
		}

		// Current message (if any) is fully consumed — fetch the next one.
		// This is a BLOCKING call — correct, because Read() on a
		// net.Conn is expected to block until data is available.
		msgType, data, err := c.conn.ReadMessage()
		if err != nil {
			// Distinguish a clean WebSocket close from a real error.
			// yamux needs to see io.EOF specifically for clean shutdown
			// to be handled as "session ended," not "session errored."
			if websocket.IsCloseError(err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
			) {
				return 0, io.EOF
			}
			return 0, err
		}

		if msgType != websocket.BinaryMessage {
			// We only ever send BinaryMessage frames (see Write below).
			// A TextMessage or control frame arriving here means
			// either a protocol violation by the peer, or gorilla's
			// automatic ping/pong handling intercepted something —
			// either way, skip it rather than trying to interpret
			// non-binary data as yamux frame bytes, which would
			// corrupt the stream.
			continue
		}

		if len(data) == 0 {
			// Empty binary message — legal per WebSocket spec but
			// meaningless here. Skip and fetch the next message
			// rather than returning (0, nil), which would violate
			// the io.Reader contract's implicit expectation of
			// forward progress.
			continue
		}

		c.readBuf = bytes.NewReader(data)
		// Loop back — will now serve from c.readBuf on the next
		// iteration of this for loop, guaranteeing forward progress
		// before returning.
	}
}

// Write implements io.Writer / net.Conn.Write.
// Wraps the given bytes as ONE WebSocket binary message.
//
// Note: this means the framing between yamux's writes and
// WebSocket's messages is: one yamux Write() call = one WS message.
// This is safe and correct — yamux's own internal framing already
// operates on discrete frames of known length, it does not require
// writes to be split or merged in any particular way by the
// underlying transport. It only requires that all bytes written
// arrive, in order, uncorrupted. One-message-per-write satisfies that.
func (c *wsNetConn) Write(b []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	select {
	case <-c.closed:
		return 0, net.ErrClosed
	default:
	}

	if err := c.conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *wsNetConn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closed)
		// Send a proper WebSocket close frame before closing the
		// underlying TCP connection — this lets the peer distinguish
		// "clean shutdown" from "connection dropped," which matters
		// for logging and for the IsCloseError check in Read above
		// to actually fire correctly on the peer's side.
		deadline := time.Now().Add(2 * time.Second)
		c.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			deadline,
		)
		err = c.conn.Close()
	})
	return err
}

// ── net.Conn interface completeness ──────────────────────────────
// yamux's Config accepts any net.Conn-like type, but the interface
// requires these methods to exist even if we don't meaningfully
// support per-operation deadlines (gorilla's ReadMessage/WriteMessage
// have their own deadline mechanisms we could wire through here,
// but keeping this simple and correct is more valuable right now
// than deadline granularity we don't yet have a concrete need for).

func (c *wsNetConn) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *wsNetConn) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }

func (c *wsNetConn) SetDeadline(t time.Time) error {
	if err := c.conn.SetReadDeadline(t); err != nil {
		return err
	}
	return c.conn.SetWriteDeadline(t)
}

func (c *wsNetConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *wsNetConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}




// internal/connector/transport/websocket.go — corrected, full version
package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"go.uber.org/zap"
)

type wsSession struct {
	session   *yamux.Session
	wsConn    *wsNetConn
	log       *zap.Logger
	done      chan struct{}
	closeOnce sync.Once
}

// DialWebSocket connects to the gateway over WSS, then layers
// yamux on top for stream multiplexing. Last-resort fallback —
// used only when QUIC and gRPC/HTTP2 both fail.
func DialWebSocket(
	ctx context.Context,
	wsURL string, // "wss://gateway.ashrix.io/tunnel"
	connectorID string,
	token string,
	tlsConfig *tls.Config,
	log *zap.Logger,
) (Session, error) {

	log.Info("trying WebSocket transport", zap.String("url", wsURL))

	dialer := websocket.Dialer{
		TLSClientConfig:  tlsConfig,
		HandshakeTimeout: 10 * time.Second,
		// ReadBufferSize / WriteBufferSize left at gorilla defaults
		// (4096 bytes) — fine for yamux frame sizes, which are
		// typically small control frames plus chunked data frames.
	}

	// Auth happens at the HTTP layer, BEFORE the WebSocket upgrade —
	// this is the mechanism from earlier in this conversation:
	// the gateway's upgrade handler validates these headers and
	// REJECTS the upgrade entirely if invalid, so an unauthenticated
	// party never even gets a WebSocket connection, let alone a
	// yamux session inside one.
	headers := http.Header{}
	headers.Set("X-Ashrix-Connector-ID", connectorID)
	headers.Set("X-Ashrix-Token", token)

	rawConn, resp, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("WebSocket dial failed (HTTP %d): %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("WebSocket dial failed: %w", err)
	}

	// Wrap the WebSocket connection as a byte-stream net.Conn
	netConn := newWSNetConn(rawConn)

	// ── yamux client — THIS is where multiplexing actually happens ──
	// From this point forward, yamux owns the framing of everything
	// that flows over netConn. Every yamux.Stream opened on this
	// session becomes one WS message per yamux frame written —
	// see the Write() method above.
	yamuxConfig := &yamux.Config{
		AcceptBacklog:          256,
		EnableKeepAlive:        true,
		KeepAliveInterval:      15 * time.Second, // beats typical NAT/firewall idle timeouts
		ConnectionWriteTimeout: 10 * time.Second,
		MaxStreamWindowSize:    512 * 1024, // 512KB — generous for higher-latency links
		LogOutput:              io.Discard,  // yamux's own logging — route to your
		                                     // structured logger in production via
		                                     // a small io.Writer adapter, not left
		                                     // as Discard forever
	}

	session, err := yamux.Client(netConn, yamuxConfig)
	if err != nil {
		netConn.Close()
		return nil, fmt.Errorf("yamux client session: %w", err)
	}

	log.Info("WebSocket+yamux session established", zap.String("url", wsURL))

	s := &wsSession{
		session: session,
		wsConn:  netConn,
		log:     log,
		done:    make(chan struct{}),
	}

	// Watch for the yamux session dying (peer closed, network died,
	// etc) and propagate that as Done() closing, matching the
	// Session interface contract used identically by QUIC and gRPC.
	go func() {
		<-session.CloseChan()
		s.closeOnce.Do(func() { close(s.done) })
	}()

	return s, nil
}

func (s *wsSession) OpenStream(ctx context.Context) (Stream, error) {
	// yamux.Session.Open() does not take a context in the version
	// most commonly vendored — if ctx cancellation mid-open matters
	// to you, wrap this in a select with ctx.Done(), but yamux's
	// Open() is typically fast (local bookkeeping, not a network
	// round-trip) so this is rarely a real concern in practice.
	stream, err := s.session.Open()
	if err != nil {
		return nil, fmt.Errorf("yamux open stream: %w", err)
	}
	return &yamuxStreamAdapter{stream: stream}, nil
}

func (s *wsSession) AcceptStream(ctx context.Context) (Stream, error) {
	stream, err := s.session.Accept()
	if err != nil {
		return nil, fmt.Errorf("yamux accept stream: %w", err)
	}
	return &yamuxStreamAdapter{stream: stream}, nil
}

func (s *wsSession) Close() error {
	s.closeOnce.Do(func() { close(s.done) })
	if err := s.session.Close(); err != nil {
		return err
	}
	return s.wsConn.Close()
}

func (s *wsSession) Done() <-chan struct{}  { return s.done }
func (s *wsSession) TransportName() string  { return "websocket" }

// yamuxStreamAdapter satisfies our own Stream interface —
// yamux's Stream type already implements net.Conn (which is a
// superset of what we need), so this is a thin pass-through.
type yamuxStreamAdapter struct {
	stream *yamux.Stream
}

func (s *yamuxStreamAdapter) Read(b []byte) (int, error)  { return s.stream.Read(b) }
func (s *yamuxStreamAdapter) Write(b []byte) (int, error) { return s.stream.Write(b) }
func (s *yamuxStreamAdapter) Close() error                { return s.stream.Close() }
func (s *yamuxStreamAdapter) CloseWrite() error            { return s.stream.CloseWrite() }






// internal/gateway/wsserver/server.go
package wsserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"go.uber.org/zap"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
)

// CPStateClient — same interface used by the QUIC and gRPC
// tunnel servers, kept consistent across all three transports
// per the design established earlier in this conversation.
type CPStateClient interface {
	ValidateConnectorToken(ctx context.Context, connectorID, token string) error
}

type Server struct {
	registry *registry.Registry
	cpState  CPStateClient
	log      *zap.Logger
	upgrader websocket.Upgrader
}

func New(reg *registry.Registry, cpState CPStateClient, log *zap.Logger) *Server {
	return &Server{
		registry: reg,
		cpState:  cpState,
		log:      log,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			// CheckOrigin: browsers should NEVER be initiating this
			// connection — only the connector binary. Rejecting
			// based on Origin header presence is one more layer
			// closing the "browser tries to open a raw tunnel"
			// attack surface named several messages ago in this
			// conversation.
			CheckOrigin: func(r *http.Request) bool {
				return r.Header.Get("Origin") == ""
			},
		},
	}
}

// HandleUpgrade is the http.HandlerFunc mounted at /tunnel.
//
// CRITICAL ordering, matching the design established earlier:
// auth validation happens BEFORE the WebSocket upgrade completes.
// If the token is invalid, we return an HTTP error and NEVER
// call Upgrade() — meaning an unauthenticated party never even
// gets a WebSocket handshake, let alone a chance to attempt a
// yamux session. This is strictly better than accepting the
// upgrade and rejecting afterward, because it closes the window
// where an attacker could hold open partially-established
// connections as a resource-exhaustion vector.
func (s *Server) HandleUpgrade(w http.ResponseWriter, r *http.Request) {
	connectorID := r.Header.Get("X-Ashrix-Connector-ID")
	token := r.Header.Get("X-Ashrix-Token")

	if connectorID == "" || token == "" {
		http.Error(w, "missing connector credentials", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.cpState.ValidateConnectorToken(ctx, connectorID, token); err != nil {
		s.log.Warn("WebSocket tunnel auth rejected",
			zap.String("connector_id", connectorID),
			zap.Error(err))
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Auth passed — NOW perform the WebSocket upgrade
	wsConn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Warn("WebSocket upgrade failed", zap.Error(err))
		return
	}

	netConn := newWSNetConn(wsConn)

	// ── yamux SERVER side — mirrors the client config exactly ──────
	// Mismatched configs between client/server (e.g. different
	// MaxStreamWindowSize) don't break yamux — the protocol
	// negotiates per-stream flow control independently — but
	// keeping them identical avoids needless asymmetry in behavior
	// that would make debugging harder later.
	yamuxConfig := &yamux.Config{
		AcceptBacklog:          256,
		EnableKeepAlive:        true,
		KeepAliveInterval:      15 * time.Second,
		ConnectionWriteTimeout: 10 * time.Second,
		MaxStreamWindowSize:    512 * 1024,
		LogOutput:              io.Discard,
	}

	session, err := yamux.Server(netConn, yamuxConfig)
	if err != nil {
		s.log.Error("yamux server session failed", zap.Error(err))
		netConn.Close()
		return
	}

	s.log.Info("WebSocket+yamux tunnel established",
		zap.String("connector_id", connectorID))

	tunnelSession := &wsTunnelSession{session: session, netConn: netConn}

	// Requires an existing management-plane registration —
	// same "management first" rule enforced by the QUIC server,
	// applied consistently across all tunnel transports.
	if _, exists := s.registry.GetByConnectorID(connectorID); !exists {
		s.log.Warn("WebSocket tunnel attempted before management registration",
			zap.String("connector_id", connectorID))
		tunnelSession.Close()
		return
	}

	s.registry.AttachTunnel(connectorID, tunnelSession, "websocket")
	defer s.registry.DetachTunnel(connectorID)

	<-session.CloseChan()

	s.log.Info("WebSocket tunnel closed", zap.String("connector_id", connectorID))
}

// wsTunnelSession implements registry.TunnelSession for the
// WebSocket+yamux fallback path — the gateway-side counterpart
// to quicTunnelSession from earlier in this conversation.
// Notice: httpproxy/proxy.go never needs to know this type exists.
// It only ever calls entry.TunnelSession.OpenStream(), exactly as
// designed several messages ago.
type wsTunnelSession struct {
	session *yamux.Session
	netConn *wsNetConn
}

func (s *wsTunnelSession) OpenStream() (registry.Stream, error) {
	stream, err := s.session.Open()
	if err != nil {
		return nil, fmt.Errorf("yamux open stream: %w", err)
	}
	return stream, nil // yamux.Stream already satisfies registry.Stream's
	                     // Read/Write/Close — no adapter needed here
}

func (s *wsTunnelSession) Close() error {
	if err := s.session.Close(); err != nil {
		return err
	}
	return s.netConn.Close()
}