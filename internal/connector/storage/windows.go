package storage

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/spf13/viper"
)

type WindowsStorage struct {
	certDir    string
	logDir     string
	certPath   string
	keyPath    string
	bundlePath string
}

func NewWindowsStorage() (*WindowsStorage, error) {
	certdir := viper.GetString("certdir")
	logDir := viper.GetString("logdir")

	CertPath := filepath.Join(certdir, "connector.crt")
	KeyPath := filepath.Join(certdir, "connector.key.enc")
	BundlePath := filepath.Join(certdir, "bundle.crt")

	return &WindowsStorage{
		certDir:    certdir,
		logDir:     logDir,
		certPath:   CertPath,
		keyPath:    KeyPath,
		bundlePath: BundlePath,
	}, nil
}

func (s *WindowsStorage) LoadCredential() (*Cred, error) {
	key, cert, err := pki_utils.LoadKeyAndCert(s.keyPath, s.certPath, secret, context)
	if err != nil {
		return nil, fmt.Errorf("failed to load gateway identity: %w", err)
	}
	// Load Trust Bundle
	trustPEM, err := os.ReadFile(s.bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read trust bundle: %w", err)
	}

	return &Cred{
		PrivKey: key,
		Cert:    cert,
		Bundle:  string(trustPEM),
	}, nil
}

func (s *WindowsStorage) CredentialExists() bool {
	_, certErr := os.Stat(s.certPath)
	_, keyErr := os.Stat(s.keyPath)
	_, bundleErr := os.Stat(s.bundlePath)
	return certErr == nil && keyErr == nil && bundleErr == nil
}

func (s *WindowsStorage) ClearCredential() error {
	os.Remove(s.certPath)
	os.Remove(s.keyPath)
	os.Remove(s.bundlePath)
	return nil
}

func (s *WindowsStorage) SaveCredential(regResult *gen.ConnectorEnrollResponse, newKey *ecdsa.PrivateKey) error {
	// ── Save renewed cert + key ────────────────────────────────
	//Key
	if err := pki_utils.WriteKey(s.keyPath, newKey, secret, context); err != nil {
		return err
	}

	//Cert
	newCert := regResult.Certificate
	block, _ := pem.Decode([]byte(newCert))
	if block == nil {
		return fmt.Errorf("Failed to decode PEM Certificate")
	}
	if block.Type != "CERTIFICATE" {
		return fmt.Errorf("Expected Certificate pem block got %q", block.Type)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	if err := pki_utils.WriteCert(s.certPath, cert); err != nil {
		return fmt.Errorf("save renewed cert credentials: %w", err)
	}

	//Bundle
	newBundle := regResult.TrustBundle
	block, _ = pem.Decode([]byte(newBundle))
	if block == nil {
		return fmt.Errorf("Failed to decode PEM Certificate")
	}
	if block.Type != "CERTIFICATE" {
		return fmt.Errorf("Expected Certificate pem block got %q", block.Type)
	}

	cert, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	if err := pki_utils.WriteCert(s.bundlePath, cert); err != nil {
		return fmt.Errorf("save renewed bundle credentials: %w", err)
	}
	return nil
}
