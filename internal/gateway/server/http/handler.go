package http_proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/pkg/crypto"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	gateway_grpc "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/grpc_client"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	"go.uber.org/zap"
)

type Handler struct {
	log            *zap.Logger
	registry       *registry.Registry
	session        *session.SessionManager
	rateLimiter    *session.RedisLimiter
	redisClient    *redis.Client
	grpcClient     *gateway_grpc.SafeClient
	cfg            *config.Config
	streamRegistry *registry.ActiveStreamRegistry
}

func NewHandler(
	log *zap.Logger,
	registry *registry.Registry,
	session *session.SessionManager,
	limiter *session.RedisLimiter,
	redisClient *redis.Client,
	grpcClient *gateway_grpc.SafeClient,
	streamRegistry *registry.ActiveStreamRegistry,
	cfg *config.Config,

) *Handler {

	return &Handler{
		log:            log,
		registry:       registry,
		session:        session,
		rateLimiter:    limiter,
		redisClient:    redisClient,
		grpcClient:     grpcClient,
		cfg:            cfg,
		streamRegistry: streamRegistry,
	}
}

func (h *Handler) ProxyHandler(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	connectorID := session.ConnectorIDFromCtx(ctx)
	AppID := session.AppIDFromCtx(ctx)
	Identity := session.IdentityFromCtx(ctx)
	sessionID := session.SessionIDFromCtx(ctx)

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

	if isWebSocketRequest(r){
		h.proxyWebSocket(w, r)
	}

	h.proxyHTTP(w, r)










	// Write request envelope + body to stream
	envelope := gen.RequestHeader{
		Method:     r.Method,
		AppId:      AppID,
		Path:       r.URL.Path,
		Query:      r.URL.RawQuery,
		UserId:     userID,    // Defaults to "" if Identity is nil
		UserEmail:  userEmail, // Defaults to "" if Identity is nil
		Headers:    flattenHeaders(r.Header),
		RequestId:  uuid.NewString(),
		SessionId:  userSessionID, // Defaults to "no-session" if sessionID is empty
		BodyLength: r.ContentLength,
	}
	if err := writeEvelope(stream, &envelope); err != nil {
		h.log.Error("Failed to write request envelope", zap.Error(err))
		http.Error(w, "failed to write request envelope", http.StatusInternalServerError)
		return
	}

	if r.Body != nil {
		if _, err := io.Copy(stream, r.Body); err != nil {
			h.log.Error("Failed to write request body", zap.Error(err))
			http.Error(w, "failed to write request body", http.StatusInternalServerError)
			return
		}
	}

	//Read response envelope + body from stream
	var respEnvelope gen.ResponseHeader
	if err := readEnvelope(stream, &respEnvelope); err != nil {
		h.log.Error("Failed to read response envelope", zap.Error(err))
		http.Error(w, "failed to read response envelope", http.StatusInternalServerError)
		return
	}
	// Validate status code from connector
	status := int(respEnvelope.StatusCode)
	if status < 100 || status > 599 {
		h.log.Warn("invalid status code from connector, mapping to 502", zap.Int32("status", respEnvelope.StatusCode))
		status = http.StatusBadGateway
	}

	for key, value := range respEnvelope.Headers {
		w.Header().Set(key, value)
	}

	// Write status header
	w.WriteHeader(status)

	// Determine whether the client expects/accepts a response body.
	allowBody := true
	if r.Method == http.MethodHead {
		allowBody = false
	}
	if status >= 100 && status < 200 {
		allowBody = false
	}
	if status == http.StatusNoContent || status == http.StatusNotModified {
		allowBody = false
	}

	// If body is allowed, stream it to the client; otherwise drain the stream
	// so the connector can finish writing/close the stream without blocking.
	if allowBody {
		if _, err := io.Copy(w, stream); err != nil {
			h.log.Error("Failed to write response body", zap.Error(err))
		}
	} else {
		// Drain at most 1MB of the discarded body with a 2-second timeout to prevent hangs
		drainDone := make(chan struct{})
		go func() {
			_, _ = io.Copy(io.Discard, io.LimitReader(stream, 1024*1024))
			close(drainDone)
		}()
		select {
		case <-drainDone:
		case <-time.After(2 * time.Second):
			h.log.Warn("Draining response body timed out, closing stream")
		}
	}

	// w.WriteHeader(http.StatusNotImplemented)
	// w.Write([]byte("Ashrix Gateway: Proxy logic pending connector integration"))
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	clientIP := r.RemoteAddr
	if !h.rateLimiter.Allow(r.Context(), clientIP) {
		http.Error(w, "Too Many Request", http.StatusTooManyRequests)
		return
	}

	//Read the user cookie
	stateCookie, err := r.Cookie("state")
	if err != nil || stateCookie.Value == "" {
		http.Error(w, "Missing Parameter Cookie", http.StatusBadRequest)
		return
	}

	//Verify and consule state
	stateKey := fmt.Sprintf("oauth_state:%s", stateCookie.Value)
	stateData, err := h.redisClient.GetDel(r.Context(), stateKey).Result()
	if err != nil {
		http.Error(w, "Missing Parameter Redis", http.StatusBadRequest)
		return
	}

	var statePayload map[string]string
	json.Unmarshal([]byte(stateData), &statePayload)
	redirectURI := statePayload["redirect_uri"]
	// redirectURI := "http://app.ashrix.io:8000"

	//Exchange token with CP via mTLS grpc
	tokenStateFromCP := r.URL.Query().Get("state")

	tokenHash := crypto.HashToken(tokenStateFromCP)

	protoIdentity, err := h.grpcClient.ExchangeToken(r.Context(), &gen.ExchangeTokenRequest{
		TokenHash:   tokenHash,
		GatewayName: h.cfg.GatewayName,
	})

	if err != nil {
		http.Error(w, "Invalid Token", http.StatusBadRequest)
		return
	}

	if err := h.session.Create(w, r, protoIdentity); err != nil {
		h.log.Error("Session Creation Failed", zap.String("err", err.Error()))
		http.Error(w, "Internal Error", http.StatusNotFound)
		return
	}

	h.log.Info("Session Creation")
	http.Redirect(w, r, redirectURI, http.StatusTemporaryRedirect)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.session.Destroy(w, r)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "ok"}`))
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "ok"}`))
}
