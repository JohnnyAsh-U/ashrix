package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "err", err)
		os.Exit(1)
	}

	log := logger.New("development") // In production use cfg.ENV

	// 1. Initialize PKI
	pki, err := crypto.NewGatewayPKI(cfg.NodeID, cfg.BasePath, cfg.UnlockSecret)
	if err != nil {
		log.Error("PKI initialization failed", "err", err)
		os.Exit(1)
	}

	// 2. Bootstrap if needed
	if !pki.IsBootstrapped() {
		log.Info("Node not bootstrapped. Starting enrollment...")
		if cfg.BootstrapToken == "" {
			log.Error("Bootstrap token required for first-time setup")
			os.Exit(1)
		}

		// Create bootstrap client, call CP, save identity
		// (Logic described in Phase 2 of roadmap)
	}

	// 3. Start Proxy Server
	proxy := server.NewProxyServer(cfg, log)
	go func() {
		if err := proxy.Start(); err != nil {
			log.Error("Proxy server failed", "err", err)
		}
	}()

	// 4. Wait for shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Gateway shutting down...")
}
