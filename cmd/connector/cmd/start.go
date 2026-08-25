package cmd

import (
	"context"
	"fmt"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/logger"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/management"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/socks"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/startup"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/storage"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/transport"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/tunnel"
	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func init() {
	startCmd.Flags().String("socks-addr", ":1080", "Local SOCKS5 proxy server address")
	startCmd.Flags().String("socks-user", "", "Local SOCKS5 username")
	startCmd.Flags().String("socks-pass", "", "Local SOCKS5 password")
	_ = viper.BindPFlag("socks_addr", startCmd.Flags().Lookup("socks-addr"))
	_ = viper.BindPFlag("socks_user", startCmd.Flags().Lookup("socks-user"))
	_ = viper.BindPFlag("socks_pass", startCmd.Flags().Lookup("socks-pass"))
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Ashrix Connector",
	Long:  `Start the Ashrix Gateway`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := runStart(); err != nil {
			return err
		}
		return nil
	},
}

func runStart() (err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "FATAL: Panic in connector runStart: %v\n", r)
			err = fmt.Errorf("panic in connector: %v", r)
		}
	}()
	fmt.Println("Starting Connector...")
	appStorage, err := storage.NewStorage()

	if err != nil {
		fmt.Fprintln(os.Stderr, "Storage Error", err)
		os.Exit(1)
	}

	token := viper.GetString("token")
	baseDir := viper.GetString("basedir")
	logDir := viper.GetString("logdir")
	CPURL := viper.GetString("cp_url")

	//-----------------------PID Checker----------------------------------
	fmt.Println("Checking and Creating PID")

	locker := config.NewPID(baseDir)
	if err := locker.Acquire(); err != nil {
		fmt.Fprintln(os.Stderr, "Exit", err) //Another process is running
		os.Exit(1)
	}

	defer locker.Release() //unlock on exit/crtl+c

	//------------------------Logger Initializer---------------------------
	fmt.Println("Initializing Logger...")

	if err := logger.LoggerInit(logger.LoggerConfig{
		Env:        "dev",
		LogDir:     logDir,
		Level:      "info",
		MaxSizeMB:  100,
		MaxAgeDays: 30,
		MaxBackups: 10,
		Compress:   true,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: cannot init logging %v\n", err)
		os.Exit(1)
	}

	defer logger.LoggerApp.Sync()

	log := logger.LoggerApp

	ctx, cancel := context.WithCancel(context.Background())

	//-------------------------Signal Handling---------------------------//

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-quit
		log.Info("Shut down signal recived", zap.String("signal", sig.String()))
		cancel()
	}()

	//-------------------------Startup Workflow---------------------------//
	//Check Cert -> renew or register
	log.Info("Running startup workflow and connecting with CP", zap.String("cp_url", CPURL))

	result, err := startup.Run(ctx, CPURL, log, token, appStorage)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nFATAL: %s\n\n", err.Error())
		os.Exit(1)
	}

	result.PKI.StartRotator()
	defer result.PKI.Stop()

	log.Info("Startup Complete",
		zap.String("connector_id", result.Status.ConnectorId),
		zap.String("gateway_id", result.Status.GatewayId),
		zap.String("gateway_url", result.Status.GatewayUrl),
		zap.Int("apps", len(result.Status.Apps)),
	)

	//------------------------ Build Transport config from status response-------------------------//

	gatewayAddr := result.Status.GatewayIp
	fmt.Println(gatewayAddr)
	if gatewayAddr == "" {
		fmt.Println("Gateway URL not valid")
		os.Exit(1)
	}

	tlsConfig := result.PKI.TLSConfig()
	connectorID := result.Status.ConnectorId
	tenantID := result.Status.TenantId
	apps := result.Status.Apps

	//-----------------------Build the Management Stream & Tunnel Loop------------------------------//
	gRPCURL := result.Status.GatewayIp + ":9444"

	//---------------------------Config for different Transport---------------------------------------------//
	config := transport.Config{
		GatewayQUICAddr: result.Status.GatewayIp + ":9445",
		GatewayGRPCAddr: result.Status.GatewayIp + ":9444",
		GatewayWSURL:    "wss://" + result.Status.GatewayIp + "/ws",
		ConnectorID:     connectorID,
		// T
		TLSConfig: tlsConfig,
	}

	attempt := 0
	for {
		if ctx.Err() != nil {
			return nil
		}
		attempt++

		log.Info("Opening management stream with gateway", zap.String("addr", gRPCURL), zap.Int("attempt", attempt))
		streamConn, err := management.OpenStream(ctx, gRPCURL, connectorID, tenantID, tlsConfig, log, apps, result.PKI, appStorage)
		if err != nil {
			log.Error("Failed to open Gateway stream", zap.String("cp_url", gRPCURL), zap.Error(err))
			select {
			case <-time.After(reconnectDelay(attempt)):
				continue
			case <-ctx.Done():
				return nil
			}
		}

		// ---------------------Hello handshake-------------------------------------------//
		helloCtx, helloCancel := context.WithTimeout(ctx, 10*time.Second)
		err = streamConn.Register(helloCtx)
		helloCancel()
		if err != nil {
			log.Error("Failed to register management stream", zap.Error(err))
			streamConn.Conn.Close()
			select {
			case <-time.After(reconnectDelay(attempt)):
				continue
			case <-ctx.Done():
				return nil
			}
		}

		log.Info("✓ connector management stream ready", zap.String("connector_id", connectorID), zap.String("app_addr", gRPCURL))
		attempt = 0

		//--------------------Mangement Receiver, HeartBeat, and Tunnel Loop ---------------------------//
		sessionCtx, sessionCancel := context.WithCancel(ctx)
		errCh := make(chan error, 3)

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("panic in management receiver", zap.Any("panic", r))
					errCh <- fmt.Errorf("management receiver panic: %v", r)
				}
			}()
			errCh <- streamConn.RunReceiver(sessionCtx)
		}()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("panic in management heartbeat", zap.Any("panic", r))
					errCh <- fmt.Errorf("management heartbeat panic: %v", r)
				}
			}()
			errCh <- streamConn.RunHeartbeat(sessionCtx, 10*time.Second)
		}()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("panic in tunnel loop", zap.Any("panic", r))
					errCh <- fmt.Errorf("tunnel loop panic: %v", r)
				}
			}()
			errCh <- runTunnelLoop(sessionCtx, config, log, apps)
		}()

		select {
		case err := <-errCh:
			if err != nil {
				log.Error("Session lost due to component error", zap.Error(err))
			}
		case <-ctx.Done():
			sessionCancel()
			streamConn.Conn.Close()
			return nil
		}

		sessionCancel()
		streamConn.Conn.Close()

		// Sleep briefly before reconnecting
		select {
		case <-time.After(reconnectDelay(1)):
		case <-ctx.Done():
			return nil
		}
	}
}

