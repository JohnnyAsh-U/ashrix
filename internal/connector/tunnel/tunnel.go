package tunnel

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	// "go.uber.org/zap"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/quic-go/quic-go"
)

type TunnelClient struct {
	log       *slog.Logger
	startedAt time.Time
	config    *config.Config
	apps      []*pb.ConnectorApps
	tlsConfig *tls.Config
}

// ConnectorTunnel manages the full lifecycle of the tunnel.
type ConnectorTunnel struct {
	log       *slog.Logger
	startedAt time.Time
	config    *config.Config

	apps []*pb.ConnectorApps
}

func NewTunnel(ctx context.Context, cfg *config.Config, tlsConfig *tls.Config, log *slog.Logger, apps []*pb.ConnectorApps) *TunnelClient {
	return &TunnelClient{
		log:       log,
		startedAt: time.Now(),
		config:    cfg,
		tlsConfig: tlsConfig,
		apps:      apps,
	}
}

func (t *TunnelClient) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		t.log.Info("dialing edge server...", "addr", t.config.Gateway.Address)

		conn, err := quic.DialAddr(ctx, t.config.Gateway.Address, t.tlsConfig, &quic.Config{
			KeepAlivePeriod: 5 * time.Second,
			MaxIdleTimeout:  10 * time.Second,
			// Allow many concurrent streams per connection
			// Each user request = one stream
			MaxIncomingStreams: 1000,
			// MaxIncomingUniStreams: -1,
		})

		if err != nil {
			t.log.Error("failed to dial edge server", "err", err)
			time.Sleep(2 * time.Second)
			continue
		}
		t.log.Info("connected to gateway server, initializing management stream")
		if err := t.handleSession(ctx, conn); err != nil {
			t.log.Warn("session terminated", "err", err)
		}
		_ = conn.CloseWithError(0x00, "reconnecting")
	}
}

func (t *TunnelClient) handleSession(ctx context.Context, conn *quic.Conn) error {
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 1. Open dedicated Management Stream
	mgmtStream, err := conn.OpenStreamSync(sessionCtx)
	if err != nil {
		return fmt.Errorf("failed to open management stream: %w", err)
	}
	defer mgmtStream.Close()

	// Send registration payload
	regMsg := proto.ControlMessage{
		MsgType:       proto.MsgType_REGISTER,
		ConnectorId:   t.config.Connector.ID,
		ConnectorApps: t.apps,
	}

	if err := json.NewEncoder(mgmtStream).Encode(&regMsg); err != nil {
		return fmt.Errorf("failed to send registration: %w", err)
	}
	t.log.Info("registration sent successfully", "connector_id", t.config.Connector.ID)

	go t.runHeartbeats(sessionCtx, mgmtStream)

	// 2. Accept server-initiated data streams
	var wg sync.WaitGroup

	errChan := make(chan error, 1)
	go func() {
		for {
			stream, err := conn.AcceptStream(sessionCtx)
			if err != nil {
				errChan <- err
			}

			wg.Add(1)
			go func(s *quic.Stream) {
				defer wg.Done()
				t.handleDataTunnel(sessionCtx, s)
			}(stream)
		}
	}()

	var sessionErr error
	select {
	case <-ctx.Done():
		sessionErr = ctx.Err()
	case err := <-errChan:
		sessionErr = err
	}

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.log.Warn("some data stream goroutines did not exit cleanly before reconnecting")
	}
	return sessionErr
}
