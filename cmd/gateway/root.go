package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)


// rootCmd is the parent of all subcommands.
// It has no RunE — running ashrix-gateway alone prints help.
var rootCmd = &cobra.Command{
	Use:   "ashrix-gateway",
	Short: "Ashrix self-hosted application access gateway",
	Long:  `Ashrix accepts application requests and routes them through outbound QUIC connectors. Configure gateway.yaml and provision the gateway identity files before starting.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("Could not determine user home directory: %w", err)
			}

			configDir := filepath.Join(home, ".ashrix")
			pkiDir := filepath.Join(configDir, "pki")
			logsDir := filepath.Join(configDir, "logs")

			if err := os.MkdirAll(pkiDir, 0o750); err != nil {
				return fmt.Errorf("Failed to create- config direactory %q: %w", configDir, err)
			}

			if err := os.MkdirAll(logsDir, 0o750); err != nil {
				return fmt.Errorf("Failed to create- config direactory %q: %w", logsDir, err)
			}
			
			defaultPath := filepath.Join(configDir, "gateway.yaml")

			if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
				if err := os.MkdirAll(pkiDir, 0o750); err != nil {
					return fmt.Errorf("Failed to create- config direactory %q: %w", configDir, err)
				}

				defaultGatewayConfig := fmt.Sprintf(`gateway:
  id: gateway-01
  certificate: "%s"
  private_key: "%s"

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
  ca_certificate: "%s"
  ca_private_key: "%s"

connectors:

applications:

logging:
  dir: "%s"
  level: info
  format: json

`,

					filepath.Join(configDir, "pki", "gateway.crt"),
					filepath.Join(configDir, "pki", "gateway.key"),
					filepath.Join(configDir, "server.crt"),
					filepath.Join(configDir, "server.key"),
					filepath.Join(configDir, "pki", "ca_cert.crt"),
					filepath.Join(configDir, "pki", "ca_key.key"),
					logsDir,
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
