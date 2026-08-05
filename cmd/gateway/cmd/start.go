package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/bootstrap"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	gateway_grpc "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/grpc_client"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/grpc"
	http_proxy "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/http"
	quic_server "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server/quic"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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

	defer logging.App.Sync()

	log := logging.App

	//--------------------------REDIS------------------------------------------------//
	redisStore, err := config.NewRedisStore(context.Background(), cfg)

	if err != nil {
		log.Error("Failed to connect to Redis", zap.String("err", err.Error()))
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
		log.Fatal("failed to load gateway identity — is the gateway registered?",
			zap.String("hint",
				fmt.Sprintf("run: ashrix-gateway register --cp-url=%s", cfg.CPURL)),
			zap.Error(err),
		)
	}

	// -----------------Preflight cert check -------------------------
	if err := pki.LoadAndVerify(); err != nil {
		if errors.Is(err, crypto.ErrCertExpired) {
			if renewErr := pki.PreflightRenew(context.Background()); renewErr != nil {
				log.Fatal("Certificate Expired and renewal failed",
					zap.String("hint", fmt.Sprintf(
						"run: ashrix-gateway register --cp-url=%s", cfg.CPURL)),
					zap.Error(renewErr))
			}
		} else {
			log.Fatal("Certifcate Invalid")
		}
	}

	//---------------------------Initialize and Load Registry------------------------------//

	reg := registry.New()

	log.Info("Initialising Registry...")

	//--------------------------Open Policy Store -----------------------------------------------//

	log.Info("Initialising Policy Store And Engine...")
	policyStore, err := store.OpenBoltStore(cfg.DataDir)
	if err != nil {
		log.Error("failed to open policy store", zap.String("error", err.Error()))
		os.Exit(1)
	}

	// Load CP public key (embedded in binary)
	verifier, err := store.RootPublicKey()
	if err != nil {
		log.Error("failed to open policy store", zap.String("error", err.Error()))
		os.Exit(1)
	}

	
	//Policy Engine takes Store as args to load the policies into the engine
	engine := policy.NewEngine(context.Background(), policyStore, verifier, log)

	defer policyStore.Close()
	log.Info("Initialising Policy And Engine Done...")

	//-----------------------Load TLS and Open stream -----------------------------------------------
	// This is the real liveness check - If CP is unreachable or
	// rejects the cert, we check if connection is successful
	// if cert expired we renew cert here

	log.Info("connecting to Control Plane", zap.String("cp_url", cfg.CPURL))

	// FIX: typo was "https//" (missing colon). Also handle scheme stripping properly.
	cpHost := strings.TrimPrefix(cfg.CPURL, "http://")
	cpHost = strings.TrimPrefix(cpHost, "https://")
	cpHost = strings.TrimSuffix(cpHost, "/")
	cpHost = strings.NewReplacer(":8001", ":9443").Replace(cpHost)

	// Context that lives until shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Connection Manager
	cm := gateway_grpc.NewConnectionManager(cpHost, pki.GetTLSConfig())

	// ── 2. Stream Manager (bidi stream + hello handshake) ─────────
	// var epochCounter atomic.Uint64
	sm := gateway_grpc.NewStreamManager(
		cm,
		func() *proto.GatewayEnvelope {
			return &proto.GatewayEnvelope{
				GatewayId: "3",
				Payload: &proto.GatewayEnvelope_Hello{
					Hello: &proto.HelloMessage{
						GatewayId:     cfg.GatewayID,
						BinaryVersion: "0",
						PolicyVersion: 0,
						CrlVersion:    0,
						TrustVersion:  0,
					},
				},
			}
		},
		log,
		128,
		func(ctx context.Context) error {
			return pki.PreflightRenew(ctx)
		},
	)

	// 3. Safe Unary Client (for HTTP handlers that need CP)
	grpcClient := gateway_grpc.NewSafeClient(cm)

	// 4. Establish initial connection
	if err := cm.RefreshConnection(ctx); err != nil {
		log.Fatal("initial connection failed", zap.Error(err))
	}

	// 5. Background loops
	go sm.Run(ctx)
	go cm.HealthCheckLoop(ctx, 10*time.Second)

	// 6. Start cert rotator after CP is confirmed alive
	pki.StartRotator()
	defer pki.Stop()

	//--------------------Start Listeners--------------------

	// Instantiate new servers
	grpcServer := grpc.NewGRPCServer(cfg, pki.GetTLSConfig(), log, reg)
	quicServer := quic_server.NewQUICServer(cfg, pki.GetTLSConfig(), log, reg)
	httpServer := http_proxy.NewProxyServer(cfg, grpcClient, reg, redisStore.Client(), log, engine)

	// Start servers in background
	go func() {
		if err := httpServer.Start(); err != nil {
			log.Error("HTTP server stopped", zap.Error(err))
		}
	}()

	go func() {
		if err := grpcServer.Start(); err != nil {
			log.Error("gRPC server stopped", zap.Error(err))
		}
	}()

	go func() {
		if err := quicServer.Start(ctx); err != nil {
			log.Error("QUIC server stopped", zap.Error(err))
		}
	}()

	// Block until SIGTERM or SIGINT (Ctrl+C / systemd stop / our stop command).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	sig := <-quit
	log.Info("shutdown signal received", zap.String("signal", sig.String()))

	// Stop listeners gracefully
	grpcServer.Stop()
	quicServer.Stop()
	// If your HTTP server has a Stop/Shutdown method, call it here:
	// httpServer.Stop()

	// Cancel context → StreamManager.Run and HealthCheckLoop exit
	cancel()

	// Close the gRPC connection explicitly
	if err := cm.Close(); err != nil {
		log.Error("failed to close CP connection", zap.Error(err))
	}
	// }()

	return nil
}
