package http_proxy

import (
	"bufio"
	"context"
	"io"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
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

	var userID,userEmail, userSessionID string
	if identity != nil {
		userID = identity.UserId
		userEmail = identity.Email
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
	envelope := proto.StreamFrame{
		AppId:     appID,
		SessionId: userSessionID,
		UserId:    userID,
		UserEmail:   userEmail,
		ConnectorId: connectorID,
		Path:        r.URL.Path,
		Host:        r.Host,
		Query:       r.URL.RawQuery,
		RequestId:   requestID,
		Headers:     frame.HeadersToProto(r.Header),
		StreamType: proto.RequestType_WS_REQUEST,
		FlowType: proto.FlowType_USER_TO_APP,
	}

	if err := frame.WriteFrame(stream, &envelope); err != nil {
		h.log.Error("failed to write request header", zap.Error(err))
		http.Error(w, "failed to send request to connector", http.StatusBadGateway)
		return
	}

	hj, ok := w.(http.Hijacker)

	if !ok {
		http.Error(w, "hijack failed", 500)
	}

	clientConn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, "hijack failed", 500)
		return
	}
	defer clientConn.Close()

	if err := r.Write(stream); err != nil {
		return
	}

	go func() {
		io.Copy(stream, bufrw)
		stream.Close()
	}()

	br := bufio.NewReader(stream)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		clientConn.Write([]byte(line))
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	io.Copy(clientConn, br)
}

