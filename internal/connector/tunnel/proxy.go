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
		MaxIdleConns:       100,
		IdleConnTimeout:    90 * time.Second,
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
			c.writeError(stream, "", 500, "internal server error due to panic")
		}
	}()

	typ, payload, err := frame.ReadFrame(stream)


	if err != nil {
		c.log.Error("failed to read connector response", zap.Error(err))
		c.writeError(stream, "", 502, "connector response error")
		return
	}

	switch typ {
		case frame.FrameType(proto.FrameType_FRAME_TYPE_HTTP_REQUEST):
			var httpRequest proto.HTTPRequest
			if err := frame.DecodeFrame(payload, &httpRequest); err != nil {
				c.log.Error("failed to decode connector response", zap.Error(err))
				c.writeError(stream, "", 502, "invalid connector response")
				return
			}
			c.handleHTTPRequest(ctx, stream, &httpRequest)
		case frame.FrameType(proto.FrameType_FRAME_TYPE_WS_OPEN):
			var wsOpen proto.WSOpen
			if err := frame.DecodeFrame(payload, &wsOpen); err != nil {
				c.log.Error("failed to decode connector response", zap.Error(err))
				c.writeError(stream, "", 502, "invalid connector response")
				return
			}
			c.handleWebSocketStream(ctx, stream, &wsOpen)
		default:
			c.writeError(stream, "", 502, "invalid connector response")
			return
	}
}
