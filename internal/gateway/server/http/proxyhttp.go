package http_proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

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
	connectorID := session.ConnectorIDFromCtx(ctx)
	requestID := uuid.NewString()

	var userID, userEmail, userSessionID string
	if Identity != nil {
		userID = Identity.UserId
		userEmail = Identity.Email
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
	envelope := proto.HTTPRequest{
		Method:      r.Method,
		AppId:       AppID,
		SessionId:   userSessionID,
		UserId:      userID,
		UserEmail:   userEmail,
		ConnectorId: connectorID,
		Path:        r.URL.Path,
		Host:        r.Host,
		Query:       r.URL.RawQuery,
		RequestId:   requestID,
		BodyLength:  r.ContentLength,
		Headers:     frame.HeadersToProto(r.Header),
	}

	if err := frame.WriteFrame(stream, proto.FrameType_FRAME_TYPE_HTTP_REQUEST, &envelope); err != nil {
		h.log.Error("failed to write request header", zap.Error(err))
		http.Error(w, "failed to send request to connector", http.StatusBadGateway)
		return
	}

	if r.Body != nil {
		if err := h.copyRequestBody(streamCtx, stream, r.Body); err != nil {
			h.log.Error("failed to write request body", zap.Error(err))
			// Differentiate between a 413 Payload Too Large and a 502 Stream Error
			if err.Error() == "request body exceeds limit" {
				http.Error(w, "Payload Too Large", http.StatusRequestEntityTooLarge)
			} else {
				http.Error(w, "failed to send request to connector", http.StatusBadGateway)
			}
			return
		}
	}

	typ, payload, err := frame.ReadFrame(stream)

	if err != nil {
		if streamCtx.Err() != nil {
			return
		}
		h.log.Error("failed to read connector response", zap.Error(err))
		http.Error(w, "connector response error jjjj", http.StatusBadGateway)
		return
	}

	if typ != frame.FrameType(proto.FrameType_FRAME_TYPE_HTTP_RESPONSE) {
		http.Error(w, "invalid connector response", http.StatusBadGateway)
		return

	}

	var response proto.HTTPResponse

	if err := frame.DecodeFrame(payload, &response); err != nil {
		http.Error(w, "invalid connector response", http.StatusBadGateway)
		return
	}

	if response.StatusCode < 100 || response.StatusCode > 599 {
		http.Error(w, "invalid connector status", http.StatusBadGateway)
		return
	}

	copyResponseHeaders(w.Header(), response.Headers)

	w.WriteHeader(int(response.StatusCode))

	if responseBodyAllowed(r, int(response.StatusCode)) {
		written, err := io.Copy(w, stream)

		h.log.Debug(
			"HTTP response body completed",
			zap.String("request_id", requestID),
			zap.Int64("bytes_written", written),
			zap.Int64("expected_bytes", response.BodyLength),
			zap.Error(err),
		)
	}
}

func (h *Handler) copyRequestBody(ctx context.Context, dst io.Writer, src io.Reader) error {

	reader := io.Reader(src)

	if h.maxRequestBody > 0 {
		reader = io.LimitReader(src, h.maxRequestBody+1)
	}
	n, err := io.Copy(dst, reader)

	if err != nil {
		return err
	}

	if h.maxRequestBody > 0 && n > h.maxRequestBody {
		return fmt.Errorf("request body exceeds limit")
	}
	return nil
}

func responseBodyAllowed(r *http.Request, status int) bool {

	if r.Method == http.MethodHead {
		return false
	}

	if status >= 100 && status < 200 {
		return false
	}

	if status == http.StatusNoContent {
		return false
	}

	if status == http.StatusNotModified {
		return false
	}

	return true
}
func copyResponseHeaders(dst http.Header, src map[string]*proto.HeaderList) {
	for key, headerList := range src {
		// Safety check in case a map value is nil
		if headerList == nil {
			continue
		}

		if isHopByHopHeader(key) {
			continue
		}
		if isHopByHopHeader(key) || isGatewayOwnedHeader(key) {
			continue
		}
		// Loop through the slice inside the Protobuf wrapper
		for _, value := range headerList.Values {
			dst.Set(key, value)
		}
	}
}

func isGatewayOwnedHeader(name string) bool {
	switch strings.ToLower(name) {
	case
		// "content-security-policy",
		// "strict-transport-security",
		// "x-content-type-options",
		// "x-frame-options",
		// "referrer-policy",
		"permissions-policy":
		return true
	default:
		return false
	}
}

func isHopByHopHeader(name string) bool {
	switch strings.ToLower(name) {
	case
		"connection",
		"keep-alive",
		"proxy-authenticate",
		"proxy-authorization",
		"te",
		"trailer",
		"transfer-encoding",
		"upgrade":
		return true

	default:
		return false
	}
}
