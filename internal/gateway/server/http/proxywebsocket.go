package http_proxy

import (
	// "context"
	"context"
	"fmt"
	"io"
	"net/http"

	// proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (h *Handler) proxyWebSocket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	appID := session.AppIDFromCtx(ctx)
	identity := session.IdentityFromCtx(ctx)
	sessionID := session.SessionIDFromCtx(ctx)
	connectorID := session.ConnectorIDFromCtx(ctx)
	requestID := uuid.NewString()

	var userID, userSessionID string
	if identity != nil {
		userID = identity.UserId
		// userEmail = identity.Email
	}

	if sessionID == "" {
		userSessionID = "no-session"
	}

	entry, ok := h.registry.GetByConnectorID(connectorID)
	if !ok || entry == nil {
		h.log.Warn("no connector for this subdomain")
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}

	if !entry.IsRoutable() {
		http.Error(w, "connector unavailable", http.StatusBadGateway)
		return
	}

	streamCtx, cancel := context.WithCancel(ctx)
	streamID := uuid.NewString()
	h.log.Debug("Opening tunnel stream", zap.String("stream_id", streamID), zap.String("connector_id", entry.ConnectorID))

	h.streamRegistry.Register(sessionID, streamID, cancel)

	defer func() {
		cancel()
		h.streamRegistry.Unregister(sessionID, streamID)
		h.log.Debug("Tunnel stream closed", zap.String("stream_id", streamID), zap.String("connector_id", entry.ConnectorID))
	}()

	//Get Tunnel Session
	tunnel, ok := h.registry.GetTunnelSession(entry.ConnectorID)
	if !ok || tunnel == nil {
		h.log.Warn("no tunnel session for this connector")
		http.Error(w, "connector unavailable", http.StatusBadGateway)
		return
	}
	stream, err := tunnel.OpenStream(streamCtx)
	if err != nil {
		h.log.Warn("Failed to open tunnel stream", zap.String("connector_id", entry.ConnectorID), zap.Error(err))
		http.Error(w, "tunnel error", http.StatusBadGateway)
		return
	}
	defer stream.Close()

	// Write request envelope + body to stream
	envelope := proto.WSOpen{
		AppId:     appID,
		SessionId: userSessionID,
		UserId:    userID,
		// UserEmail:   userEmail,
		ConnectorId: connectorID,
		Path:        r.URL.Path,
		Host:        r.Host,
		Query:       r.URL.RawQuery,
		RequestId:   requestID,
		Headers:     frame.HeadersToProto(r.Header),
	}

	if err := frame.WriteFrame(stream, proto.FrameType_FRAME_TYPE_WS_OPEN, &envelope); err != nil {
		h.log.Error("failed to write request header", zap.Error(err))
		http.Error(w, "failed to send request to connector", http.StatusBadGateway)
		return
	}

	typ, payload, err := frame.ReadFrame(stream)
	if err != nil {
		h.log.Error("failed to read response header", zap.Error(err))
		http.Error(w, "failed to receive response from connector", http.StatusBadGateway)
		return
	}

	if typ != frame.FrameType(proto.FrameType_FRAME_TYPE_WS_OPEN_RESPONSE) {
		h.log.Error("unexpected frame type", zap.String("type", string(typ)))
		http.Error(w, "unexpected response from connector", http.StatusBadGateway)
		return
	}

	var wsOpenResp proto.WSOpenResponse
	if err := frame.DecodeFrame(payload, &wsOpenResp); err != nil {
		h.log.Error("failed to decode response header", zap.Error(err))
		http.Error(w, "failed to process response from connector", http.StatusBadGateway)
		return
	}

	if wsOpenResp.StatusCode != http.StatusSwitchingProtocols {
		h.log.Error("unexpected response status", zap.Int("status_code", int(wsOpenResp.StatusCode)))
		http.Error(w, "unexpected response from connector", http.StatusBadGateway)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		stream.Close()
		h.log.Error("Failed to upgrade websocket connection", zap.Error(err))
		h.log.Warn("Tunnel stream closed", zap.String("stream_id", streamID), zap.String("connector_id", entry.ConnectorID))
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "closed")

	err = proxyWebSocketData(ctx, conn, stream, h.log)

	if err != nil && streamCtx.Err() == nil {
		h.log.Error("websocket copy loop failed", zap.Error(err))
	}
}

func proxyWebSocketData(ctx context.Context, client *websocket.Conn, stream io.ReadWriteCloser, log *zap.Logger) error {

	//read data from conn and write to stream
	errCh := make(chan error, 2)

	go func() {
		err := websocketToTunnel(ctx, client, stream, log)
		errCh <- err
	}()

	go func() {
		err := tunnelToWebsocket(ctx, client, stream, log)
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		_ = client.Close(websocket.StatusPolicyViolation, "session closed")
		_ = stream.Close()
		log.Warn("Tunnel stream closed")
		return ctx.Err()

	case err := <-errCh:
		_ = client.Close(websocket.StatusAbnormalClosure, "connection closed")
		_ = stream.Close()
		log.Warn("Tunnel stream closed", zap.Error(err))
		return err
	}
}

func websocketToTunnel(ctx context.Context, client *websocket.Conn, stream io.ReadWriteCloser, log *zap.Logger) error {
	for {
		messageType, reader, err := client.Reader(ctx)
		if err != nil {
			return err
		}

		data, err := io.ReadAll(io.LimitReader(reader, 16<<20))
		if err != nil {
			return err
		}

		wsFrame := &proto.WSData{
			MessageType: int32(messageType),
			Data:        data,
		}

		if err := frame.WriteFrame(stream, proto.FrameType_FRAME_TYPE_WS_DATA, wsFrame); err != nil {
			return err
		}
	}
}

func tunnelToWebsocket(ctx context.Context, client *websocket.Conn, stream io.ReadWriteCloser, log *zap.Logger) error {
	for {
		typ, payload, err := frame.ReadFrame(stream)
		if err != nil {
			return err
		}

		switch typ {
		case frame.FrameType(proto.FrameType_FRAME_TYPE_WS_DATA):
			var wsFrame proto.WSData
			if err := frame.DecodeFrame(payload, &wsFrame); err != nil {
				return err
			}

			messageType := websocket.MessageType(wsFrame.MessageType)

			writer, err := client.Writer(ctx, messageType)
			if err != nil {
				return err
			}

			if _, err := writer.Write(wsFrame.Data); err != nil {
				return err
			}

			if err := writer.Close(); err != nil {
				return err
			}

		case frame.FrameType(proto.FrameType_FRAME_TYPE_WS_CLOSE):
			var wsClose proto.WSClose

			if err := frame.DecodeFrame(payload, &wsClose); err != nil {
				return err
			}

			_ = client.Close(websocket.StatusCode(wsClose.Code), wsClose.Reason)
			return nil
		default:
			return fmt.Errorf("unexpected frame type: %v", typ)
		}
	}
}
