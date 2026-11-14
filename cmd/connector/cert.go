package main

import (
	"fmt"
	"os"

	// "os"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/identity"
	"github.com/JohnnyAsh-U/ashrix-api/internal/pki"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

func init() {
	rootCmd.AddCommand(newCSRCommand())
	rootCmd.AddCommand(newInstallCommand())
}

func newCSRCommand() *cobra.Command {
	var (
		connectorID string
		gatewayQuicAddr     string
		csrOut      string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate private key and CSR",
		RunE: func(cmd *cobra.Command, args []string) error {
			var appConfig config.Config
			err := viper.Unmarshal(&appConfig)
			if err != nil {
				return fmt.Errorf("couldn't load config")
			}

			if csrOut == "" {
				csrOut = appConfig.Connector.ID + ".csr"
			}

			if err := identity.CreateKeyAndCSR(
				appConfig.Connector.PrivateKey,
				appConfig.Connector.Certificate,
				connectorID,
				csrOut,
			); err != nil {
				return err
			}

			// Update the connector ID in the yaml file
			appConfig.Connector.ID = connectorID
			appConfig.Gateway.Address = gatewayQuicAddr
			out, err := yaml.Marshal(&appConfig)
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}
			if err := os.WriteFile(viper.ConfigFileUsed(), out, 0o644); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Private key created\n",
			)
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"CSR created: %s\n",
				csrOut,
			)
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Submit the CSR to the Ashrix gateway: %s. Never transfer the private key.\n",
				csrOut,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&connectorID, "name", "", "Connector ID")
	cmd.Flags().StringVar(&csrOut, "out", "", "Where to write CSR (default: <id>.csr)")
	cmd.Flags().StringVar(&gatewayQuicAddr, "quic-addr", "", "Gateway QUIC Address (ex: localhost:8444)")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("quic-addr")

	return cmd
}

func newInstallCommand() *cobra.Command {
	var (
		certPath string
		// caPath     string
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install gateway-signed connector certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			var appConfig config.Config
			err := viper.Unmarshal(&appConfig)
			if err != nil {
				return fmt.Errorf("couldn't load config")
			}

			// 1. Parse bundle: first = leaf, rest = CA(s)
			certPEM, err := os.ReadFile(certPath)
			if err != nil {
				return fmt.Errorf("read connector certificate: %w", err)
			}

			// Parse the cert and make sure it matches the connector id in the config yaml
			cert, err := pki.ParseCert(certPEM)
			if err != nil {
				return fmt.Errorf("read connector certificate: %w", err)
			}

			if cert.Subject.CommonName == "" || appConfig.Connector.ID != cert.Subject.CommonName {
				return fmt.Errorf(
					"Connector Name doesn't match the Certificate common name, Trying generating a new CSR with the current connector ID, or try modifying the config to match the cert subject Name")
			}

			if err := identity.InstallCertificate(
				appConfig,
				appConfig.Connector.ID,
				certPEM,
			); err != nil {
				return err
			}

			fmt.Fprintln(
				cmd.OutOrStdout(),
				"Connector certificate installed successfully.",
			)

			return nil
		},
	}

	cmd.Flags().StringVar(&certPath, "cert", "", "Signed connector certificate")

	_ = cmd.MarkFlagRequired("cert")

	return cmd
}
