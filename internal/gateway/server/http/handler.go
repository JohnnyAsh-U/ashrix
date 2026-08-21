package http_proxy

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/pkg/crypto"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
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
	maxRequestBody int64
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
		maxRequestBody: 500,
	}
}

func (h *Handler) ProxyHandler(w http.ResponseWriter, r *http.Request) {

	if isWebSocketRequest(r){
		h.proxyWebSocket(w, r)
	}

	h.proxyHTTP(w, r)
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
