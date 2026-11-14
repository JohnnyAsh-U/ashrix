package proxy

import (
	"context"
	"crypto/tls"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
)

// ProxyServer handles incoming user traffic and routes it to connectors.
type ProxyServer struct {
	http  *http.Server
	https *http.Server
	cfg *config.Config
	log   *slog.Logger
}

func NewProxyServers(
	cfg *config.Config,
	log *slog.Logger,
	proxyHandler *ProxyHandler,
) *ProxyServer {

	// HTTP server - no TLS, same router
	httpServer := &http.Server{
		Addr:              cfg.Listeners.HTTP.Address,
		Handler:           proxyHandler.r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// HTTPS server TLS,
	httpsServer := &http.Server{
		Addr:              cfg.Listeners.HTTPS.Address,
		Handler:           proxyHandler.r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		IdleTimeout:       120 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	return &ProxyServer{
		http:  httpServer,
		https: httpsServer,
		cfg: cfg,
		log:   log,
	}
}

// Start serves HTTP
func (s *ProxyServer) StartProxyServers() error {
	s.log.Info("Gateway HTTP server starting", slog.String("addr", s.http.Addr))

	go func() {
		if err := s.http.ListenAndServe(); err != nil {
			log.Fatal("HTTP: %w", err)
			return
		}
	}()

	go func() {
		if err := s.https.ListenAndServeTLS(
			s.cfg.Listeners.HTTPS.Certificate,
			s.cfg.Listeners.HTTPS.PrivateKey,
		); err != nil {
			log.Fatal("HTTPS: %w", err)
			return
		}
	}()
	return nil
}

func (s *ProxyServer) ShutdownProxyServers(ctx context.Context) error {
	s.log.Info("Gateway HTTP  server shutting down")
	if err := s.http.Shutdown(ctx); err != nil {
		return err
	}
	if err := s.https.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}

func (s *ProxyServer) Stop() error {
	s.log.Info("Gateway HTTP + gRPC server stopping")
	return s.http.Close()
}


