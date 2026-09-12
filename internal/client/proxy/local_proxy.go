package proxy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/client/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/client/transport"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/flow"
)

type LocalProxy struct {
	cfg        *config.Config
	gwClient   *transport.GatewayClient
	resourceID string
	localAddr  string
	listener   net.Listener
	logger     *slog.Logger
	mu         sync.Mutex
	closed     bool
}

type netStream struct {
	net.Conn
}

func (n *netStream) CloseRead() error {
	if tcp, ok := n.Conn.(*net.TCPConn); ok {
		return tcp.CloseRead()
	}
	return nil
}

func (n *netStream) CloseWrite() error {
	if tcp, ok := n.Conn.(*net.TCPConn); ok {
		return tcp.CloseWrite()
	}
	return nil
}

func (n *netStream) Context() context.Context {
	return context.Background()
}

type flowStreamAdapter struct {
	io.ReadWriteCloser
}

func (f *flowStreamAdapter) CloseRead() error {
	return nil
}

func (f *flowStreamAdapter) CloseWrite() error {
	return f.Close()
}

func (f *flowStreamAdapter) Context() context.Context {
	return context.Background()
}

func NewLocalProxy(cfg *config.Config, gwClient *transport.GatewayClient, resourceID, localPort string, logger *slog.Logger) (*LocalProxy, error) {
	if localPort == "" {
		localPort = "0"
	}
	addr := fmt.Sprintf("127.0.0.1:%s", localPort)

	return &LocalProxy{
		cfg:        cfg,
		gwClient:   gwClient,
		resourceID: resourceID,
		localAddr:  addr,
		logger:     logger,
	}, nil
}

func (p *LocalProxy) Start(ctx context.Context) error {
	l, err := net.Listen("tcp", p.localAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", p.localAddr, err)
	}
	p.listener = l
	p.localAddr = l.Addr().String()

	p.logger.Info("Local TCP proxy listening",
		slog.String("local_addr", p.localAddr),
		slog.String("resource_id", p.resourceID),
	)

	go func() {
		<-ctx.Done()
		p.Close()
	}()

	for {
		conn, err := p.listener.Accept()
		if err != nil {
			p.mu.Lock()
			isClosed := p.closed
			p.mu.Unlock()
			if isClosed || ctx.Err() != nil {
				return nil
			}
			p.logger.Error("error accepting local connection", slog.Any("error", err))
			time.Sleep(100 * time.Millisecond)
			continue
		}

		go p.handleLocalConnection(ctx, conn)
	}
}

func (p *LocalProxy) LocalAddr() string {
	return p.localAddr
}

func (p *LocalProxy) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	if p.listener != nil {
		return p.listener.Close()
	}
	return nil
}

func (p *LocalProxy) handleLocalConnection(ctx context.Context, localConn net.Conn) {
	defer localConn.Close()

	p.logger.Debug("handling incoming connection", slog.String("client_addr", localConn.RemoteAddr().String()))

	gwStream, err := p.gwClient.DialAndOpenStream(ctx, p.resourceID)
	if err != nil {
		p.logger.Error("failed to establish Ashrix stream to Gateway",
			slog.String("resource_id", p.resourceID),
			slog.Any("error", err),
		)
		return
	}
	defer gwStream.Close()

	streamA := &netStream{Conn: localConn}
	streamB := &flowStreamAdapter{ReadWriteCloser: gwStream}

	res := flow.Relay(ctx, streamA, streamB)
	if res.Err != nil {
		p.logger.Debug("proxy stream relay finished",
			slog.Int64("bytes_sent", res.BytesAToB),
			slog.Int64("bytes_received", res.BytesBToA),
			slog.Any("error", res.Err),
		)
	}
}
