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
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/router"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server"
	http_proxy "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/quic"
	"github.com/JohnnyAsh-U/ashrix-api/internal/pki"
)

func RunGateway(ctx context.Context, config *config.Config, log *slog.Logger) error {
	log.Info("Starting Gateway...")

	// PKI Initializing
	PKICA, err := pki.LoadCA(config.PKI.CACertificate, config.PKI.CAPrivateKey)

	if err != nil {
		log.Error("failed to load pki", slog.Any("err", err))
		return err
	}

	//Gateway CA
	pki, err := crypto.NewLocalGatewayPKI(config.Gateway.Certificate, config.Gateway.PrivateKey, PKICA.Certificate)

	if err != nil {
		log.Error("failed to load gateway cert", slog.Any("err", err))
		return err
	}

	// 	//---------------------------Initialize and Load Registry------------------------------//

	reg := registry.New(log)
	authorizedConnectors := make(map[string]string, len(config.Connectors))
	for _, connector := range config.Connectors {
		authorizedConnectors[connector.ID] = "active"
	}
	reg.SetAuthorizedConnectors(authorizedConnectors)

	log.Info("Initialising Registry...")

	rtr := router.NewRouter(reg)

	// 	//2. Access Logger
	accessLogger := logging.NewAccessLogger(config)
	defer accessLogger.Close()

	//--------------------Start Listeners--------------------
	mtlsConfig := pki.GetTLSConfig()

	log.Info("initializing Gateway servers")

	httpServer := http_proxy.NewProxyServer(
		config,
		reg,
		log,
		rtr,
		accessLogger,
	)

	quicServer := quic.NewQUICServer(
		config,
		mtlsConfig,
		log,
		reg,
		rtr,
	)

	if err := server.StartServers(config, mtlsConfig, ctx, log, httpServer, quicServer); err != nil {
		log.Error(err.Error())
		return fmt.Errorf("Erro: %w", err)
	}


	// 	// Block until SIGTERM or SIGINT (Ctrl+C / systemd stop / our stop command).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)


	sig := <-quit
	log.Info("shutdown signal received", slog.String("signal", sig.String()))
	
	server.ShutdownServers(httpServer, quicServer, log)

	return nil

}
