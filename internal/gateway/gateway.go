package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/proxy"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/tunnel"
	"github.com/JohnnyAsh-U/ashrix-api/internal/pki"
)

func RunGateway(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	log.Info("Starting Ashrix gateway")

	// PKI Initializing
	ca, err := pki.LoadCA(cfg.PKI.CACertificate, cfg.PKI.CAPrivateKey)

	if err != nil {
		log.Error("failed to load pki", slog.Any("err", err))
		return fmt.Errorf("load gateway CA: %w", err)
	}

	//Gateway CA
	gatewayPKI, err := crypto.NewLocalGatewayPKI(cfg.Gateway.Certificate, cfg.Gateway.PrivateKey, ca.Certificate, cfg.Gateway.ServerName)
	if err != nil {
		log.Error("failed to load gateway cert", slog.Any("err", err))
		return fmt.Errorf("load gateway certificate: %w", err)
	}

	// 	//2. Access Logger
	accessLogger := logging.NewAccessLogger(cfg)
	defer accessLogger.Close()

	// Tunnel
	quicTunnel, err := tunnel.NewQuicTunnel(
		cfg,
		gatewayPKI.GetTLSConfig().Clone(),
		log,
	)
	proxyHandler := proxy.NewProxyHandler(cfg, log, quicTunnel, accessLogger)

	// //Proxy server - HTTP and HTTPS
	proxyServers := proxy.NewProxyServers(cfg,log,proxyHandler)


	if err := proxyServers.StartProxyServers(); err != nil {
		log.Warn("Error starting http/https proxy servers", slog.Any("err", err))
	}

	if err := quicTunnel.Start(ctx); err != nil {
		log.Warn("Error starting quic server", slog.Any("err", err))
	}

	// 	// Block until SIGTERM or SIGINT (Ctrl+C / systemd stop / our stop command).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	sig := <-quit
	log.Info("shutdown signal received", slog.String("signal", sig.String()))

	// Stop QUIC connections.
	quicTunnel.Stop()

	// Stop listeners gracefully.
	if err := proxyServers.ShutdownProxyServers(ctx); err != nil {
		log.Error("failed to stop HTTP/HTTPS server",slog.Any("err", err))
	}

	return nil

}
