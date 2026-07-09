package quic

import (
	"context"
	"fmt"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/quic-go/quic-go"
)

type quicTunnelSession struct {
	conn *quic.Conn
}

func (s *quicTunnelSession) OpenStream() (registry.Stream, error) {
	stream, err := s.conn.OpenStreamSync(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Open Tunnel Stream: %w", err)
	}
	return &quicStreamAdapter{stream: stream}, nil
}

func (s *quicTunnelSession) Close() error {
	return s.conn.CloseWithError(0, "Closing")
}

type quicStreamAdapter struct {
	stream *quic.Stream
}

func (s *quicStreamAdapter) Read(p []byte) (int, error)  { return s.stream.Read(p) }
func (s *quicStreamAdapter) Write(p []byte) (int, error) { return s.stream.Write(p) }
func (s *quicStreamAdapter) Close() error                { return s.stream.Close() }
