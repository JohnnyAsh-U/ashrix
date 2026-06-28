package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/startup"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func init() {
	startCmd.Flags().String("token", "", "First Time Token")


	viper.BindPFlag("token", startCmd.Flags().Lookup("token"))
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Ashrix Connector",
	Long:  `Start the Ashrix Gateway`,
	RunE:  runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting Connector...")
	baseDir := config.BaseDir()
	if err := config.InitDirs(); err != nil {
		fmt.Println(err)
	} //Creates Data dir for certs and logs
	certDir := filepath.Join(baseDir, "certs")
	logDir := filepath.Join(baseDir, "logs")

	CPURL := "http://localhost:8001"

	token := viper.GetString("token")

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

	if err := connector.LoggerInit(connector.LoggerConfig{
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

	defer connector.LoggerApp.Sync()

	log := connector.LoggerApp

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

	result, err := startup.Run(ctx, CPURL, certDir, "SECRET", "CONNECTOR", log, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nFATAL: %s\n\n", err.Error())
		os.Exit(1)
	}

	defer result.PKI.Stop()

	log.Info("Startup Complete",
		zap.String("connector_id", result.Status.ConnectorId),
		zap.String("gateway_id", result.Status.GatewayId),
		zap.String("gateway_url", result.Status.GatewayUrl),
		zap.Int("apps", len(result.Status.Apps)),
	)

	//------------------------ Build Transport config from status response-----------------------------------//

	gatewayAddr := result.Status.GatewayUrl
	if gatewayAddr == "" {
		fmt.Println("Gateway URL not valid")
		os.Exit(1)
	}

	// tlsConfig := result.PKI.TLSConfig()

	//--------------------------Build Connector config -------------------------------//


	//--------------------------Start Cert Rotator------------------------------------//
	result.PKI.StartRotator()
	log.Info("Connector ready - Opening management stream with gateway")




	<-ctx.Done()
	log.Info("Connector stopped")
	return nil
}
