package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/bootstrap"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
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

	// Load and validate config — fails loud if required values missing
	cfg, err := config.LoadFromViper()
	if err != nil {
		return err
	}
	if err := config.Validate(cfg); err != nil {
		return err
	}

	//Init logging

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

	// Refuse to start if another instance is already running.
	// Prevents two gateways running simultaneously with the same identity —
	// which is a security event for Ashrix, not just a bug.
	if bootstrap.IsPIDAlive(cfg.PIDFile) {
		return fmt.Errorf(
			"gateway already running (pid file: %s) — run 'ashrix-gateway stop' first",
			cfg.PIDFile,
		)
	}

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

	//Renew http Client
	client := bootstrap.NewClient(cfg.CPURL)

	//Initialize the Gateway PKI
	pki, err := crypto.NewGatewayPKI(cfg.GatewayID, cfg.DataDir, "SECRET", "GATEWAY", logging.App, client, cfg)

	if err != nil {
		return fmt.Errorf("failed to initialize gateway pki: %w", err)
	}

	pki.StartRotator()
	defer pki.Stop()


	log.Info("gateway started",
		zap.String("cp_url", cfg.CPURL),
		zap.String("data_dir", cfg.DataDir),
		zap.Int("pid", pid),
	)

	log.Info("Connecting to Control Plane GRPC...")

	// fmt.Println([]byte(gatewayInfo.Certificate))

	// TODO: pass id.TLSCert into the proxy/listener
	// proxy := proxy.New(cfg, id)
	// return proxy.ListenAndServe()

	// Block until SIGTERM or SIGINT (Ctrl+C / systemd stop / our stop command).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit

	log.Info("shutting down", zap.String("signal", sig.String()))
	// // TODO: proxy.Shutdown(ctx)

	return nil
}
