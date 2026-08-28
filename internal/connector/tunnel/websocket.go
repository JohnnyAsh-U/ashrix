package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

func (c *ConnectorTunnel) handleWebSocketStream(ctx context.Context, stream transport.Stream, request *proto.StreamFrame) {
	//===============Getting the app from the applist using requestheader app id================//
	var requestApp *proto.ConnectorApps
	for i := range c.apps {
		if c.apps[i].Id == request.DestAppId {
			requestApp = c.apps[i]
			break
		}
	}

	if requestApp == nil {
		c.log.Error("App Not Found")
		return
	}

	// // ── Build upstream URL ────────────────────────────────────────
	upstream := fmt.Sprintf("%s", requestApp.Upstream)

	appConn, err := net.Dial("tcp", upstream)
	start := time.Now()

	c.log.Debug("proxying websocket",
		"path", request.Path,
		"user", request.SourceId,
		"appName", requestApp.Name,
	)

	go io.Copy(appConn, stream)

	c.log.Debug("request complete",
		"path", request.Path,
		"latency", time.Since(start),
		"error", err,
	)
	io.Copy(stream, appConn)
}

