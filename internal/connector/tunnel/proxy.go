package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/quic-go/quic-go"
)

func (c *TunnelClient) handleDataTunnel(ctx context.Context, stream *quic.Stream) {

	defer func() {
		if r := recover(); r != nil {
			c.log.Error("panic in handleRequestStream", "panic", r)
		}
		if stream != nil {
			stream.Close()
		}
	}()

	if stream == nil {
		return
	}

	if ctx.Err() != nil {
		c.log.Info("Context Cancelled")
		return
	}

	payload, err := frame.ReadFrame(stream)

	if err != nil {
		c.log.Error("failed to read connector response", "error", err)
		return
	}

	var req proto.StreamFrame
	if err := frame.DecodeFrame(payload, &req); err != nil {
		c.log.Error("failed to decode connector response", "error", err)
		return
	}

	//===============Getting the app from the applist using requestheader app id================//
	var requestApp *proto.ConnectorApps
	for i := range c.apps {
		if c.apps[i].Id == req.AppId {
			requestApp = c.apps[i]
			break
		}
	}

	if requestApp == nil {
		c.log.Error("App Not Found")
		return
	}

	c.log.Info("proxying request",
		"method", req.Method,
		"path", req.Path,
		"appName", requestApp.Name,
	)

	dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dialCancel()

	upstream := fmt.Sprintf("%s", requestApp.Upstream)


	appConn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", upstream)


	// appConn, err := net.Dial("tcp", upstream)
	if err != nil {
		c.log.Error("failed to send request", "error", err)
		return
	}
	defer appConn.Close()

	// ── Call internal app ─────────────────────────────────────────
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(2)


	// Quic stream(gateway) to tcp backend
	go func ()  {
		defer wg.Done()
		_, _ = io.Copy(appConn, stream)
		if tc, ok := appConn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()

	// Quic stream(gateway) to tcp backend
	go func ()  {
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
}
