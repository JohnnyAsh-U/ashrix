package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("Could not determine user home directory: %w", err)
		}

		configDir := filepath.Join(home, ".ashrix", "connector")
		logDir := filepath.Join(configDir, "logs")
		defaultPath := filepath.Join(configDir, "connector.yaml")

		if err := os.MkdirAll(configDir, 0o750); err != nil {
			return fmt.Errorf("Failed to create- config directory %q: %w", configDir, err)
		}

		if err := os.MkdirAll(logDir, 0o750); err != nil {
			return fmt.Errorf("Failed to create- log directory %q: %w", logDir, err)
		}

		if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
			if err := os.MkdirAll(configDir, 0o750); err != nil {
				return fmt.Errorf("Failed to create- config directory %q: %w", configDir, err)
			}

			defaultGatewayConfig := fmt.Sprintf(`connector:
  id: connector-01
  certificate: "%s"
  private_key: "%s"
  ca: "%s"

gateway:
  address: 
  server_name: 

applications:

logging:
  dir: "%s"
  level: info
  format: json

`,
				filepath.Join(configDir, "connector.crt"),
				filepath.Join(configDir, "connector.key"),
				filepath.Join(configDir, "bundle.crt"),
				filepath.Join(configDir, ""),
			)
			if err := os.WriteFile(defaultPath, []byte(defaultGatewayConfig), 0o600); err != nil {
				return fmt.Errorf("failed to write config %q: %w", defaultPath, err)
			}
		}

		fmt.Fprintf(os.Stderr, "Initialized default config at %s\n", defaultPath)

		viper.SetConfigFile(defaultPath)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("Failed to read specified config %q: %w", defaultPath, err)

		}
		return nil
	},
}
