package http_proxy

import (
	"bufio"
	"context"
	"io"
	"maps"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/frame"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
	"go.uber.org/zap"
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
	h.log.Debug("Opening tunnel stream", zap.String("stream_id", streamID), zap.String("app_id", AppID))

	h.streamRegistry.Register(sessionID, streamID, cancel)

	defer func() {
		cancel()
		h.streamRegistry.Unregister(sessionID, streamID)
		h.log.Debug("Tunnel stream closed", zap.String("stream_id", streamID), zap.String("app_id", AppID))
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
		h.log.Warn("Routing failed", zap.String("app_id", AppID), zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer stream.Close()

	if err := r.Write(stream); err != nil {
		h.log.Error("failed to write request to stream", zap.Error(err))
		http.Error(w, "failed to send request to connector", http.StatusBadGateway)
		return
	}

	br := bufio.NewReader(stream)

	resp, err := http.ReadResponse(br, r)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}

	maps.Copy(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
	resp.Body.Close()
}
