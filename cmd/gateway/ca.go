package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/pki"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

var (
	connectorCSRPath  string
	connectorOutPath  string
	connectorDays     int
	gatewayID         string
	gatewayServerName string
)

func init() {
	createConnectorCmd.Flags().StringVar(&connectorCSRPath, "csr", "", "Path to connector CSR PEM file")
	createConnectorCmd.Flags().StringVar(&connectorOutPath, "out", "", "Path to write issued certificate (default: <pki-dir>/connectors/<id>.crt)")
	createConnectorCmd.Flags().IntVar(&connectorDays, "days", 30, "Connector certificate validity in days (maximum 30)")

	initCACmd.Flags().StringVar(&gatewayID, "name", "", "Gateway ID")
	initCACmd.Flags().StringVar(&gatewayServerName, "server-name", "", "Public DNS name or IP address used by connectors")
	rootCmd.AddCommand(initCACmd)
	rootCmd.AddCommand(showCACmd)
	rootCmd.AddCommand(createConnectorCmd)

	_ = initCACmd.MarkFlagRequired("name")
	_ = initCACmd.MarkFlagRequired("server-name")
}

var initCACmd = &cobra.Command{
	Use:   "init",
	Short: "Init Ashrix Gateway",
	RunE: func(cmd *cobra.Command, args []string) error {
		var appConfig config.Config

		if err := viper.Unmarshal(&appConfig); err != nil {
			return fmt.Errorf("couldn't load config: %w", err)
		}

		if gatewayServerName == "" {
			return fmt.Errorf("--server-name is required")
		}
		// appConfig.Gateway.ServerName = gatewayServerName

		// First Load CA and verify to be sure it doesn't exist
		var ca *pki.CA

		ca, err := pki.LoadCA(appConfig.PKI.CACertificate, appConfig.PKI.CAPrivateKey)

		if err != nil {
			fmt.Printf("Error Loading CA: %v\n", err)

			fmt.Println("Generating CA...")
			// Then Generate if id doesn't exist
			ca, err = pki.GenerateCA("Ashrix")
			if err != nil {
				return fmt.Errorf("generate CA: %w", err)
			}
			fmt.Println("Generating CA Done...")

			if err := pki.SaveCA(ca, appConfig.PKI.CACertificate, appConfig.PKI.CAPrivateKey); err != nil {
				return fmt.Errorf("Error Saving CA: %v\n", err)
			}
			fmt.Println("Saving Done...")
		}

		fmt.Println("CA Loaded")

		// Load Gateway CA, and verify chain with root ca

		var gatewayCA *crypto.LocalGatewayPKI

		gatewayCA, err = crypto.NewLocalGatewayPKI(
			appConfig.Gateway.Certificate,
			appConfig.Gateway.PrivateKey,
			ca.Certificate,
			appConfig.Gateway.ServerName,
		)

		if err != nil {
			// If fails, then generate key and csr
			key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				return fmt.Errorf("Generating signing key: %w", err)
			}

			csr, err := pki.GenerateCSR(key, gatewayID, "gateway")
			if err != nil {
				return fmt.Errorf("Generating csr: %w", err)
			}

			//Sign it
			_, gatewayCert, err := ca.IssueGatewayCertificate(gatewayID, gatewayServerName, csr, pki.MaxGatewayCertificateValidity)
			if err != nil {
				return fmt.Errorf("Issuing cert: %w", err)
			}

			//Save Key and Cert
			if err := crypto.SaveGatwayCertAndKey(appConfig.Gateway.Certificate, appConfig.Gateway.PrivateKey, gatewayCert, key); err != nil {
				return fmt.Errorf("Error Saving CA: %v\n", err)
			}
			fmt.Println("Saving Done...")
			gatewayCA, err = crypto.NewLocalGatewayPKI(
				appConfig.Gateway.Certificate,
				appConfig.Gateway.PrivateKey,
				ca.Certificate,
				gatewayServerName,
			)

			if err != nil {
				return fmt.Errorf("%w", err)
			}
		}

		// Update the gateway ID in the yaml file
		appConfig.Gateway.ID = gatewayID
		appConfig.Gateway.ServerName = gatewayServerName
		out, err := yaml.Marshal(&appConfig)
		if err != nil {
			return fmt.Errorf("marshal config: %w", err)
		}
		if err := os.WriteFile(viper.ConfigFileUsed(), out, 0o644); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		fmt.Println("Gateway:", gatewayID)
		fmt.Println("Root CA and Gateway Cert Loaded")
		fmt.Println("Gateway CERT Validity", gatewayCA.Cert().NotBefore, gatewayCA.Cert().NotAfter)
		return nil
	}}

