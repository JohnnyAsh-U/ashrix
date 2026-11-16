package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/quic-go/quic-go"
)

var (
	ErrStreamClosed = errors.New("Stream Closed")
	ErrConnectorID  = errors.New("Connector Mismatch")
	ErrNoStream     = errors.New("No Stream")
	ErrAppNotFound  = errors.New("App Not Found")
	ErrAppRequest   = errors.New("App Request Failed")
)

func (c *TunnelClient) handleDataTunnel(ctx context.Context, stream *quic.Stream) error {

	defer func() {
		if r := recover(); r != nil {
			c.log.Error("panic in handleRequestStream", "panic", r)
		}
		if stream != nil {
			stream.Close()
		}
	}()

	if stream == nil {
		return ErrNoStream
	}

	if ctx.Err() != nil {
		c.log.Info("Context Cancelled")
		return ctx.Err()
	}

	payload, err := frame.ReadFrame(stream)

	if err != nil {
		c.log.Error("failed to read connector response", "error", err)
		return err
	}

	var req proto.StreamFrame
	if err := frame.DecodeFrame(payload, &req); err != nil {
		c.log.Error("failed to decode connector response", "error", err)
		return err
	}

	// Check if the connector id matches
	if req.ConnectorId == "" || req.ConnectorId != c.config.Connector.ID {
		c.log.Error("Connector ID Mismatched")
		return ErrConnectorID
	}

	//===============Getting the app from the applist using requestheader app id================//
	var requestApp *config.Application
	for _, app := range c.config.Applications {
		if app.Name == req.AppId {
			requestApp = &app
			break
		}
	}

	if requestApp == nil {
		c.log.Error("App Not Found")
		return ErrAppNotFound
	}

	c.log.Info("proxying request",
		"method", req.Method,
		"path", req.Path,
		"appName", requestApp.Name,
	)

	dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dialCancel()

	upstream := fmt.Sprintf("%s", requestApp.Target)

	appConn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", upstream)

	// appConn, err := net.Dial("tcp", upstream)
	if err != nil {
		c.log.Error("failed to send request", "error", err)
		return ErrAppRequest
	}
	defer appConn.Close()

	// ── Call internal app ─────────────────────────────────────────
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(2)

	// Quic stream(gateway) to tcp backend
	go func() {
		defer wg.Done()
		_, _ = io.Copy(appConn, stream)
		if tc, ok := appConn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()

	// Quic stream(gateway) to tcp backend
	go func() {
		defer wg.Done()
		_, _ = io.Copy(stream, appConn)

		_ = stream.Close()
	}()

	wg.Wait()
	// go io.Copy(appConn, stream)

	c.log.Info("request complete",
		"method", req.Method,
		"path", req.Path,
		"latency", time.Since(start),
		"error", err,
	)
	return nil
}
