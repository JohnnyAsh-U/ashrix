package quic

import (
	"context"
	"fmt"

	"github.com/JohnnyAsh-U/ashrix-api/pkg/flow"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/quic-go/quic-go"
)

type quicTunnelSession struct {
	conn *quic.Conn
}

func (s *quicTunnelSession) OpenStream(ctx context.Context, req *proto.StreamFrame) (flow.Stream, error) {
	stream, err := s.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("Open Tunnel Stream: %w", err)
	}
	adapter := &quicStreamAdapter{
		stream: stream,
		ctx:    ctx,
	}

	if err := frame.WriteFrame(adapter, req); err != nil {
		stream.Close()
		return nil, fmt.Errorf("failed to write open request frame: %w", err)
	}

	return adapter, nil
}

func (s *quicTunnelSession) Close() error {
	return s.conn.CloseWithError(0, "Closing")
}

type quicStreamAdapter struct {
	stream *quic.Stream
	ctx    context.Context
}

func (s *quicStreamAdapter) Read(p []byte) (int, error)  { return s.stream.Read(p) }
func (s *quicStreamAdapter) Write(p []byte) (int, error) { return s.stream.Write(p) }
func (s *quicStreamAdapter) Close() error                { return s.stream.Close() }
func (s *quicStreamAdapter) CloseRead() error {
	s.stream.CancelRead(0)
	return nil
}
func (s *quicStreamAdapter) CloseWrite() error {
	s.stream.CancelWrite(0)
	return nil
}
func (s *quicStreamAdapter) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}
