package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/bootstrap"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/router"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/grpc"
	http_proxy "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http"
	quic_server "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/quic"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/version"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/telemetry"

	// proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Ashrix Gateway",
	Long:  `Start the self-hosted Ashrix application gateway using local connector and certificate configuration.`,
	RunE:  runStart,
}

func runStart(cmd *cobra.Command, args []string) error {

	// --------Load and validate config — fails loud if required values missing------------
	cfg, err := config.LoadFromViper()
	if err != nil {
		return err
	}
	if err := config.Validate(cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return fmt.Errorf("create gateway data directory: %w", err)
	}

	//--------------Init logging-----------------------------------------

	if err := logging.Init(logging.Config{
		Env:        "dev",
		LogDir:     cfg.LogDir,
		Level:      "info",
		MaxSizeMB:  100,
		MaxAgeDays: 30,
		MaxBackups: 10,
		Compress:   true,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: cannot init logging %v\n", err)
		os.Exit(1)
	}

	log := logging.App
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if !cfg.OTelEnabled {
		log.Info("OpenTelemetry disabled")
	} else if cfg.OTelEndpoint == "" {
		log.Warn("OpenTelemetry not started: OTEL_EXPORTER_OTLP_ENDPOINT is not set")
	} else {
		serviceVersion := cfg.OTelServiceVersion
		if serviceVersion == "" {
			serviceVersion = version.GetGatewayVersion()
		}
		provider, telemetryErr := telemetry.Bootstrap(ctx, telemetry.Config{
			Endpoint:        cfg.OTelEndpoint,
			Insecure:        cfg.OTelInsecure,
			ServiceName:     cfg.OTelServiceName,
			ServiceVersion:  serviceVersion,
			MetricsInterval: cfg.OTelMetricsInterval,
			GatewayID:       cfg.GatewayID,
			Enabled:         true,
		})
		if telemetryErr != nil {
			log.Warn("OpenTelemetry initialization failed; continuing without telemetry", slog.Any("error", telemetryErr))
		} else {
			logging.AddHandler(telemetry.NewSlogHandler(provider.LoggerProvider))
			log = logging.App
			defer func() {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer shutdownCancel()
				if shutdownErr := provider.Shutdown(shutdownCtx); shutdownErr != nil {
					log.Warn("OpenTelemetry shutdown failed", slog.Any("error", shutdownErr))
				}
			}()
			log.Info("OpenTelemetry initialized", slog.String("endpoint", cfg.OTelEndpoint))
		}
	}

	//--------------------------Check PID Exist----------------------------------------
	// Refuse to start if another instance is already running.
	// Prevents two gateways running simultaneously with the same identity —
	// which is a security event for Ashrix, not just a bug.
	if bootstrap.IsPIDAlive(cfg.PIDFile) {
		return fmt.Errorf(
			"gateway already running (pid file: %s) — run 'ashrix-gateway stop' first",
			cfg.PIDFile,
		)
	}

	defer os.Remove(cfg.PIDFile)

	//-----------------------------Writing PID File-----------------------------------------

	// Write PID file immediately so stop/status work.
	// Deferred removal cleans up on clean exit.
	// Crash = stale PID file — IsPIDAlive handles that on next start.
	pid := os.Getpid()
	if err := os.WriteFile(
		cfg.PIDFile,
		[]byte(strconv.Itoa(pid)),
		0644,
	); err != nil {
		return fmt.Errorf("failed to write pid file: %w", err)
	}

	//---------------------GATEWAY PKI INITIALIZING-------------------------

	pki, err := crypto.NewLocalGatewayPKI(cfg.DataDir, "SECRET", "GATEWAY")

	if err != nil {
		log.Error("failed to load local gateway identity", slog.Any("err", err))
		os.Exit(1)
	}

	if err := pki.LoadAndVerify(); err != nil {
		log.Error("local gateway certificate is invalid", slog.Any("err", err))
		os.Exit(1)
	}

	//---------------------------Initialize and Load Registry------------------------------//

	reg := registry.New(log)
	reg.SetCrlEntries(cfg.RevokedConnectorSerials)
	authorizedConnectors := make(map[string]string, len(cfg.AuthorizedConnectors))
	for _, connectorID := range cfg.AuthorizedConnectors {
		authorizedConnectors[connectorID] = "active"
	}
	reg.SetAuthorizedConnectors(authorizedConnectors)

	log.Info("Initialising Registry...")

	rtr := router.NewRouter(reg)

	//2. Access Logger
	accessLogger := logging.NewAccessLogger(cfg)
	defer accessLogger.Close()

	//--------------------Start Listeners--------------------
	mtlsConfig := pki.GetTLSConfig()

	log.Info("initializing Gateway servers")

	grpcServer := grpc.NewGRPCServer(log, reg, mtlsConfig, cfg.GRPCPort == cfg.HTTPSPort)
	httpServer := http_proxy.NewProxyServer(
		cfg,
		reg,
		log,
		rtr,
		grpcServer,
		accessLogger,
	)

	quicServer := quic_server.NewQUICServer(
		cfg,
		mtlsConfig,
		log,
		reg,
		rtr,
	)

	if err := server.StartServers(cfg, mtlsConfig, ctx, log, grpcServer, httpServer, quicServer); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	// Block until SIGTERM or SIGINT (Ctrl+C / systemd stop / our stop command).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-quit:
		log.Info("shutdown signal received", slog.String("signal", sig.String()))
	}

	server.ShutdownServers(httpServer, grpcServer, quicServer, log)

	cancel()
	return nil
}
