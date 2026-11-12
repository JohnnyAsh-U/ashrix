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
	// "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/grpc"
	http_proxy "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http"
	quic_server "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/quic"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/tlsconfig"
)

func StartServers(
	cfg *config.Config,
	tlsConfig *tls.Config,
	ctx context.Context,
	log *slog.Logger,
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


		// =====================================================
		//
		//
		// When the port of grpc and http are different
		// The HTTP tls is handled here
		// =====================================================

		httpListener, err := net.Listen("tcp", fmt.Sprintf("%s", cfg.Listeners.HTTPS.Address))
		if err != nil {
			return fmt.Errorf("listen HTTP %s: %w", cfg.Listeners.HTTPS.Address, err)
		}

		gatewayTLS := tlsconfig.NewGatewayTLSConfig(mtlsConfig, publicTLS)
		tlsListener := tls.NewListener(httpListener, gatewayTLS.Shared)

		go func() {
			log.Info("Gateway running with HTTP", "addr", cfg.Listeners.HTTPS.Address)
			if err := httpServer.Start(tlsListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("HTTP server stopped", "error", err)
			}
		}()

	
	

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
		log.Info("Gateway QUIC listener started", "addr", cfg.Listeners.QUIC.Address)
		if err := quicServer.Start(ctx); err != nil {
			log.Error("QUIC server stopped", "error", err)
		}
	}()

	return nil
}

func ShutdownServers(
	httpServer *http_proxy.ProxyServer,
	quicServer *quic_server.QUICServer,
	log *slog.Logger,
) {

	// Stop accepting new QUIC connections first.
	quicServer.Stop()

	// Stop listeners gracefully.
	if err := httpServer.Stop(); err != nil {
		log.Error(
			"failed to stop HTTP/gRPC server",
			slog.Any("err", err),
		)
	}
}
