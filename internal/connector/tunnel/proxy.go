package tunnel

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
		// DisableCompression: true,
	},
}

// handleRequestStream reads a request from the stream,
// proxies it to the internal app, streams response back.
// Runs in its own goroutine — one per concurrent user request.
func (c *ConnectorTunnel) handleRequestStream(ctx context.Context, stream transport.Stream) {
	defer stream.Close()
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("panic in handleRequestStream", zap.Any("panic", r))
		}
	}()

	payload, err := frame.ReadFrame(stream)

	if err != nil {
		c.log.Error("failed to read connector response", zap.Error(err))
		return
	}

	var streamFrame proto.StreamFrame
	if err := frame.DecodeFrame(payload, &streamFrame); err != nil {
		c.log.Error("failed to decode connector response", zap.Error(err))
		return
	}

	switch streamFrame.StreamType {
	case proto.RequestType_HTTP_REQUEST:

		c.handleHTTPRequest(ctx, stream, &streamFrame)
	case proto.RequestType_WS_REQUEST:

		c.handleWebSocketStream(ctx, stream, &streamFrame)
	default:
		return
	}
}
