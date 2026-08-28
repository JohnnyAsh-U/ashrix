package http_proxy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/JohnnyAsh-U/ashrix-api/pkg/crypto"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/redis/go-redis/v9"

	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	gateway_grpc "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/grpc_client"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/router"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http/web"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
)

type Handler struct {
	log            *slog.Logger
	registry       *registry.Registry
	session        *session.SessionManager
	rateLimiter    *session.RedisLimiter
	redisClient    *redis.Client
	grpcClient     *gateway_grpc.SafeClient
	cfg            *config.Config
	streamRegistry *registry.ActiveStreamRegistry
	router         *router.Router
	maxRequestBody int64
	errorHandler *web.ErrorHandler
}

func NewHandler(
	log *slog.Logger,
	registry *registry.Registry,
	session *session.SessionManager,
	limiter *session.RedisLimiter,
	redisClient *redis.Client,
	grpcClient *gateway_grpc.SafeClient,
	streamRegistry *registry.ActiveStreamRegistry,
	cfg *config.Config,
	rtr *router.Router,
	errHandler *web.ErrorHandler,
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
		router:         rtr,
		maxRequestBody: 500,
		errorHandler: errHandler,
	}
}

func (h *Handler) ProxyHandler(w http.ResponseWriter, r *http.Request) {

	if isWebSocketRequest(r){
		h.proxyWebSocket(w, r)
		return
	}

	h.proxyHTTP(w, r)
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	clientIP := r.RemoteAddr
	if !h.rateLimiter.Allow(r.Context(), clientIP) {
		h.errorHandler.ErrorPage(w, http.StatusTooManyRequests, "Too Many Requests", "The server is currently experiencing a high volume of traffic. Please try again later.", "")
		return
	}

	//Read the user cookie
	stateCookie, err := r.Cookie("state")
	if err != nil || stateCookie.Value == "" {
		h.errorHandler.ErrorPage(w, http.StatusBadRequest, "Missing Parameter Cookie", "Please provide a valid state cookie.", "")
		return
	}

	//Verify and consule state
	stateKey := fmt.Sprintf("oauth_state:%s", stateCookie.Value)
	stateData, err := h.redisClient.GetDel(r.Context(), stateKey).Result()
	if err != nil {
		h.errorHandler.ErrorPage(w, http.StatusBadRequest, "Missing Parameter Redis", "The provided state parameter is invalid or has expired.", "")
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
		h.errorHandler.ErrorPage(w, http.StatusBadRequest, "Invalid Token", "The provided token is invalid or has expired.", "")
		return
	}

	if err := h.session.Create(w, r, protoIdentity); err != nil {
		h.log.Error("Session Creation Failed", slog.Any("err", err))
		h.errorHandler.ErrorPage(w, http.StatusInternalServerError, "Internal Error", "An internal error occurred while creating the session.", "")
		return
	}

	h.log.Info("Session Creation")
	http.Redirect(w, r, redirectURI, http.StatusTemporaryRedirect)
}


func isWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
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
