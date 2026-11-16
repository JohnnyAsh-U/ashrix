package tunnel

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	ashrixerrors "github.com/JohnnyAsh-U/ashrix-api/internal/errors"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/cenkalti/backoff/v5"
	"github.com/quic-go/quic-go"
)

type TunnelClient struct {
	log       *slog.Logger
	startedAt time.Time
	config    *config.Config
	// apps      []*pb.ConnectorApps
	tlsConfig *tls.Config
}

// ConnectorTunnel manages the full lifecycle of the tunnel.
type ConnectorTunnel struct {
	log       *slog.Logger
	startedAt time.Time
	config    *config.Config

	apps []*pb.ConnectorApps
}

func NewTunnel(cfg *config.Config, tlsConfig *tls.Config, log *slog.Logger) *TunnelClient {
	return &TunnelClient{
		log:       log,
		startedAt: time.Now(),
		config:    cfg,
		tlsConfig: tlsConfig,
	}
}

func (t *TunnelClient) Run(ctx context.Context) error {

	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = 1 * time.Second
	expBackoff.MaxInterval = 30 * time.Second
	expBackoff.Multiplier = 2
	expBackoff.RandomizationFactor = 0.5
	expBackoff.Reset()

	operation := func() (bool, error) {
		// for {
		if ctx.Err() != nil {
			return false, ctx.Err()
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
			err = fromQUICError(err)
			if ashrixerrors.IsPermanent(err) {
				t.log.Error("permanent error, stopping", "code", ashrixerrors.CodeOf(err), "err", err)
				return false, backoff.Permanent(err)
			}
			t.log.Error("failed to dial edge server", "err", err)
			return false, err
		}
		t.log.Info("connected to gateway server, initializing management stream")
		if err := t.handleSession(ctx, conn); err != nil {
			err = fromQUICError(err)
			if ashrixerrors.IsPermanent(err) {
				t.log.Error("permanent error, stopping", "code", ashrixerrors.CodeOf(err), "err", err)
				return false, backoff.Permanent(err)
			}
			t.log.Warn("session terminated", "err", err)
			return false, err
		}
		t.log.Info("Session ended cleanly")
		return true, nil
	}

	notify := func(err error, next time.Duration) {
		t.log.Warn("retrying", "err", err, "retry_in", next)
	}

	_, err := backoff.Retry(ctx, operation,
		backoff.WithBackOff(expBackoff),
		backoff.WithNotify(notify),
	)
	return err
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

	apps := make([]*pb.ConnectorApps, 0, len(t.config.Applications))
	for _, app := range t.config.Applications {
		apps = append(apps, &pb.ConnectorApps{
			Id:       app.Name,
			Name:     app.Name,
			Upstream: app.Target,
		})
	}

	// Send registration payload
	regMsg := proto.ControlMessage{
		MsgType:       proto.MsgType_REGISTER,
		ConnectorId:   t.config.Connector.ID,
		ConnectorApps: apps,
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
				_ = t.handleDataTunnel(sessionCtx, s)
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

// fromQUICError converts gateway CloseWithError back to typed error
func fromQUICError(err error) error {
	var appErr *quic.ApplicationError
	if errors.As(err, &appErr) {
		return &ashrixerrors.Error{
			Code: ashrixerrors.Code(appErr.ErrorCode),
			Msg:  appErr.ErrorMessage,
		}
	}
	return err
}
