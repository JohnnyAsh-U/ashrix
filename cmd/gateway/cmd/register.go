package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/bootstrap"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/logging"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func init() {
	registerCmd.Flags().String(
		"cp-url", "",
		"Control Plane URL",
	)

	registerCmd.Flags().String(
		"token", "",
		"First Time Token",
	)

	// Token is intentionally NOT a flag — it would appear in ps aux and shell history.
	// Operators must use the env var: ASHRIX_TOKEN=tok_xxx ashrix-gateway register
	// This is enforced by the absence of a flag binding for token.

	viper.BindPFlag("cp_url", registerCmd.Flags().Lookup("cp-url"))
	viper.BindPFlag("token", registerCmd.Flags().Lookup("token"))

	rootCmd.AddCommand(registerCmd)
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register this gateway with the Ashrix Control Plane",
	Long: `Register this gateway with the Ashrix Control Plane.

This command:
  1. Generates an ECDSA P-256 keypair
  2. Generates a CSR (Certificate Signing Request)
  3. POSTs the CSR to the Control Plane using your bootstrap token
  4. Receives and stores the signed certificate
  5. Writes gateway.yaml with the assigned gateway ID

The bootstrap token must be set via environment variable:
  export ASHRIX_TOKEN=tok_xxx
  ashrix-gateway register --cp-url=https://cp.ashrix.io

The token is never written to disk.
After registration, run: ashrix-gateway start`,
	RunE: runRegister,
}

func runRegister(cmd *cobra.Command, args []string) error {

	// Token must come from env — never from a flag.
	// If token is empty, fail immediately with a clear message.
	token := viper.GetString("token")
	if token == "" {
		return fmt.Errorf(
			"bootstrap token required\n\n" +
				"Set it via flag:\n" +
				"ashrix-gateway register --token=<string>\n",
		)
	}

	cpURL := viper.GetString("cp_url")
	if cpURL == "" {
		return fmt.Errorf(
			"control plane URL required\n\n" +
				"Set via flag\n" +
				"ashrix-gateway register --cp-url=https://cp.ashrix.io\n",
		)
	}

	Home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("Error %s", err)
	}
	dataDir := filepath.Join(Home, "/.ashrix/gateway/data")
	logDir := filepath.Join(Home, "/.ashrix/gateway/logs")


	//Init logging

	if err := logging.Init(logging.Config{
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

	defer logging.App.Sync()

	log := logging.App

	// Create data directory if it doesn't exist.
	// 0700 — only the gateway process user can read/write it.
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("failed to create data directory %s: %w", dataDir, err)
	}

	// Create data directory if it doesn't exist.
	// 0700 — only the gateway process user can read/write it.
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("failed to create data directory %s: %w", dataDir, err)
	}

	cfg := &config.Config{
		CPURL:   cpURL,
		Token:   token,
		DataDir: dataDir,
		LogDir:  logDir,
	}

	fmt.Println("Starting Gateway...")

	fmt.Println("Generating ECDSA P-256 keypair...")

	priv, err := crypto.GenerateECDSAP256()

	if err != nil {
		return err
	}

	fmt.Println("Generating CSR...")

	csrPem, err := crypto.GenerateCSR(
		priv,
	)

	if err != nil {
		return err
	}

	fmt.Println("Connecting to Control Plane: ", cfg.CPURL)

	client := bootstrap.NewClient(cfg.CPURL)

	fmt.Println("Registering Gateway...")

	apiResp, err := client.Bootstrap(cfg.Token, string(csrPem))

	if err != nil {
		return err
	}

	if err := bootstrap.VerifyResponse(apiResp); err != nil {
		return fmt.Errorf("Invalid CP Response %w", err)
	}

	fmt.Println("Bootstrap Successful", &apiResp.Data.GatewayId)

	fmt.Println(apiResp.Data.Certificate)

	log.Info("Writing config, cert, bundle and key to directory")

	
	configDir := filepath.Join(Home, "/.ashrix/gateway")

	writeErr := utils.WriteConfig(cfg.CPURL, apiResp.Data.GatewayId, apiResp.Data.GatewayName,apiResp.Data.GatewayUrl, cfg.DataDir, configDir, cfg.LogDir)

	if writeErr != nil {
		return writeErr
	}

	if saveErr := utils.SaveKeyAndCertAndBundle(priv, apiResp.Data.Certificate, apiResp.Data.TrustBundle, cfg.DataDir, "SECRET", "GATEWAY"); saveErr != nil {
		return saveErr
	}

	logging.Audit.Log(logging.AuditEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		EventType: logging.EventGatewayRegistered,
		GatewayID: apiResp.Data.GatewayId,
		Decision:  "ALLOW",
	})

	log.Info("Gateway Registered", zap.String("gateway_id", apiResp.Data.GatewayId), zap.String("cp_url", cfg.CPURL))

	fmt.Println("Now run 'ashrix-gateway start' to run the gateway")

	return nil
}
