package main

import (
	"fmt"
	"os"
	// "os"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/identity"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(newCSRCommand())
	rootCmd.AddCommand(newInstallCommand())
}

func newCSRCommand() *cobra.Command {
	var (
		connectorID string
		keyPath     string
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

			if appConfig.Connector.ID == "" {
				return fmt.Errorf("connector.id is empty in config")
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

			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Private key created: %s\n",
				keyPath,
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

	cmd.Flags().StringVar(&connectorID, "connector", "", "Connector ID")
	cmd.Flags().StringVar(&csrOut, "out", "", "Where to write CSR (default: <id>.csr)")

	_ = cmd.MarkFlagRequired("connector")

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
