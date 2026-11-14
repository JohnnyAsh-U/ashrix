package main

import (
	"fmt"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(validateConfigCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show Ashrix Gateway config",
	RunE: func(cmd *cobra.Command, args []string) error {
		var appConfig config.Config
		err := viper.Unmarshal(&appConfig)
		if err != nil {
			fmt.Println("couldn't load config")
		}

		fmt.Println("=== Ashrix Gateway Status ===")
		fmt.Printf("\nGateway: %s\n", appConfig.Gateway.ID)
		fmt.Printf("HTTP Addr: %s\n", appConfig.Listeners.HTTP.Address)
		fmt.Printf("HTTPS Addr: %s\n", appConfig.Listeners.HTTPS.Address)
		fmt.Printf("HTTPS Certificate: %s\n", appConfig.Listeners.HTTPS.Certificate)
		fmt.Printf("HTTPS Private Key: %s\n", appConfig.Listeners.HTTPS.PrivateKey)
		fmt.Printf("QUIC Addr: %s\n", appConfig.Listeners.QUIC.Address)
		fmt.Printf("PKI CA Certificate: %s\n", appConfig.PKI.CACertificate)
		fmt.Printf("PKI CA Private Key: %s\n", appConfig.PKI.CAPrivateKey)

		fmt.Printf("Logging: %s;%s\n", appConfig.Logging.Level, appConfig.Logging.Format)

		fmt.Println("=== Connectors ===")

		for _, conn := range appConfig.Connectors {
			fmt.Printf("Name: %s\n", conn.ID)
			fmt.Printf("Certificate Serial: %s\n", conn.CertificateSerial)
		}

		fmt.Println("=== Applications ===")

		for _, app := range appConfig.Applications {
			fmt.Printf("Name: %s\n", app.Name)
			fmt.Printf("Domain: %v\n", app.Domain)
			fmt.Printf("Connector: %v\n", app.Connector)
		}

		if err != nil {
			fmt.Println("certificate or key: not found — provision gateway identity files")
			return nil
		}
		return nil
	},
}

var validateConfigCmd = &cobra.Command{
	Use:   "test-config",
	Short: "Validate Ashrix Gateway config",
	RunE: func(cmd *cobra.Command, args []string) error {
		var appConfig config.Config
		err := viper.Unmarshal(&appConfig)
		if err != nil {
			fmt.Println("couldn't load config")
		}

		fmt.Println("=== Ashrix Gateway Status ===")
		fmt.Printf("\nGateway: %s\n", appConfig.Gateway.ID)
		fmt.Printf("HTTP Addr: %s\n", appConfig.Listeners.HTTP.Address)
		fmt.Printf("HTTPS Addr: %s\n", appConfig.Listeners.HTTPS.Address)
		fmt.Printf("HTTPS Certificate: %s\n", appConfig.Listeners.HTTPS.Certificate)
		fmt.Printf("HTTPS Private Key: %s\n", appConfig.Listeners.HTTPS.PrivateKey)
		fmt.Printf("QUIC Addr: %s\n", appConfig.Listeners.QUIC.Address)
		fmt.Printf("PKI CA Certificate: %s\n", appConfig.PKI.CACertificate)
		fmt.Printf("PKI CA Private Key: %s\n", appConfig.PKI.CAPrivateKey)

		fmt.Printf("Logging: %s;%s\n", appConfig.Logging.Level, appConfig.Logging.Format)

		fmt.Println("=== Connectors ===")

		for _, conn := range appConfig.Connectors {
			fmt.Printf("Name: %s\n", conn.ID)
			fmt.Printf("Certificate Serial: %s\n", conn.CertificateSerial)
		}

		fmt.Println("=== Applications ===")

		for _, app := range appConfig.Applications {
			fmt.Printf("Name: %s\n", app.Name)
			fmt.Printf("Domain: %v\n", app.Domain)
			fmt.Printf("Connector: %v\n", app.Connector)
		}

		if err := appConfig.Validate(); err != nil {
			return err
		}

		return nil

	}}
