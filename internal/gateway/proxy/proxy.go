package proxy

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/tunnel"
	"github.com/JohnnyAsh-U/ashrix-api/internal/telemetry"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// ProxyServer handles incoming user traffic and routes it to connectors.
type ProxyServer struct {
	http *http.Server
	log  *slog.Logger
}

func NewProxyServer(
	cfg *config.Config,
	log *slog.Logger,
	registry *tunnel.Registry,
	accessLogger *logging.AccessLogger,
) *ProxyServer {

	// handler := NewHandler(log)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.ClientIPFromHeader("X-Real-IP"))
	r.Use(logging.AccessLogMiddleware(accessLogger, cfg.Gateway.ID, log))
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(AppAccessMiddleware(registry, cfg, log))

	r.Get("/_ashrix/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok"}`))
	})

	r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		ProxyRequest(w, r, registry, log)
	})

	return &ProxyServer{
		http: &http.Server{
			Addr:              "",
			Handler:           telemetry.HTTPMiddleware(cfg.Gateway.ID)(r),
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
		log: log,
	}
}

func (h *ProxyServer) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "ok"}`))
}


// Start serves HTTP and gRPC on the supplied TLS listener.
func (s *ProxyServer) Start(listener net.Listener) error {
	s.log.Info("Gateway HTTP + gRPC server starting", slog.String("addr", listener.Addr().String()))
	return s.http.Serve(listener)
}

func (s *ProxyServer) Shutdown(ctx context.Context) error {
	s.log.Info("Gateway HTTP + gRPC server shutting down")
	return s.http.Shutdown(ctx)
}

func (s *ProxyServer) Stop() error {
	s.log.Info("Gateway HTTP + gRPC server stopping")
	return s.http.Close()
}

func isWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}