func runTunnelLoop(ctx context.Context, config transport.Config, log *zap.Logger, apps []*pb.ConnectorApps) error {
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
				zap.Error(err),
				zap.Duration("retry_in", delay),
				zap.Int("attempt", attempt),
			)
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		log.Info("✓ connector ready",
			zap.String("connector_id", config.ConnectorID),
			zap.String("app_addr", config.GatewayGRPCAddr),
			zap.String("tunnel", transportProto.TransportName()),
		)

		// Record connection time to check for stability
		connectTime := time.Now()

		socksAddr := viper.GetString("socks_addr")
		socksUser := viper.GetString("socks_user")
		socksPass := viper.GetString("socks_pass")

		var socksServer *socks.Server
		if socksUser != "" && socksPass != "" {
			log.Info("Starting SOCKS5 proxy server on connector", zap.String("addr", socksAddr), zap.String("username", socksUser))
			socksServer = socks.NewServer(socksAddr, socksUser, socksPass, transportProto, log)
			go func() {
				if err := socksServer.Start(ctx); err != nil {
					log.Error("SOCKS5 server error", zap.Error(err))
				}
			}()
		}

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
				log.Error("Tunnel error", zap.Error(tunnelErr))
			}
		case <-ctx.Done():
			if socksServer != nil {
				socksServer.Stop()
			}
			transportProto.Close()
			return ctx.Err()
		}

		if socksServer != nil {
			socksServer.Stop()
		}
		transportProto.Close()

		// Reset attempt counter if the connection was stable (> 60 seconds)
		if time.Since(connectTime) > 60*time.Second {
			attempt = 0
		}

		// Transient — reconnect with backoff
		delay := reconnectDelay(attempt + 1)
		log.Warn("disconnected — reconnecting",
			zap.Error(tunnelErr),
			zap.Duration("retry_in", delay),
			zap.Int("attempt", attempt),
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
	if base > 60*time.Second {
		base = 60 * time.Second
	}
	jitter := time.Duration(rand.Int63n(int64(base / 5)))
	return base + jitter
}
