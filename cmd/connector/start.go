package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"

	// "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/identity"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/logger"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/startup"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/tunnel"
	"github.com/JohnnyAsh-U/ashrix-api/internal/telemetry"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	// _ "net/http/pprof"
)

func init() {
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Ashrix Connector",
	Long:  `Start the Ashrix Gateway`,
	RunE: runStart,
}

func runStart(cmd *cobra.Command, args []string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "FATAL: Panic in connector runStart: %v\n", r)
			err = fmt.Errorf("panic in connector: %v", r)
		}
	}()

	var appConfig config.Config
	err = viper.Unmarshal(&appConfig)
	if err != nil {
		return fmt.Errorf("couldn't load config: %w", err)
	}

	if err := appConfig.Validate(); err != nil {
		return fmt.Errorf("Validation Error: %w", err)
	}

	

	//------------------------Logger Initializer---------------------------
	fmt.Println("Initializing Logger...")

	if err := logger.LoggerInit(logger.LoggerConfig{
		Env:        "prod",
		LogDir:     appConfig.Logging.Dir,
		Level:      "info",
		MaxSizeMB:  100,
		MaxAgeDays: 30,
		MaxBackups: 10,
		Compress:   true,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: cannot init logging %v\n", err)
		os.Exit(1)
	}

	log := logger.LoggerApp

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	telemetryConfig, telemetryConfigErr := telemetry.ConfigFromEnv("ashrix-connector", "")
	if telemetryConfigErr != nil {
		log.Warn("OpenTelemetry configuration invalid; continuing without telemetry", "error", telemetryConfigErr)
	} else {
		// serviceName = telemetryConfig.ServiceName
		switch {
		case !telemetryConfig.Enabled:
			log.Info("OpenTelemetry disabled")
		case telemetryConfig.Endpoint == "":
			log.Warn("OpenTelemetry not started: OTEL_EXPORTER_OTLP_ENDPOINT is not set")
		default:
			provider, telemetryErr := telemetry.Bootstrap(ctx, telemetryConfig)
			if telemetryErr != nil {
				log.Warn("OpenTelemetry initialization failed; continuing without telemetry", "error", telemetryErr)
			} else {
				log = telemetry.AddSlogHandler(log, telemetry.NewSlogHandler(provider.LoggerProvider))
				defer func() {
					shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer shutdownCancel()
					if err := provider.Shutdown(shutdownCtx); err != nil {
						log.Warn("OpenTelemetry shutdown failed", "error", err)
					}
				}()
				log.Info("OpenTelemetry initialized", "endpoint", telemetryConfig.Endpoint)
			}
		}
	}

	//-------------------------Signal Handling---------------------------//

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-quit
		log.Info("Shut down signal received", "signal", sig.String())
		cancel()
	}()

	// credential, err := appStorage.LoadCredential()
	key, err := identity.LoadPrivateKey(appConfig.Connector.PrivateKey)
	if err != nil {
		return fmt.Errorf("load local connector credentials: %w", err)
	}

	cert, err := identity.LoadCert(appConfig.Connector.Certificate)

	if err != nil {
		return fmt.Errorf("load local connector credentials: %w", err)
	}

	bundle, err := identity.LoadCert(appConfig.Connector.CA)

	if err != nil {
		return fmt.Errorf("load local connector credentials: %w", err)
	}


	if cert.Subject.CommonName != appConfig.Connector.ID {
		return fmt.Errorf("configured connector ID %q does not match certificate identity %q", appConfig.Connector.ID, cert.Subject.CommonName)
	}
	if time.Now().After(cert.NotAfter) {
		return fmt.Errorf("connector certificate expired; provision a renewed certificate and trust bundle")
	}

	pki, err := startup.NewLocalPKIInitialiser(cert, key, string(bundle.Raw))
	if err != nil {
		return fmt.Errorf("initialize local connector identity: %w", err)
	}
	connectorID := appConfig.Connector.ID
	apps := make([]*pb.ConnectorApps, 0, len(appConfig.Applications))
	for _, app := range appConfig.Applications {
		apps = append(apps, &pb.ConnectorApps{
			Id:                    app.Name,
			Name:                  app.Name,
			Upstream:              app.Target,
		})
	}

	transportConfig := transport.Config{
		GatewayQUICAddr: appConfig.Gateway.Address,
		ConnectorID:     connectorID,
		TLSConfig:       pki.TLSConfig(),
	}

	attempt := 0
	for {
		if ctx.Err() != nil {
			return nil
		}
		attempt++

		//--------------------Mangement Receiver, HeartBeat, and Tunnel Loop ---------------------------//
		sessionCtx, sessionCancel := context.WithCancel(ctx)
		errCh := make(chan error, 1)



		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("panic in tunnel loop", "panic", r)
					errCh <- fmt.Errorf("tunnel loop panic: %v", r)
				}
			}()
			errCh <- runTunnelLoop(sessionCtx, transportConfig, log, apps)
		}()

		// go func() {
		// 	log.Info("Connector pprof server started", "addr", "127.0.0.1:6061")

		// 	if err := http.ListenAndServe("127.0.0.1:6061", nil); err != nil {
		// 		log.Error("Connector pprof server stopped", "error", err)
		// 	}
		// }()

		select {
		case err := <-errCh:
			if err != nil {
				log.Error("Session Shut Down", "error", err)
			}
		case <-ctx.Done():
			sessionCancel()
			return nil
		}

		sessionCancel()

		// Sleep briefly before reconnecting
		select {
		case <-time.After(reconnectDelay(1)):
		case <-ctx.Done():
			return nil
		}
	}
}

func runTunnelLoop(ctx context.Context, config transport.Config, log *slog.Logger, apps []*pb.ConnectorApps) error {
	attempt := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		attempt++

		transportProto, err := transport.Negotiate(ctx, config, log)
		if err != nil {
			delay := reconnectDelay(attempt)
			log.Warn("tunnel negotiation failed — reconnecting",
				"error", err,
				"retry_in", delay,
				"attempt", attempt,
			)
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		log.Info("connector ready",
			"connector_id", config.ConnectorID,
			"tunnel", transportProto.TransportName(),
		)

		// Record connection time to check for stability
		connectTime := time.Now()

		
		// Tunnel: accept and proxy requests
		errCh := make(chan error, 1)
		connTunnel := tunnel.NewTunnel(log, apps)

		go func() {
			errCh <- connTunnel.AcceptLoop(ctx, transportProto)
		}()

		var tunnelErr error
		select {
		case tunnelErr = <-errCh:
			if tunnelErr != nil {
				log.Error("Tunnel error", "error", tunnelErr)
			}
		case <-ctx.Done():
			transportProto.Close()
			return ctx.Err()
		}

		transportProto.Close()

		// Reset attempt counter if the connection was stable (> 60 seconds)
		if time.Since(connectTime) > 60*time.Second {
			attempt = 0
		}

		// Transient — reconnect with backoff
		delay := reconnectDelay(attempt + 1)
		log.Warn("disconnected — reconnecting",
			"error", tunnelErr,
			"retry_in", delay,
			"attempt", attempt,
		)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// reconnectDelay returns exponential backoff with jitter and 60s cap.
func reconnectDelay(attempt int) time.Duration {
	base := time.Duration(1<<attempt) * time.Second
	if base > 10*time.Second {
		base = 10 * time.Second
	}
	jitter := time.Duration(rand.Int63n(int64(base / 5)))
	return base + jitter
}
