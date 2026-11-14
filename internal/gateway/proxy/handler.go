package proxy

import (
	"log/slog"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/tunnel"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// ProxyServer handles incoming user traffic and routes it to connectors.
type ProxyHandler struct {
	r *chi.Mux
}

func NewProxyHandler(
	cfg *config.Config,
	log *slog.Logger,
	quicTunnel *tunnel.QuicTunnel,
	accessLogger *logging.AccessLogger,
) *ProxyHandler {

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.ClientIPFromHeader("X-Real-IP"))
	r.Use(logging.AccessLogMiddleware(accessLogger, cfg.Gateway.ID, log))
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(AppAccessMiddleware(quicTunnel, cfg, log))

	r.Get("/_ashrix/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok"}`))
	})

	r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		ProxyRequest(w, r, quicTunnel, log)
	})

	return &ProxyHandler{
		r: r,
	}
}

func (h *ProxyServer) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "ok"}`))
}

