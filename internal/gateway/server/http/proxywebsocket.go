package http_proxy

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
)

func (h *Handler) proxyWebSocket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	appID := session.AppIDFromCtx(ctx)
	identity := session.IdentityFromCtx(ctx)
	sessionID := session.SessionIDFromCtx(ctx)
	requestID := uuid.NewString()

	var userID, userEmail string
	if identity != nil {
		userID = identity.UserId
		userEmail = identity.Email
	}

	streamCtx, cancel := context.WithCancel(ctx)
	streamID := uuid.NewString()
	h.log.Debug("Opening tunnel stream", slog.String("stream_id", streamID), slog.String("app_id", appID))

	h.streamRegistry.Register(sessionID, streamID, cancel)

	defer func() {
		cancel()
		h.streamRegistry.Unregister(sessionID, streamID)
		h.log.Debug("Tunnel stream closed", slog.String("stream_id", streamID), slog.String("app_id", appID))
	}()

	req := &proto.StreamFrame{
		RequestId:    requestID,
		FlowType:     proto.FlowType_USER_TO_APP,
		// TenantId:     identity.TenantId,
		SessionId:    sessionID,
		SourceId:     userID,
		SourceEmail:  userEmail,
		DestAppId:    appID,
		Method:       r.Method,
		ProtocolType: proto.ProtocolType_PROTOCOL_TCP,
		Host:         r.Host,
		Path:         r.URL.Path,
		Query: r.URL.RawQuery,
		Headers:      frame.HeadersToProto(r.Header),
		BodyLength:   r.ContentLength,
		StreamType:   proto.RequestType_WS_REQUEST,
	}

	stream, err := h.router.Route(streamCtx, req, false)
	if err != nil {
		h.log.Warn("Routing failed", slog.String("app_id", appID), slog.Any("error", err))
		h.errorHandler.ErrorPage(w, http.StatusBadGateway, "Bad Gateway", "The gateway encountered an unexpected error.", "")
		return
	}
	defer stream.Close()

	hj, ok := w.(http.Hijacker)
	if !ok {
		h.errorHandler.ErrorPage(w, http.StatusInternalServerError, "Internal Server Error", "The gateway encountered an unexpected error.", "")
		return
	}

	clientConn, bufrw, err := hj.Hijack()
	if err != nil {
		h.errorHandler.ErrorPage(w, http.StatusInternalServerError, "Internal Server Error", "The gateway encountered an unexpected error.", "")
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
