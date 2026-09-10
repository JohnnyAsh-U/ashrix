package cmd

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
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/health"
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
	// _ "net/http/pprof"
)

func init() {
	startCmd.Flags().String("socks-addr", ":1080", "Local SOCKS5 proxy server address")
	_ = viper.BindPFlag("socks_addr", startCmd.Flags().Lookup("socks-addr"))
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
		Env:        "prod",
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

	log := logger.LoggerApp

	ctx, cancel := context.WithCancel(context.Background())

	//-------------------------Signal Handling---------------------------//

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-quit
		log.Info("Shut down signal received", "signal", sig.String())
		cancel()
	}()

	attempt := 0
	for {
		if ctx.Err() != nil {
			return nil
		}
		attempt++

		//-------------------------Startup Workflow---------------------------//
		//Check Cert -> renew or register
		log.Info("Running startup workflow and connecting with CP", "cp_url", CPURL)

		result, err := startup.Run(ctx, CPURL, log, token, appStorage)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nFATAL: %s\n\n", err.Error())
			os.Exit(1)
		}

		result.PKI.StartRotator()
		defer result.PKI.Stop()

		log.Info("Startup Complete",
			"connector_id", result.Status.ConnectorId,
			"gateway_id", result.Status.GatewayId,
			"gateway_url", result.Status.GatewayUrl,
			"apps", len(result.Status.Apps),
		)

		//------------------------ Build Transport targets from status response-------------------------//
		tlsConfig := result.PKI.TLSConfigWithCRL(result.Status.GetCrlEntries())
		connectorID := result.Status.ConnectorId
		tenantID := result.Status.TenantId
		apps := result.Status.Apps

		type GatewayTarget struct {
			Ip          string
			GrpcPort    string
			QuicPort    string
			IsSecondary bool
		}

		var targets []GatewayTarget
		if result.Status.GatewayIp != "" {
			targets = append(targets, GatewayTarget{
				Ip:          result.Status.GatewayIp,
				GrpcPort:    result.Status.GrpcPort,
				QuicPort:    result.Status.QuicPort,
				IsSecondary: false,
			})
		}
		if result.Status.SecondaryGatewayIp != "" {
			targets = append(targets, GatewayTarget{
				Ip:          result.Status.SecondaryGatewayIp,
				GrpcPort:    result.Status.SecondaryGrpcPort,
				QuicPort:    result.Status.SecondaryQuicPort,
				IsSecondary: true,
			})
		}

		if len(targets) == 0 {
			log.Error("Gateway address is empty / invalid")
			os.Exit(1)
		}

		currentTarget := targets[(attempt-1)%len(targets)]
		log.Info("Selecting Gateway target",
			"target_ip", currentTarget.Ip,
			"is_secondary", currentTarget.IsSecondary,
			"attempt", attempt,
		)

		//---------------------------Config for selected Transport Target---------------------------------------------//
		config := transport.Config{
			GatewayQUICAddr: currentTarget.Ip + ":" + currentTarget.QuicPort,
			GatewayGRPCAddr: currentTarget.Ip + ":" + currentTarget.GrpcPort,
			GatewayWSURL:    "wss://" + currentTarget.Ip + "/ws",
			ConnectorID:     connectorID,
			OpenSock:        result.Status.OpenSock,
			TLSConfig:       tlsConfig,
		}

		//-----------------------Build the Management Stream & Tunnel Loop------------------------------//
		log.Info("Opening management stream with gateway", "addr", config.GatewayGRPCAddr, "is_secondary", currentTarget.IsSecondary, "attempt", attempt)
		managementConn, err := management.OpenStream(ctx, config.GatewayGRPCAddr, connectorID, tenantID, tlsConfig, log, apps, result.PKI, appStorage)
		if err != nil {
			log.Error("Failed to open Gateway stream", "addr", config.GatewayGRPCAddr, "is_secondary", currentTarget.IsSecondary, "error", err)
			select {
			case <-time.After(reconnectDelay(attempt)):
				continue
			case <-ctx.Done():
				return nil
			}
		}

		// ---------------------Hello handshake-------------------------------------------//
		helloCtx, helloCancel := context.WithTimeout(ctx, 10*time.Second)
		err = managementConn.Register(helloCtx)
		helloCancel()
		if err != nil {
			log.Error("Failed to register management stream", "addr", config.GatewayGRPCAddr, "error", err)
			managementConn.Conn.Close()
			select {
			case <-time.After(reconnectDelay(attempt)):
				continue
			case <-ctx.Done():
				return nil
			}
		}

		log.Info("connector management stream ready", "connector_id", connectorID, "app_addr", config.GatewayGRPCAddr, "is_secondary", currentTarget.IsSecondary)

		//--------------------Mangement Receiver, HeartBeat, and Tunnel Loop ---------------------------//
		sessionCtx, sessionCancel := context.WithCancel(ctx)
		errCh := make(chan error, 3)

		healthChecker := health.NewChecker(log, apps)
		go healthChecker.Start(sessionCtx)
		managementConn.Checker = healthChecker

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("panic in management receiver", "panic", r)
					errCh <- fmt.Errorf("management receiver panic: %v", r)
				}
			}()
			errCh <- managementConn.RunReceiver(sessionCtx)
		}()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("panic in management heartbeat", "panic", r)
					errCh <- fmt.Errorf("management heartbeat panic: %v", r)
				}
			}()
			errCh <- managementConn.RunHeartbeat(sessionCtx, 10*time.Second)
		}()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("panic in tunnel loop", "panic", r)
					errCh <- fmt.Errorf("tunnel loop panic: %v", r)
				}
			}()
			errCh <- runTunnelLoop(sessionCtx, config, log, apps, managementConn)
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
			managementConn.Conn.Close()
			return nil
		}

		sessionCancel()
		managementConn.Conn.Close()

		// Sleep briefly before reconnecting
		select {
		case <-time.After(reconnectDelay(1)):
		case <-ctx.Done():
			return nil
		}
	}
}

func runTunnelLoop(ctx context.Context, config transport.Config, log *slog.Logger, apps []*pb.ConnectorApps, managementConn *management.ManagementConn) error {
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
			"app_addr", config.GatewayGRPCAddr,
			"tunnel", transportProto.TransportName(),
		)

		// Record connection time to check for stability
		connectTime := time.Now()

		socksAddr := viper.GetString("socks_addr")

		var socksServer *socks.Server
		if config.OpenSock {
			log.Info("Starting SOCKS5 proxy server on connector", "addr", socksAddr)
			credentialStore, err := socks.NewCredentialStore()
			if err != nil {
				log.Error("SOCKS5 server cred error", "error", err)
			}
			for _, app := range apps {
				credentialStore.Replace([]socks.Credential{{
					AppID:        app.Id,
					PasswordHash: app.SockPass,
				}})
			}
			socksServer = socks.NewServer(socksAddr, credentialStore, transportProto, log)
			go func() {
				if err := socksServer.ListenAndServe(ctx); err != nil {
					log.Error("SOCKS5 server error", "error", err)
				}
			}()
		}

		// Tunnel: accept and proxy requests
		errCh := make(chan error, 1)
		connTunnel := tunnel.NewTunnel(log, apps, managementConn)

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
