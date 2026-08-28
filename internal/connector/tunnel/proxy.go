package tunnel

import (
	"context"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)


// handleRequestStream reads a request from the stream,
// proxies it to the internal app, streams response back.
// Runs in its own goroutine — one per concurrent user request.
func (c *ConnectorTunnel) handleRequestStream(ctx context.Context, stream transport.Stream) {
	defer stream.Close()
	defer func() {
		c.managementConn.ActiveStreams.Add(-1)
		if r := recover(); r != nil {
			c.log.Error("panic in handleRequestStream", "panic", r)
		}
	}()

	payload, err := frame.ReadFrame(stream)

	if err != nil {
		c.log.Error("failed to read connector response", "error", err)
		return
	}

	var streamFrame proto.StreamFrame
	if err := frame.DecodeFrame(payload, &streamFrame); err != nil {
		c.log.Error("failed to decode connector response", "error", err)
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
