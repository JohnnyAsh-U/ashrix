package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	// "net/http/pprof"

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

	publicCert, err := tls.LoadX509KeyPair("server.crt", "server.key")

	if err != nil {
		log.Warn("Unable to load public HTTPS certificate; HTTPS disabled", "error", err)
		return fmt.Errorf("Configure the TLS Cert.")
	}

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

	// ---------------------------------------------------------
	// 4. Start HTTP / HTTPS / gRPC
	// ---------------------------------------------------------

	samePorts := false
	if cfg.GRPCPort == cfg.HTTPSPort {
		samePorts = true
	}

	if samePorts {

		// =====================================================
		// Production path
		//
		// If Same Port
		//
		// HTTPS + gRPC mTLS
		// =====================================================
		// When Same Port for http and grpc the TLS Is Handled HERE

		rawListener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.HTTPSPort))
		if err != nil {
			return fmt.Errorf("listen HTTPS %s: %w", cfg.HTTPSPort, err)
		}

		gatewayTLS := tlsconfig.NewGatewayTLSConfig(mtlsConfig, publicTLS)

		tlsListener := tls.NewListener(rawListener, gatewayTLS.Shared)

		go func() {
			log.Info("Gateway HTTPS/gRPC listener started", "addr", cfg.HTTPSPort)
			if err := httpServer.Start(tlsListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("HTTPS server stopped", "error", err)
			}
		}()

	} else {

		// =====================================================
		//
		//
		// When the port of grpc and http are different
		// The HTTP tls is handled here
		// =====================================================

		httpListener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.HTTPSPort))
		if err != nil {
			return fmt.Errorf("listen HTTP %s: %w", cfg.HTTPSPort, err)
		}

		gatewayTLS := tlsconfig.NewGatewayTLSConfig(mtlsConfig, publicTLS)
		tlsListener := tls.NewListener(httpListener, gatewayTLS.Shared)

		go func() {
			log.Info("Gateway running with HTTP", "addr", cfg.HTTPSPort)
			if err := httpServer.Start(tlsListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("HTTP server stopped", "error", err)
			}
		}()

		//grpc tls is handled in the grpc server

		grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
		if err != nil {
			return fmt.Errorf("listen gRPC %s: %w", cfg.GRPCPort, err)
		}

		go func() {
			log.Info("Gateway gRPC mTLS listener started", "addr", cfg.GRPCPort)
			if err := grpcServer.Serve(grpcListener); err != nil {
				log.Error("gRPC server stopped", "error", err)
			}
		}()
	}

	// ---------------------------------------------------------
	// 5. Local pprof / diagnostics
	// ---------------------------------------------------------

	// go func() {
	// 	mux := http.NewServeMux()

	// 	mux.HandleFunc("/debug/pprof/", pprof.Index)
	// 	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	// 	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	// 	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	// 	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// 	log.Info("Gateway pprof listener started", "addr", "127.0.0.1:6060")

	// 	if err := http.ListenAndServe("127.0.0.1:6060", mux); err != nil {
	// 		log.Error("pprof server stopped", "error", err)
	// 	}
	// }()

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
