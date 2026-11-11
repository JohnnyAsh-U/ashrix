package storage

import (
	"fmt"
	"os"
	"path/filepath"

	pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
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
