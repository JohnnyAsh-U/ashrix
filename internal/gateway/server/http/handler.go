package http_proxy

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/router"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http/web"
)

type Handler struct {
	log          *slog.Logger
	router       *router.Router
	errorHandler *web.ErrorHandler
}

func NewHandler(
	log *slog.Logger,
	rtr *router.Router,
	errHandler *web.ErrorHandler,
) *Handler {

	return &Handler{
		log:          log,
		router:       rtr,
		errorHandler: errHandler,
	}
}

func (h *Handler) ProxyHandler(w http.ResponseWriter, r *http.Request) {

	if isWebSocketRequest(r) {
		h.proxyWebSocket(w, r)
		return
	}

	h.proxyHTTP(w, r)
}

func isWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "ok"}`))
}
