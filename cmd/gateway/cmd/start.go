package cmd

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/bootstrap"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	gateway_grpc "github.com/JohnnyAsh-U/ashrix-api/internal/gateway/grpc"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/server"
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
	defer os.Remove(cfg.PIDFile)

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

	//-----------------------Load TLS and Open stream -----------------------------------------------
	// This is the real liveness check - If CP is unreachable or
	// rejects the cert, we check if connection is successful
	// if cert expired we renew cert here

	log.Info("connecting to Control Plane",
		zap.String("cp_url", cfg.CPURL),
	)
	tlsConfig := pki.GetTLSConfig()

	//Replace http or https with empty string
	replacer := strings.NewReplacer(":8001", ":9443", "http://", "", "https//", "")

	ctx, cancel := context.WithCancel(context.Background())

	grpcConn, stream, err := gateway_grpc.OpenStream(ctx, replacer.Replace(cfg.CPURL), tlsConfig, pki, log)
	
	if err != nil {
		if fe, ok := errors.AsType[*gateway_grpc.FatalError](err); ok {
			log.Fatal(fe.UserMessage)
		}
		log.Fatal("Failed to open Control Plane stream - is CP reachable and cert valid?",
			zap.String("cp_url", cfg.CPURL), zap.Error(err))
	}

	defer grpcConn.Close()


	// ---------------------Hello handshake---------------------
	//CP confirms this gateway is known and trusted
	helloCtx, helloCancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = gateway_grpc.SendHello(helloCtx, stream, cfg.GatewayID, log)
	helloCancel()
	if err != nil {
		log.Fatal(err.Error())
	}

	//-----------------Start Cert Rotator---------------------------------------------
	//start cert rotator after cp connection confirmed alive
	pki.StartRotator()
	defer pki.Stop()

	//---------------------Build stream handler-------------------------

	handler := gateway_grpc.NewStreamHandler(
		cfg.GatewayID,
		stream,
		pki,
		log,
	)

	log.Info("gateway started",
		zap.String("cp_url", cfg.CPURL),
		zap.String("data_dir", cfg.DataDir),
		zap.Int("pid", pid),
	)

	//--------------------Start Listeners--------------------

	// Instantiate new servers
	grpcServer := server.NewGRPCServer(cfg, pki.GetTLSConfig(), log)
	quicServer := server.NewQUICServer(cfg, pki.GetTLSConfig(), log)

	// Start servers in background
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

	go func() {
		sig := <-quit
		log.Info("shutdown signal received", zap.String("signal", sig.String()))
		
		// Stop listeners gracefully
		grpcServer.Stop()
		quicServer.Stop()
		
		cancel()
	}()

	//-------------------Reconnect Loop--------------------------------

	// handler.Run() blocks until the stream dies.
	// On disconnect, we reconnect unless shutdown was requested.
	attempt := 0

	for {
		attempt++

		streamErr := handler.Run(ctx)

		//ctx was cancelled - clean shutdown requested
		if ctx.Err() != nil {
			log.Info("gateway shutting down cleanly")
			break
		}

		// Permanent error — cannot recover, tell operator
		if fatalErr, ok := errors.AsType[*gateway_grpc.FatalError](streamErr); ok {
			log.Fatal(fatalErr.UserMessage)
		}

		// Transient error — reconnect with backoff
		delay := reconnectDelay(attempt)
		log.Warn("stream disconnected — reconnecting",
			zap.Error(streamErr),
			zap.Duration("retry_in", delay),
			zap.Int("attempt", attempt),
		)
		pki.Stop()

		select {
		case <-time.After(delay):
			// Backoff elapsed — try to reconnect
		case <-ctx.Done():
			// Shutdown signal arrived during backoff
			log.Info("gateway shutting down during reconnect")
			goto done
		}


		log.Info("redialing CP",
			zap.String("cp_url", cfg.CPURL),
			zap.Int("attempt", attempt),
		)

		grpcConn,stream, err = gateway_grpc.OpenStream(ctx, replacer.Replace(cfg.CPURL), tlsConfig, pki, log)
		if err != nil {
			if fe, ok := errors.AsType[*gateway_grpc.FatalError](err); ok {
				log.Fatal(fe.UserMessage)
			}
			log.Warn("failed to open stream on reconnect — will retry", zap.Error(err))
			continue
		}

		handler = gateway_grpc.NewStreamHandler(
			cfg.GatewayID,
			stream,
			pki,
			log,
		)

		pki.StartRotator()
		attempt = 0

		log.Info("✓ reconnected to CP",
			zap.String("gateway_id", cfg.GatewayID),
		)
	}

done:
	log.Info("shutting down")
	// // TODO: proxy.Shutdown(ctx)
	return nil
}

// reconnectDelay returns exponential backoff with a 60s cap.
// Jittered to prevent thundering herd if many gateways disconnect simultaneously.
func reconnectDelay(attempt int) time.Duration {
	base := min(
		// 2s, 4s, 8s, 16s, 32s...
		time.Duration(1<<attempt)*time.Second, 60*time.Second)
	// Add up to 20% jitter
	jitter := time.Duration(rand.Int63n(int64(base / 5)))
	return base + jitter
}
