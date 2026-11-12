package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	// "net/http"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	// "github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

// handleHTTPRequest reads an HTTP request from the stream,
// proxies it to the gateway.
// Runs in its own goroutine — one per concurrent user request.
func (c *ConnectorTunnel) handleHTTPRequest(ctx context.Context, stream transport.Stream, request *proto.StreamFrame) {

	//===============Getting the app from the applist using requestheader app id================//
	var requestApp *gen.ConnectorApps
	for i := range c.apps {
		if c.apps[i].Id == request.AppId {
			requestApp = c.apps[i]
			break
		}
	}

	if requestApp == nil {
		c.log.Error("App Not Found")
		return
	}

	c.log.Debug("proxying request",
		"method", request.Method,
		"path", request.Path,
		"appName", requestApp.Name,
	)

	upstream := fmt.Sprintf("%s", requestApp.Upstream)

	appConn, err := net.Dial("tcp", upstream)
	if err != nil {
		c.log.Error("failed to send request", "error", err)
		return
	}

	// ── Call internal app ─────────────────────────────────────────
	start := time.Now()

	go io.Copy(appConn, stream)

	c.log.Debug("request complete",
		"method", request.Method,
		"path", request.Path,
		"latency", time.Since(start),
		"error", err,
	)

	defer appConn.Close()
	defer stream.Close()

	io.Copy(stream, appConn)

}

