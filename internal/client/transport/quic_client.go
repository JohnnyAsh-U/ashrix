package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/client/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/client/storage"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
)

type GatewayClient struct {
	cfg   *config.Config
	store storage.Store
}

func NewGatewayClient(cfg *config.Config, store storage.Store) *GatewayClient {
	return &GatewayClient{
		cfg:   cfg,
		store: store,
	}
}

type StreamAdapter struct {
	stream *quic.Stream
}

func (s *StreamAdapter) Read(p []byte) (n int, err error) {
	return s.stream.Read(p)
}

func (s *StreamAdapter) Write(p []byte) (n int, err error) {
	return s.stream.Write(p)
}

func (s *StreamAdapter) Close() error {
	return s.stream.Close()
}

func (s *StreamAdapter) CloseRead() error {
	s.stream.CancelRead(0)
	return nil
}

func (s *StreamAdapter) CloseWrite() error {
	return s.stream.Close()
}

func (s *StreamAdapter) Context() context.Context {
	return s.stream.Context()
}

func (g *GatewayClient) DialAndOpenStream(ctx context.Context, targetResourceID string) (io.ReadWriteCloser, error) {
	creds, err := g.store.Load()
	if err != nil {
		return nil, fmt.Errorf("authentication required: %w", err)
	}

	tlsConf := &tls.Config{
		InsecureSkipVerify: true, // For development/self-signed certs; set to false in production
		NextProtos:         []string{"ashrix-quic-v1"},
	}

	quicConfig := &quic.Config{
		KeepAlivePeriod: 15 * time.Second,
		MaxIdleTimeout:  30 * time.Second,
	}

	conn, err := quic.DialAddr(ctx, g.cfg.GatewayURL, tlsConf, quicConfig)
	if err != nil {
		// Fall back to local TCP gateway if QUIC port connection is in TCP test mode
		return g.dialTCPFallback(ctx, targetResourceID, creds)
	}

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		conn.CloseWithError(0, "failed to open stream")
		return nil, fmt.Errorf("failed to open QUIC stream: %w", err)
	}

	adapter := &StreamAdapter{stream: stream}

	// Prepare StreamFrame metadata
	reqID := uuid.New().String()
	streamFrame := &proto.StreamFrame{
		RequestId:   reqID,
		SessionId:   creds.SessionToken,
		DestAppId:   targetResourceID,
		DestAppName: targetResourceID,
		FlowType:    proto.FlowType_USER_TO_APP,
		StreamType:  proto.RequestType_UNSPECIFIED_REQUEST,
	}

	if err := frame.WriteFrame(adapter, streamFrame); err != nil {
		stream.Close()
		return nil, fmt.Errorf("failed to send initial StreamFrame handshake: %w", err)
	}

	// Read opening response ACK/ERR frame
	payload, err := frame.ReadFrame(adapter)
	if err != nil {
		stream.Close()
		return nil, fmt.Errorf("failed to read opening ACK frame from Gateway: %w", err)
	}

	var responseFrame proto.StreamFrame
	if err := frame.DecodeFrame(payload, &responseFrame); err != nil {
		stream.Close()
		return nil, fmt.Errorf("failed to decode response frame from Gateway: %w", err)
	}

	if responseFrame.Method == "OPEN_ERR" {
		stream.Close()
		return nil, fmt.Errorf("gateway rejected connection request for resource %s", targetResourceID)
	}

	return adapter, nil
}

func (g *GatewayClient) dialTCPFallback(ctx context.Context, targetResourceID string, creds *storage.SessionCredentials) (io.ReadWriteCloser, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", g.cfg.GatewayURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ashrix Gateway at %s: %w", g.cfg.GatewayURL, err)
	}

	reqID := uuid.New().String()
	streamFrame := &proto.StreamFrame{
		RequestId:   reqID,
		SessionId:   creds.SessionToken,
		DestAppId:   targetResourceID,
		DestAppName: targetResourceID,
		FlowType:    proto.FlowType_USER_TO_APP,
	}

	if err := frame.WriteFrame(conn, streamFrame); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send StreamFrame over TCP fallback: %w", err)
	}

	return conn, nil
}
