package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
)

// ProxyServer handles incoming user traffic and routes it to connectors.
type ProxyServer struct {
	http          *http.Server
	httpListener  net.Listener
	https         *http.Server
	httpsListener net.Listener
	cfg           *config.Config
	log           *slog.Logger
}

func NewProxyServers(
	cfg *config.Config,
	log *slog.Logger,
	proxyHandler *ProxyHandler,
) (*ProxyServer, error) {

	// HTTP server - no TLS, same router

	httpListener, err := net.Listen("tcp", cfg.Listeners.HTTP.Address)

	if err != nil {
		return &ProxyServer{}, err
	}

	httpServer := &http.Server{
		Addr:              httpListener.Addr().String(),
		Handler:           proxyHandler.r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// HTTPS server TLS,
	httpsListener, err := net.Listen("tcp", cfg.Listeners.HTTPS.Address)

	if err != nil {
		return &ProxyServer{}, err
	}

	httpsServer := &http.Server{
		Addr:              httpsListener.Addr().String(),
		Handler:           proxyHandler.r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		IdleTimeout:       120 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	return &ProxyServer{
		http:          httpServer,
		httpListener:  httpListener,
		https:         httpsServer,
		httpsListener: httpsListener,
		cfg:           cfg,
		log:           log,
	}, nil
}

// Start serves HTTP
func (s *ProxyServer) StartProxyServers() error {
	s.log.Info("Gateway HTTP server starting", slog.String("addr", s.http.Addr))

	go func() {
		if err := s.http.Serve(s.httpListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP: %v", err)
			return
		}
	}()

	// HTTPS only starts if cert + key exist and are valid.
	// Missing files = HTTP-only mode, not a crash.
	if err := s.startHTTPSIfConfigured(); err != nil {
		return err
	}

	return nil
}

func (s *ProxyServer) startHTTPSIfConfigured() error {
	certFile := s.cfg.Listeners.HTTPS.Certificate
	keyFile := s.cfg.Listeners.HTTPS.PrivateKey

	// 1. Not configured at all
	if certFile == "" || keyFile == "" {
		s.log.Warn("HTTPS disabled: certificate or private_key not configured, running HTTP only")
		return nil
	}

	// 2. Files do not exist
	if _, err := os.Stat(certFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.log.Warn("HTTPS disabled: certificate file not found, running HTTP only", "file", certFile)
			return nil
		}
		return err
	}
	if _, err := os.Stat(keyFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.log.Warn("HTTPS disabled: private key file not found, running HTTP only", "file", keyFile)
			return nil
		}
		return err
	}

	// 3. Files exist but are invalid / mismatched pair
	// Check this BEFORE starting the goroutine, so you fail fast
	// instead of log.Fatal inside a goroutine.
	if _, err := tls.LoadX509KeyPair(certFile, keyFile); err != nil {
		s.log.Error("HTTPS disabled: failed to load certificate pair, running HTTP only", "cert", certFile, "key", keyFile, "err", err)
		return nil
	}
	// 4. Only now start it
	s.log.Info("Gateway HTTPS server starting", "cert", certFile)
	go func() {
		if err := s.https.ServeTLS(s.httpsListener, certFile, keyFile); err != nil && err != http.ErrServerClosed {
			s.log.Error("HTTPS server failed", "err", err)
		}
	}()

	return nil
}

func (s *ProxyServer) ShutdownProxyServers(ctx context.Context) error {
	s.log.Info("Gateway HTTP  server shutting down")
	if err := s.http.Shutdown(ctx); err != nil {
		return err
	}
	// Only shutdown HTTPS if it was actually configured
	if s.cfg.Listeners.HTTPS.Certificate != "" && s.cfg.Listeners.HTTPS.PrivateKey != "" && s.https != nil {
		if err := s.https.Shutdown(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *ProxyServer) Stop() error {
	s.log.Info("Gateway HTTP + gRPC server stopping")
	return s.http.Close()
}
