package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/coder/websocket"

	// "github.com/gorilla/websocket"
	"go.uber.org/zap"
)

func (c *ConnectorTunnel) handleWebSocketStream(ctx context.Context, stream transport.Stream, request *proto.WSOpen) {
	//===============Getting the app from the applist using requestheader app id================//
	var requestApp *proto.ConnectorApps
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

	// ── Build upstream URL ────────────────────────────────────────
	upstream := fmt.Sprintf("ws://%s%s", requestApp.Upstream, request.Path)
	if request.Query != "" {
		upstream += "?" + request.Query
	}

	// ADD THIS LOOP: Map the protobuf headers into the upstream HTTP request
	var req http.Request
	if request.Headers != nil {
		for key, headerList := range request.Headers {
			if headerList != nil {
				req.Header[key] = headerList.Values
			}
		}
	}

	// ── Build WS request ────────────────────────────────────────
	conn, _, err := websocket.Dial(ctx, upstream, &websocket.DialOptions{
		HTTPHeader: req.Header,
	})

	if err != nil {
		c.log.Error("failed to connect to websocket", zap.Error(err))
		_ = frame.WriteFrame(stream, proto.FrameType_FRAME_TYPE_ERROR, &proto.WSOpenResponse{
			RequestId:  request.RequestId,
			StatusCode: http.StatusBadGateway,
			Headers:    frame.HeadersToProto(nil),
		})
		return
	}

	defer conn.Close(websocket.StatusNormalClosure, "connection closed by upstream")

	// ── Send successful response back to gateway ──────────────────
	if err := frame.WriteFrame(stream, proto.FrameType_FRAME_TYPE_WS_OPEN_RESPONSE, &proto.WSOpenResponse{
		RequestId:  request.RequestId,
		StatusCode: http.StatusSwitchingProtocols,
	}); err != nil {
		c.log.Error("failed to send WS open response", zap.Error(err))
		return
	}

	c.log.Debug("proxying websocket",
		zap.String("path", request.Path),
		zap.String("user", request.UserId),
		zap.String("appName", requestApp.Name),
	)

	proxyConnectorWebSocket(ctx, stream, conn)	
}


func proxyConnectorWebSocket(ctx context.Context, stream transport.Stream, conn *websocket.Conn) error {
	errCh := make(chan error, 2)

	go func(){
		errCh <- connectorToTunnel(ctx, stream, conn)
	}()

	go func(){
		errCh <- tunnelToConnector(ctx, stream, conn)
	}()

	select {
	case err := <-errCh:
		_ = conn.Close(websocket.StatusNormalClosure, "connection closed by upstream")
		_ = stream.Close()
		return err
	case <-ctx.Done():
		_ = conn.Close(websocket.StatusPolicyViolation, "connection closed by upstream")
		_ = stream.Close()
		return ctx.Err()
	}
}


func connectorToTunnel(ctx context.Context, stream transport.Stream, conn *websocket.Conn) error {
	for {
		msgType, r, err := conn.Reader(ctx)
		if err != nil {
			return err
		}

		data, err := io.ReadAll(io.LimitReader(r, 16<<20))
		if err != nil {
			return err
		}

		if err := frame.WriteFrame(stream, proto.FrameType_FRAME_TYPE_WS_DATA, &proto.WSData{
			MessageType: int32(msgType),
			Data: data,
		}); err != nil {
			return err
		}
	}
}


func tunnelToConnector(ctx context.Context, stream transport.Stream, conn *websocket.Conn) error {
	for {
		typ, payload, err := frame.ReadFrame(stream)
		if err != nil {
			return err
		}

		switch typ {
		case frame.FrameType(proto.FrameType_FRAME_TYPE_WS_DATA):
			var wsData proto.WSData
			if err := frame.DecodeFrame(payload, &wsData); err != nil {
				return err
			}
			if err := conn.Write(ctx, websocket.MessageType(wsData.MessageType), wsData.Data); err != nil {
				return err
			}
		case frame.FrameType(proto.FrameType_FRAME_TYPE_WS_PING):
			//Do nothing
		case frame.FrameType(proto.FrameType_FRAME_TYPE_WS_PONG):
			// Do nothing
		case frame.FrameType(proto.FrameType_FRAME_TYPE_WS_CLOSE):
			var wsClose proto.WSClose
			if err := frame.DecodeFrame(payload, &wsClose); err != nil {
				return err
			}
			if err := conn.Close(websocket.StatusCode(wsClose.Code), wsClose.Reason); err != nil {
				return err
			}
		default:
			return errors.New("invalid frame type")
		}
	}
}





