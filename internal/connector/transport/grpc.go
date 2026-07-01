package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"sync"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

// grpcSession implements Session over gRPC/HTTP2.
// One gRPC connection carries N tunnel streams via HTTP/2 multiplexing.
// No yamux needed — HTTP/2 IS the multiplexer.
type grpcSession struct {
	conn   *grpc.ClientConn
	client pb.TunnelServiceClient
	log    *zap.Logger
	done   chan struct{}
	once   sync.Once
}

func DialGRPC(
	ctx context.Context,
	addr string,
	tlsConfig *tls.Config,
	log *zap.Logger,
) (Session, error) {
	log.Info("trying gRPC/HTTP2 transport", zap.String("addr", addr))

	creds := credentials.NewTLS(tlsConfig)

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
		// Large chunk support for proxying big responses
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(32*1024*1024), // 32MB per chunk
			grpc.MaxCallSendMsgSize(32*1024*1024),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("gRPC dial: %w", err)
	}

	log.Info("gRPC/HTTP2 connection established",
		zap.String("addr", addr),
	)

	s := &grpcSession{
		conn:   conn,
		client: pb.NewTunnelServiceClient(conn),
		log:    log,
		done:   make(chan struct{}),
	}

	// Watch for connection death
	go func() {
		for {
			state := conn.GetState()
			if state == connectivity.Shutdown ||
				state == connectivity.TransientFailure {
				s.once.Do(func() { close(s.done) })
				return
			}
			if !conn.WaitForStateChange(ctx, state) {
				s.once.Do(func() { close(s.done) })
				return
			}
		}
	}()

	return s, nil
}

// OpenStream opens a gRPC tunnel stream.
// Used for the management/registration stream.
func (s *grpcSession) OpenStream(ctx context.Context) (Stream, error) {
	grpcStream, err := s.client.Tunnel(ctx)
	if err != nil {
		return nil, fmt.Errorf("gRPC open stream: %w", err)
	}
	return newGRPCStream(grpcStream), nil
}

// AcceptStream opens a stream and signals READY to gateway.
// The gateway sends a request into this waiting stream.
//
// NOTE: gRPC client cannot truly "accept" a server-initiated stream
// the way QUIC can. Instead, the connector pre-opens streams and
// signals READY — gateway picks up a waiting stream and sends into it.
// This is the standard ZTNA pattern for gRPC tunnels.
func (s *grpcSession) AcceptStream(ctx context.Context) (Stream, error) {
	grpcStream, err := s.client.Tunnel(ctx)
	if err != nil {
		return nil, fmt.Errorf("gRPC accept stream: %w", err)
	}

	stream := newGRPCStream(grpcStream)

	// Signal to gateway: this stream is ready for a request
	if err := stream.sendChunk(&pb.TunnelChunk{
		Payload: &pb.TunnelChunk_Ready{Ready: true},
	}); err != nil {
		return nil, fmt.Errorf("send READY: %w", err)
	}

	return stream, nil
}

func (s *grpcSession) Close() error {
	s.once.Do(func() { close(s.done) })
	return s.conn.Close()
}

func (s *grpcSession) Done() <-chan struct{}  { return s.done }
func (s *grpcSession) TransportName() string  { return "grpc" }

// ── grpcStream ────────────────────────────────────────────────────────────────

// grpcStream adapts a gRPC bidirectional stream to our Stream interface.
// Internally chunks writes to stay under gRPC message size limits.
// Reassembles chunks on reads into a continuous byte stream.
type grpcStream struct {
	stream  pb.TunnelService_TunnelClient

	// Read buffer — holds unconsumed bytes from last Recv
	readBuf []byte
	readPos int
	readMu  sync.Mutex

	// Write mutex — gRPC streams are not concurrent-write safe
	writeMu sync.Mutex
}

func newGRPCStream(s pb.TunnelService_TunnelClient) *grpcStream {
	return &grpcStream{stream: s}
}

// Write sends bytes as body chunks.
// Automatically chunks at 32KB to stay under gRPC limits.
// Supports arbitrary payload sizes — file downloads, large APIs, all fine.
func (s *grpcStream) Write(b []byte) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	const chunkSize = 32 * 1024 // 32KB per chunk

	total := 0
	for len(b) > 0 {
		chunk := b
		if len(chunk) > chunkSize {
			chunk = b[:chunkSize]
		}

		if err := s.stream.Send(&pb.TunnelChunk{
			Payload: &pb.TunnelChunk_BodyChunk{
				BodyChunk: chunk,
			},
		}); err != nil {
			return total, err
		}

		total += len(chunk)
		b = b[len(chunk):]
	}
	return total, nil
}

// Read receives body chunks and presents them as a continuous stream.
// Handles buffering so callers get exactly what they asked for.
func (s *grpcStream) Read(b []byte) (int, error) {
	s.readMu.Lock()
	defer s.readMu.Unlock()

	// Serve from buffer if we have leftover bytes
	if s.readPos < len(s.readBuf) {
		n := copy(b, s.readBuf[s.readPos:])
		s.readPos += n
		return n, nil
	}

	// Fetch next chunk from stream
	for {
		msg, err := s.stream.Recv()
		if err == io.EOF {
			return 0, io.EOF
		}
		if err != nil {
			return 0, err
		}

		switch p := msg.Payload.(type) {

		case *pb.TunnelChunk_BodyChunk:
			if len(p.BodyChunk) == 0 {
				continue // skip empty chunks
			}
			s.readBuf = p.BodyChunk
			s.readPos = 0
			n := copy(b, s.readBuf)
			s.readPos = n
			return n, nil

		case *pb.TunnelChunk_EndStream:
			return 0, io.EOF

		case *pb.TunnelChunk_ReqHeader:
			// Request header — caller (proxy) handles this
			// Store it so proxy can read it via ReadHeader()
			// For now: encode as first bytes so proxy JSON decoder sees it
			// This is handled at the envelope level above stream
			continue

		default:
			// Unknown chunk type — skip
			continue
		}
	}
}

func (s *grpcStream) Close() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Send end-of-stream marker before closing
	_ = s.stream.Send(&pb.TunnelChunk{
		Payload: &pb.TunnelChunk_EndStream{EndStream: true},
	})
	return s.stream.CloseSend()
}

func (s *grpcStream) CloseWrite() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_ = s.stream.Send(&pb.TunnelChunk{
		Payload: &pb.TunnelChunk_EndStream{EndStream: true},
	})
	return s.stream.CloseSend()
}

func (s *grpcStream) sendChunk(chunk *pb.TunnelChunk) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.stream.Send(chunk)
}

