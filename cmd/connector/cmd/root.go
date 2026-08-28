package cmd

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/storage"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "1.0.0"
	Storage storage.Storage
)

// rootCmd is the parent of all subcommands.
// It has no RunE — running ashrix-connector alone prints help.
var rootCmd = &cobra.Command{
	Use:   "ashrix-connector",
	Short: "Ashrix Connector — Secure tunnel management",
	Long: `Ashrix Connector connects your internal applications to the
Ashrix Gateway, enabling secure access via your existing
identity provider — no VPN required.

Run 'ashrix-connector start' to connect your internal application`,
	Version: version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initViper()
		var err error
		Storage, err = storage.NewStorage()
		if err != nil {
			log.Fatalf("Failed to initialize storage: %v", err)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringP("token", "t", "", "Ephemeral token from Control Plane")

	viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token"))
}

func initConfig() {
	home, err := os.UserConfigDir()
	if err != nil {
		os.Exit(1)
	}
	var BaseDir string
	if runtime.GOOS == "windows" {
		BaseDir = filepath.Join(home, "Ashrix")
	} else {
		BaseDir = filepath.Join(home, "ashrix")
	}

	certDir := filepath.Join(BaseDir, "certs")
	logDir := filepath.Join(BaseDir, "logs")

	//Create the each folder; may exist or may not
	// Create certs directory
	err = os.MkdirAll(certDir, 0700)
	if err != nil {
		os.Exit(1)
	}

	// Create logs directory
	err = os.MkdirAll(logDir, 0700)
	if err != nil {
		os.Exit(1)
	}

	viper.Set("basedir", BaseDir)
	viper.Set("certdir", certDir)
	viper.Set("logdir", logDir)
}

func initViper() {
	viper.SetDefault("cp_url", "http://localhost:8001")
	viper.SetDefault("connector.heartbeat_interval", 30)
	viper.SetDefault("connector.reconnect_attempts", 5)
}
