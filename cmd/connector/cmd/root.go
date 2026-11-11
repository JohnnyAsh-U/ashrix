package cmd

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
	Short: "Ashrix outbound application connector",
	Long: `Ashrix Connector connects your internal applications to the
Ashrix Gateway over an outbound QUIC tunnel.

Configure connector identity, gateway endpoints, and apps in connector.yaml,
then run 'ashrix-connector start'.`,
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
	viper.SetConfigName("connector")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(BaseDir)
	viper.AddConfigPath(".")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Fatalf("failed to read connector config: %v", err)
		}
	}
}

func initViper() {
	viper.SetDefault("connector.open_sock", false)
	viper.SetDefault("socks_addr", "127.0.0.1:1080")
}
