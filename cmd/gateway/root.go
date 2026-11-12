package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd is the parent of all subcommands.
// It has no RunE — running ashrix-gateway alone prints help.
var rootCmd = &cobra.Command{
	Use:   "ashrix-gateway",
	Short: "Ashrix self-hosted application access gateway",
	Long:  `Ashrix accepts application requests and routes them through outbound QUIC connectors. Configure gateway.yaml and provision the gateway identity files before starting.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initConfig()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}



func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Path to custom config file(disables auto-creation")
}



func initConfig() error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("Failed to read specified config %q: %w", cfgFile, err)
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("Could not determine user home directory: %w", err)
		}

		configDir := filepath.Join(home, ".ashrix")
		defaultPath := filepath.Join(configDir, "gateway.yaml")

		if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
			if err := os.MkdirAll(configDir, 0o750); err != nil {
				return fmt.Errorf("Failed to create- config direactory %q: %w", configDir, err)
			}
		}
		defaultGatewayConfig := fmt.Sprintf(`gateway:
  id: gateway-01

listeners:
  http:
    address: "0.0.0.0:80"

  https:
    address: "0.0.0.0:443"
    certificate: "%s"
    private_key: "%s"

  quic:
    address: "0.0.0.0:8443"
	
  pki:
   ca_certificate: %s
   ca_private_key: %s
`,
	filepath.Join(configDir, "server.crt"), 
	filepath.Join(configDir, "server.key"),
	filepath.Join(configDir, "pki", "ca_cert.crt"),
	filepath.Join(configDir, "pki", "ca_key.key"),
)
		if err := os.WriteFile(defaultPath, []byte(defaultGatewayConfig), 0o600); err != nil {
			return fmt.Errorf("failed to write config %q: %w", defaultPath, err)
		}
		fmt.Fprintf(os.Stderr, "Initialized default config at %s\n", defaultPath)

		viper.SetConfigFile(defaultPath)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("Failed to read specified config %q: %w", cfgFile, err)
		}
	}
	return nil
}
