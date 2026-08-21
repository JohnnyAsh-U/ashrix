package tunnel

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:       100,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: true,
	},
}

// handleRequestStream reads a request from the stream,
// proxies it to the internal app, streams response back.
// Runs in its own goroutine — one per concurrent user request.
func (c *ConnectorTunnel) handleRequestStream(stream transport.Stream) {
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

	if typ != frame.FrameType(proto.FrameType_FRAME_TYPE_HTTP_REQUEST) {
		c.writeError(stream, "", 502, "invalid connector response")
		return
	}

	var request proto.HTTPRequest

	if err := frame.DecodeFrame(payload, &request); err != nil {
		c.log.Error("failed to decode connector response", zap.Error(err))
		c.writeError(stream, request.RequestId, 502, "invalid connector response")
		return
	}

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
		c.writeError(stream, request.RequestId, 502, "App Not Found")
		return
	}

	c.log.Debug("proxying request",
		zap.String("method", request.Method),
		zap.String("path", request.Path),
		zap.String("user", request.UserEmail),
		zap.String("appName", requestApp.Name),
	)

	// ── Build upstream URL ────────────────────────────────────────
	upstream := fmt.Sprintf("%s://%s%s", requestApp.Protocol, requestApp.Upstream, request.Path)
	if request.Query != "" {
		upstream += "?" + request.Query
	}

	// ── Build HTTP request ────────────────────────────────────────
	req, err := http.NewRequest(request.Method, upstream, stream)
	if err != nil {
		c.log.Error("failed to build request", zap.Error(err))
		c.writeError(stream, request.RequestId, 500, "internal error")
		return
	}

	// // ADD THIS LOOP: Map the protobuf headers into the upstream HTTP request
	if request.Headers != nil {
		for key, headerList := range request.Headers {
			if headerList != nil {
				req.Header[key] = headerList.Values
			}
		}
	}

	// Inject verified identity headers
	// Internal app trusts these — they come from the connector,
	// not from the user directly
	req.Header.Set("X-Ashrix-User-ID", request.UserId)
	req.Header.Set("X-Ashrix-User-Email", request.UserEmail)
	// req.Header.Set("X-Ashrix-Request-ID", envelope.RequestID)

	// Strip headers user should not control
	req.Header.Del("X-Forwarded-For")
	req.Host = "localhost"
	// ── Call internal app ─────────────────────────────────────────
	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		c.log.Error("upstream unreachable",
			zap.Error(err),
		)
		c.writeError(stream, request.RequestId, 502, "upstream unreachable")
		return
	}
	defer resp.Body.Close()

	response := gen.HTTPResponse{
		RequestId:  request.RequestId,
		StatusCode: int32(resp.StatusCode),
		BodyLength: resp.ContentLength,
		Headers:    frame.HeadersToProto(resp.Header),
	}
	if err := frame.WriteFrame(stream, proto.FrameType_FRAME_TYPE_HTTP_RESPONSE, &response); err != nil {
		c.log.Error("failed to write response envelope",
			zap.Error(err),
		)
		return
	}

	// ── Stream response body ──────────────────────────────────────
	// Bytes flow: app → connector → stream → gateway → user
	// No buffering — works for any response size
	written, err := io.Copy(stream, resp.Body)

	c.log.Debug("request complete",
		zap.String("method", request.Method),
		zap.String("path", request.Path),
		zap.Int("status", resp.StatusCode),
		zap.Int64("bytes", written),
		zap.Duration("latency", time.Since(start)),
		zap.Error(err),
	)
}

func (c *ConnectorTunnel) writeError(stream transport.Stream, requestID string, code int, msg string) {
	body := []byte(msg)
	header := gen.HTTPResponse{
		RequestId:  requestID,
		StatusCode: int32(code),
		BodyLength: int64(len(body)),
		Headers:    frame.HeadersToProto(nil),
	}
	if err := frame.WriteFrame(stream, proto.FrameType_FRAME_TYPE_HTTP_RESPONSE, &header); err != nil {
		c.log.Error("failed to write error envelope", zap.Error(err))
		return
	}
	if _, err := stream.Write(body); err != nil {
		c.log.Error("failed to write error body", zap.Error(err))
	}
}
