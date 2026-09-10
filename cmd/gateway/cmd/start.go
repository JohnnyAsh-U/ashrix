package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	// "time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/bootstrap"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	gateway_grpc "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/grpc_client"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/router"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/grpc"
	http_proxy "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http"
	quic_server "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/quic"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"

	// proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Ashrix Gateway",
	Long: `Start the Ashrix Gateway. The gateway must be registered first.

Loads identity (private key + certificate) from the data directory.
Connects to the Control Plane and begins policy sync.

If the gateway is not registered, run:
  ashrix-gateway register --cp-url=<url> (with ASHRIX_TOKEN set)`,
	RunE: runStart,
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

	//--------------------------REDIS------------------------------------------------//
	redisStore, err := config.NewRedisStore(context.Background(), cfg)

	if err != nil {
		log.Error("Failed to connect to Redis", slog.String("err", err.Error()))
	}

	log.Info("Redis connected")
	defer redisStore.Close()

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

	client := bootstrap.NewClient(cfg.CPURL)

	pki, err := crypto.NewGatewayPKI(
		cfg.GatewayID,
		cfg.DataDir,
		"SECRET",
		"GATEWAY",
		logging.App,
		client,
		cfg,
	)

	if err != nil {
		log.Error("failed to load gateway identity — is the gateway registered?",
			slog.String("hint",
				fmt.Sprintf("run: ashrix-gateway register --cp-url=%s", cfg.CPURL)),
			slog.Any("err", err),
		)
		os.Exit(1)
	}

	// -----------------Preflight cert check -------------------------
	if err := pki.LoadAndVerify(); err != nil {
		if errors.Is(err, crypto.ErrCertExpired) {
			if renewErr := pki.PreflightRenew(context.Background()); renewErr != nil {
				log.Error("Certificate Expired and renewal failed",
					slog.String("hint", fmt.Sprintf(
						"run: ashrix-gateway register --cp-url=%s", cfg.CPURL)),
					slog.Any("err", renewErr))
				os.Exit(1)
			}
		} else {
			log.Error("Certifcate Invalid")
			os.Exit(1)
		}
	}

	//---------------------------Initialize and Load Registry------------------------------//

	reg := registry.New(log)

	log.Info("Initialising Registry...")

	//--------------------------Open Policy Store -----------------------------------------------//

	log.Info("Initialising Policy Store And Engine...")
	policyStore, err := store.OpenBoltStore(cfg.DataDir)
	if err != nil {
		log.Error("failed to open policy store", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Load CP public key (embedded in binary)
	verifier, err := store.RootPublicKey()
	if err != nil {
		log.Error("failed to open policy store", slog.String("error", err.Error()))
		os.Exit(1)
	}

	//Policy Engine takes Store as args to load the policies into the engine
	engine, err := policy.NewEngine(context.Background(), policyStore, verifier, cfg.GatewayID, log)
	if err != nil {
		log.Error("failed to initialize policy engine", slog.Any("err", err))
	}

	defer policyStore.Close()
	log.Info("Initialising Policy And Engine Done...")

	rtr := router.NewRouter(reg, engine, policyStore)

	//-----------------------Load TLS and Open stream -----------------------------------------------
	// This is the real liveness check - If CP is unreachable or
	// rejects the cert, we check if connection is successful
	// if cert expired we renew cert here

	log.Info("connecting to Control Plane", slog.String("cp_url", cfg.CPURL))

	cpHost := strings.TrimPrefix(cfg.CPURL, "http://")
	cpHost = strings.TrimPrefix(cpHost, "https://")
	cpHost = strings.TrimSuffix(cpHost, "/")
	cpHost = strings.NewReplacer(":8001", ":9443").Replace(cpHost)

	// Context that lives until shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//-------------------Session Initializing------------------------
	activeStreams := registry.NewActiveStreamRegistry()

	sessions, err := session.NewSessionManager(redisStore.Client(), cfg.SessionTTL, activeStreams, cfg.CookieSecure, cfg.GatewayID, cfg.TenantId)
	if err != nil {
		log.Error("Session initialization failed", slog.Any("err", err))
		os.Exit(1)
	}
	log.Info("Session Initialized")

	// 1. Connection Manager
	cm := gateway_grpc.NewConnectionManager(cpHost, pki.GetTLSConfig())

	//2. Access Logger
	accessLogger := logging.NewAccessLogger(cfg, nil)
	defer accessLogger.Close()

	// ── 3. Stream Manager (bidi stream + hello handshake) ─────────
	// var epochCounter atomic.Uint64
	sm := gateway_grpc.NewStreamManager(
		cm,
		policyStore,
		cfg,
		engine,
		log,
		128,
		pki,
		reg,
		redisStore.Client(),
		sessions,
		accessLogger,
	)

	// StreamManager implements LogSender.
	accessLogger.SetSender(sm)

	// 4. Safe Unary Client (for HTTP handlers that need CP)
	grpcClient := gateway_grpc.NewSafeClient(cm)

	// 5. Establish initial connection
	if err := cm.RefreshConnection(ctx); err != nil {
		log.Error("initial connection failed", slog.Any("err", err))
	}

	// 6. Background loops
	go sm.Run(ctx)
	// go cm.HealthCheckLoop(ctx, 10*time.Second)

	// 6. Start cert rotator after CP is confirmed alive
	pki.StartRotator()
	defer pki.Stop()

	//--------------------Start Listeners--------------------
	mtlsConfig := pki.GetTLSConfig()

	log.Info("initializing Gateway servers")

	grpcServer := grpc.NewGRPCServer(log, reg, mtlsConfig, cfg.GRPCPort == cfg.HTTPSPort)
	httpServer := http_proxy.NewProxyServer(
		cfg,
		grpcClient,
		reg,
		redisStore.Client(),
		sessions,
		log,
		activeStreams,
		engine,
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

	sig := <-quit
	log.Info("shutdown signal received", slog.String("signal", sig.String()))

	server.ShutdownServers(httpServer, grpcServer, quicServer, log)

	// Cancel context → StreamManager.Run exits.
	cancel()

	// Close the CP connection explicitly.
	if err := cm.Close(); err != nil {
		log.Error(
			"failed to close CP connection",
			slog.Any("err", err),
		)
	}

	return nil
}
