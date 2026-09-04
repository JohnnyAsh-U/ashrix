package transport

import (
	// "context"
	// "crypto/tls"
	// "fmt"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	// "github.com/gorilla/websocket"
	// "github.com/hashicorp/yamux"
)

type quicSession struct {
	conn *quic.Conn
	log  *slog.Logger
	done chan struct{}
	once sync.Once
}

// DialQUIC attempts a QUIC connection to the gateway.
// Returns error if UDP is blocked or gateway unreachable.
func DialQUIC(
	ctx context.Context,
	addr string,
	tlsConfig *tls.Config,
	log *slog.Logger,
) (Session, error) {
	log.Info("trying QUIC transport", "addr", addr)

	conn, err := quic.DialAddr(ctx, addr, tlsConfig, &quic.Config{
		KeepAlivePeriod: 15 * time.Second,
		MaxIdleTimeout:  30 * time.Second,
		// Allow many concurrent streams per connection
		// Each user request = one stream
		MaxIncomingStreams:    1000,
		MaxIncomingUniStreams: -1,
	})
	if err != nil {
		return nil, fmt.Errorf("QUIC dial: %w", err)
	}

	log.Info("QUIC connection established", "addr", addr)

	s := &quicSession{
		conn: conn,
		log:  log,
		done: make(chan struct{}),
	}

	// Watch for connection death
	go func() {
		<-conn.Context().Done()
		s.once.Do(func() { close(s.done) })
	}()

	return s, nil
}

func (s *quicSession) OpenStream(ctx context.Context) (Stream, error) {
	stream, err := s.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("QUIC open stream: %w", err)
	}
	return &quicStream{stream: stream}, nil
}

func (s *quicSession) AcceptStream(ctx context.Context) (Stream, error) {
	stream, err := s.conn.AcceptStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("QUIC accept stream: %w", err)
	}
	return &quicStream{stream: stream}, nil
}

func (s *quicSession) Close() error {
	s.once.Do(func() { close(s.done) })
	return s.conn.CloseWithError(0, "closing")
}

func (s *quicSession) Done() <-chan struct{} { return s.done }
func (s *quicSession) TransportName() string { return "quic" }

// quicStream wraps quic.Stream implementing our Stream interface.
type quicStream struct {
	stream *quic.Stream
}

func (s *quicStream) Read(b []byte) (int, error)  { return s.stream.Read(b) }
func (s *quicStream) Write(b []byte) (int, error) { return s.stream.Write(b) }
func (s *quicStream) Close() error                { return s.stream.Close() }
func (s *quicStream) CloseWrite() error {
	return s.stream.Close()
}
