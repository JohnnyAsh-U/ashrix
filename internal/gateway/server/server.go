package server

import (
	"log/slog"
	"net/http"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"

)

// ProxyServer handles incoming user traffic and routes it to connectors.
type ProxyServer struct {
	http *http.Server
	log  *slog.Logger
}

func NewProxyServer(cfg *config.Config, log *slog.Logger) *ProxyServer {
	mux := http.NewServeMux()

	// Central proxy handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract hostname
		// 2. Identify Target Org/App
		// 3. Verify Session Cookie
		// 4. Check Policy
		// 5. Proxy to Connector gRPC stream
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("Ashrix Gateway: Proxy logic pending connector integration"))
	})

	srv := &http.Server{
		// Addr:    cfg.HTTPAddr,
		Handler: mux,
		// TLSConfig will be set by the main loop using GatewayPKI
	}

	return &ProxyServer{
		http: srv,
		log:  log,
	}
}

func (s *ProxyServer) Start() error {
	s.log.Info("Gateway Proxy Server starting", "addr", s.http.Addr)
	// In production, this uses s.http.ListenAndServeTLS("", "")
	// with the Gateway's certificate identity.
	return s.http.ListenAndServe()
}