var showCACmd = &cobra.Command{
	Use:   "show-ca",
	Short: "Show Ashrix Gateway CA",
	RunE: func(cmd *cobra.Command, args []string) error {
		var appConfig config.Config
		err := viper.Unmarshal(&appConfig)
		if err != nil {
			fmt.Println("couldn't load config")
		}
		data, err := os.ReadFile(appConfig.PKI.CACertificate)
		if err != nil {
			return err
		}

		cert, err := pki.ParseCertificatePEM(data)
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()

		fmt.Fprintf(out, "Subject: %s\n", cert.Subject)
		fmt.Fprintf(out, "Serial: %s\n", pki.SerialString(cert.SerialNumber))
		fmt.Fprintf(out, "Not before: %s\n", cert.NotBefore.UTC())
		fmt.Fprintf(out, "Not after: %s\n", cert.NotAfter.UTC())
		fmt.Fprintf(out, "Is CA: %t\n", cert.IsCA)

		return nil
	},
}

var createConnectorCmd = &cobra.Command{
	Use:   "add-connector",
	Short: "Sign and Add Connector to config",
	RunE: func(cmd *cobra.Command, args []string) error {
		var appConfig config.Config
		if err := viper.Unmarshal(&appConfig); err != nil {
			return fmt.Errorf("couldn't load config: %w", err)
		}

		if connectorCSRPath == "" {
			return fmt.Errorf("--csr is required")
		}
		if connectorDays <= 0 {
			return fmt.Errorf("--days must be positive")
		}

		// Get Connector ID from CSR
		// Open the file
		csrPEM, err := os.ReadFile(connectorCSRPath)
		if err != nil {
			return fmt.Errorf("read csr: %w", err)
		}

		// Parse the CSR
		csr, err := pki.ParseCSR(csrPEM)

		if err != nil {
			return fmt.Errorf("error with csr: %w", err)
		}

		connectorID := csr.Subject.CommonName

		//Check the ConnectorID
		if connectorID == "" {
			return fmt.Errorf("Connector ID Missing in CSR")
		}

		// 1. Don't duplicate
		for _, c := range appConfig.Connectors {
			if c.ID == connectorID {
				return fmt.Errorf("connector %q already exists", connectorID)
			}
		}

		// 2. Load CA - change this to whatever your loader is called
		//    in internal/.../pki, e.g. LoadCA / NewCA / Load
		ca, err := pki.LoadCA(
			appConfig.PKI.CACertificate,
			appConfig.PKI.CAPrivateKey,
		)
		if err != nil {
			return fmt.Errorf("load CA: %w", err)
		}

		certPEM, cert, err := ca.IssueCertificate(
			connectorID,
			csrPEM,
			time.Duration(connectorDays)*24*time.Hour,
		)

		if err != nil {
			return fmt.Errorf("issue certificate: %w", err)
		}

		// 4. Write cert to disk
		if connectorOutPath == "" {
			connectorOutPath = filepath.Join(connectorID + ".crt")
		}

		if err := os.MkdirAll(filepath.Dir(connectorOutPath), 0o755); err != nil {
			return fmt.Errorf("create cert dir: %w", err)
		}

		caPEM, err := os.ReadFile(appConfig.PKI.CACertificate)
		if err != nil {
			caPEM = pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: ca.Certificate.Raw,
			})
		}

		//Leaf first, CA second. Newline between them.
		bundle := certPEM

		if len(bundle) == 0 || bundle[len(bundle)-1] != '\n' {
			bundle = append(bundle, '\n')
		}

		bundle = append(bundle, caPEM...)

		if err := os.WriteFile(connectorOutPath, bundle, 0o644); err != nil {
			return fmt.Errorf("write certificate: %w", err)
		}

		// 5. Append to config
		appConfig.Connectors = append(appConfig.Connectors, config.Connector{
			ID:                connectorID,
			CertificateSerial: pki.SerialString(cert.SerialNumber),
		})

		// 6. Save YAML back to the same file Viper loaded
		//    Viper.WriteConfig() drops comments/formatting, so marshal
		//    the struct directly if you want to keep it clean.
		out, err := yaml.Marshal(&appConfig)
		if err != nil {
			return fmt.Errorf("marshal config: %w", err)
		}
		if err := os.WriteFile(viper.ConfigFileUsed(), out, 0o644); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		outW := cmd.OutOrStdout()

		fmt.Fprintf(outW, "Connector %q added\n", connectorID)
		fmt.Fprintf(outW, "Certificate: %s\n", connectorOutPath)
		fmt.Fprintf(outW, "Serial: %s\n", pki.SerialString(cert.SerialNumber))
		fmt.Fprintf(outW, "Not after: %s\n", cert.NotAfter.UTC().Format(time.RFC3339))

		return nil
	},
}
