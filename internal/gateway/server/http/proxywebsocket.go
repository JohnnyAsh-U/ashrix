package http_proxy

import (
	"bufio"
	"context"
	"io"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/flow"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (h *Handler) proxyWebSocket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	appID := session.AppIDFromCtx(ctx)
	identity := session.IdentityFromCtx(ctx)
	sessionID := session.SessionIDFromCtx(ctx)
	requestID := uuid.NewString()

	var userID string
	if identity != nil {
		userID = identity.UserId
	}

	streamCtx, cancel := context.WithCancel(ctx)
	streamID := uuid.NewString()
	h.log.Debug("Opening tunnel stream", zap.String("stream_id", streamID), zap.String("app_id", appID))

	h.streamRegistry.Register(sessionID, streamID, cancel)

	defer func() {
		cancel()
		h.streamRegistry.Unregister(sessionID, streamID)
		h.log.Debug("Tunnel stream closed", zap.String("stream_id", streamID), zap.String("app_id", appID))
	}()

	req := flow.OpenRequest{
		Version:  1,
		FlowID:   requestID,
		FlowType: flow.FlowUserToApp,
		Protocol: flow.ProtocolTCP,
		Source: flow.Endpoint{
			Type:        flow.EndpointUser,
			PrincipalID: userID,
			DeviceID:    "no-agent",
		},
		Destination: flow.Endpoint{
			Type:  flow.EndpointApp,
			AppID: appID,
		},
		HTTPMethod:  r.Method,
		HTTPPath:    r.URL.Path,
		HTTPHost:    r.Host,
		HTTPQuery:   r.URL.RawQuery,
		HTTPHeaders: r.Header,
		BodyLength:  r.ContentLength,
		StreamType:  int32(proto.RequestType_WS_REQUEST),
	}

	stream, err := h.router.Route(streamCtx, req)
	if err != nil {
		h.log.Warn("Routing failed", zap.String("app_id", appID), zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer stream.Close()

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack failed", 500)
		return
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
