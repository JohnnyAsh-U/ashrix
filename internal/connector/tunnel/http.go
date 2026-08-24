package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	// "net/http"
	"time"

	"go.uber.org/zap"

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
		zap.String("method", request.Method),
		zap.String("path", request.Path),
		zap.String("user", request.UserEmail),
		zap.String("appName", requestApp.Name),
	)

	upstream := fmt.Sprintf("%s", requestApp.Upstream)

	appConn, err := net.Dial("tcp", upstream)
	if err != nil {
		c.log.Error("failed to send request", zap.Error(err))
		return
	}

	// ── Call internal app ─────────────────────────────────────────
	start := time.Now()

	go io.Copy(appConn, stream)

	c.log.Debug("request complete",
		zap.String("method", request.Method),
		zap.String("path", request.Path),
		zap.Duration("latency", time.Since(start)),
		zap.Error(err),
	)

	defer appConn.Close()
	defer stream.Close()

	io.Copy(stream, appConn)

}

// func (c *ConnectorTunnel) writeError(stream transport.Stream, requestID string, code int, msg string) {
// 	body := []byte(msg)
// 	header := gen.HTTPResponse{
// 		RequestId:  requestID,
// 		StatusCode: int32(code),
// 		BodyLength: int64(len(body)),
// 		Headers:    frame.HeadersToProto(nil),
// 	}
// 	if err := frame.WriteFrame(stream, proto.FrameType_FRAME_TYPE_HTTP_RESPONSE, &header); err != nil {
// 		c.log.Error("failed to write error envelope", zap.Error(err))
// 		return
// 	}
// 	if _, err := stream.Write(body); err != nil {
// 		c.log.Error("failed to write error body", zap.Error(err))
// 	}
// }
