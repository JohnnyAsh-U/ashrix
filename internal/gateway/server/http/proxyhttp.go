package http_proxy

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"maps"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
)

func (h *Handler) proxyHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	AppID := session.AppIDFromCtx(ctx)
	Identity := session.IdentityFromCtx(ctx)
	sessionID := session.SessionIDFromCtx(ctx)
	requestID := uuid.NewString()

	var userID, userEmail string
	if Identity != nil {
		userID = Identity.UserId
		userEmail = Identity.Email
	}

	streamCtx, cancel := context.WithCancel(ctx)
	streamID := uuid.NewString()
	h.log.Debug("Opening tunnel stream", slog.String("stream_id", streamID), slog.String("app_id", AppID))

	h.streamRegistry.Register(sessionID, streamID, cancel)

	defer func() {
		cancel()
		h.streamRegistry.Unregister(sessionID, streamID)
		h.log.Debug("Tunnel stream closed", slog.String("stream_id", streamID), slog.String("app_id", AppID))
	}()

	req := &proto.StreamFrame{
		RequestId:   requestID,
		FlowType: proto.FlowType_USER_TO_APP,
		// TenantId: Identity.TenantId,
		SessionId: sessionID,
		SourceId: userID,
		SourceEmail: userEmail,
		DestAppId: AppID,
		Method: r.Method,
		ProtocolType: proto.ProtocolType_PROTOCOL_TCP,
		Host: r.Host,
		Path: r.URL.Path,
		Query: r.URL.RawQuery,
		Headers: frame.HeadersToProto(r.Header),
		BodyLength:  r.ContentLength,
		StreamType:  proto.RequestType_HTTP_REQUEST,
	}

	stream, err := h.router.Route(streamCtx, req, false)
	if err != nil {
		h.log.Warn("Routing failed", slog.String("app_id", AppID), slog.Any("error", err))
		h.errorHandler.ErrorPage(w, http.StatusBadGateway, "Bad Gateway", "The gateway encountered an unexpected error.", "")
		return
	}
	defer stream.Close()

	if err := r.Write(stream); err != nil {
		h.log.Error("failed to write request to stream", slog.Any("error", err))
		h.errorHandler.ErrorPage(w, http.StatusBadGateway, "Bad Gateway", "The gateway encountered an unexpected error.", "")
		return
	}

	br := bufio.NewReader(stream)

	resp, err := http.ReadResponse(br, r)
	if err != nil {
		h.errorHandler.ErrorPage(w, http.StatusBadGateway, "Bad Gateway", "The gateway encountered an unexpected error.", "")
		return
	}

	maps.Copy(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
	resp.Body.Close()
}
