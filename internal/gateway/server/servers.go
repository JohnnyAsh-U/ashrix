package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/grpc"
	grpc_server "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/grpc"
	http_proxy "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http"
	quic_server "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/quic"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/tlsconfig"
)

func StartServers(
	cfg *config.Config,
	tlsConfig *tls.Config,
	ctx context.Context,
	log *slog.Logger,
	grpcServer *grpc_server.GRPCServer,
	httpServer *http_proxy.ProxyServer,
	quicServer *quic_server.QUICServer,
) error {

	// ---------------------------------------------------------
	// 1. Ashrix mTLS configuration
	// ---------------------------------------------------------

	mtlsConfig := tlsConfig.Clone()

	// ---------------------------------------------------------
	// 2. Public HTTPS certificate
	// ---------------------------------------------------------

	var publicTLS *tls.Config
	httpsEnabled := false

	publicCert, err := tls.LoadX509KeyPair("server.crt", "server.key")

	if err != nil {
		log.Warn("Unable to load public HTTPS certificate; HTTPS disabled", "error", err)
		if cfg.IsProdEnv {
			return fmt.Errorf("Configure the TLS Cert.")
		}
	} else {
		publicTLS = &tls.Config{
			MinVersion: tls.VersionTLS13,
			Certificates: []tls.Certificate{
				publicCert,
			},
			NextProtos: []string{
				"h2",
				"http/1.1",
			},
		}
		httpsEnabled = true
	}

	// ---------------------------------------------------------
	// 3. Build Gateway TLS configurations
	// ---------------------------------------------------------

	gatewayTLS := tlsconfig.NewGatewayTLSConfig(mtlsConfig, publicTLS, httpsEnabled)

	// ---------------------------------------------------------
	// 4. Start HTTP / HTTPS / gRPC
	// ---------------------------------------------------------

	if httpsEnabled && cfg.IsProdEnv {

		// =====================================================
		// Production path
		//
		// TCP :443
		//
		// HTTPS + gRPC mTLS
		// =====================================================
		// When Prod and Cert is Available the TLS Is Handled HERE

		rawListener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.HTTPSPort))
		if err != nil {
			return fmt.Errorf("listen HTTPS %s: %w", cfg.HTTPSPort, err)
		}

		tlsListener := tls.NewListener(rawListener, gatewayTLS.Shared)

		go func() {
			log.Info("Gateway HTTPS/gRPC listener started", "addr", cfg.HTTPSPort)
			if err := httpServer.Start(tlsListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("HTTPS server stopped", "error", err)
			}
		}()

	} else {

		// =====================================================
		// HTTPS disabled
		//
		// TCP :80  → plaintext HTTP
		// TCP :443 → gRPC mTLS
		// =====================================================

		httpListener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.HTTPPort))
		if err != nil {
			return fmt.Errorf("listen HTTP %s: %w", cfg.HTTPPort, err)
		}

		go func() {
			log.Info("Gateway running without HTTPS", "addr", cfg.HTTPPort)
			if err := httpServer.Start(httpListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("HTTP server stopped", "error", err)
			}
		}()

		// -----------------------------------------------------
		// gRPC always remains mTLS.
		// -----------------------------------------------------
		// When Dev mode Different Port is used for HTTP AND GRPC so the tls for
		//grpc is handled in the grpc server

		grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
		if err != nil {
			return fmt.Errorf("listen gRPC %s: %w", cfg.GRPCPort, err)
		}

		// grpcTLSListener := tls.NewListener(grpcListener, gatewayTLS.GRPC)

		go func() {
			log.Info("Gateway gRPC mTLS listener started", "addr", cfg.GRPCPort)
			if err := grpcServer.Serve(grpcListener); err != nil {
				log.Error("gRPC server stopped", "error", err)
			}
		}()
	}

	// ---------------------------------------------------------
	// 5. QUIC
	// ---------------------------------------------------------

	go func() {
		log.Info("Gateway QUIC listener started", "addr", cfg.QUICPort)
		if err := quicServer.Start(ctx); err != nil {
			log.Error("QUIC server stopped", "error", err)
		}
	}()

	return nil
}

func ShutdownServers(
	httpServer *http_proxy.ProxyServer,
	grpcServer *grpc.GRPCServer,
	quicServer *quic_server.QUICServer,
	log *slog.Logger,
) {

	// Stop accepting new QUIC connections first.
	quicServer.Stop()
	grpcServer.Stop()

	// Stop listeners gracefully.
	if err := httpServer.Stop(); err != nil {
		log.Error(
			"failed to stop HTTP/gRPC server",
			slog.Any("err", err),
		)
	}
}
