package startup

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
)

var (
	secret  = "CONNECTOR"
	contextConn = "CONNECTOR"
)

type CredentialPaths struct {
	CertPath   string
	KeyPath    string
	BundlePath string
}

func DefaultCredPaths(dataDir string) CredentialPaths {
	return CredentialPaths{
		CertPath:   filepath.Join(dataDir, "connector.crt"),
		KeyPath:    filepath.Join(dataDir, "connector.key.enc"),
		BundlePath: filepath.Join(dataDir, "bundle.crt"),
	}
}

func (p CredentialPaths) Exist() bool {
	_, certErr := os.Stat(p.CertPath)
	_, keyErr := os.Stat(p.KeyPath)
	_, bundleErr := os.Stat(p.BundlePath)
	return certErr == nil && keyErr == nil && bundleErr == nil
}

func (p CredentialPaths) Load() (*x509.Certificate, *ecdsa.PrivateKey, *tls.Certificate, error) {
	// Load Identity
	key, cert, err := pki_utils.LoadKeyAndCert(p.KeyPath, p.CertPath, secret, contextConn)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load gateway identity: %w", err)
	}

	// Load Trust Bundle
	trustPEM, err := os.ReadFile(p.BundlePath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read trust bundle: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(trustPEM) {
		return nil, nil, nil, fmt.Errorf("failed to parse trust bundle")
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  key,
		Leaf:        cert,
	}

	return cert, key, tlsCert, nil
}

func (p CredentialPaths) Save(cert *x509.Certificate, key *ecdsa.PrivateKey, bundlePEM []byte) error {
	if err := pki_utils.WriteCert(p.CertPath, cert); err != nil {
		return fmt.Errorf("Error Creating Cert: %w", err)
	}
	if err := pki_utils.WriteKey(p.KeyPath, key, secret, contextConn); err != nil {
		return fmt.Errorf("Error creating Key %w", err)
	}
	if err := os.WriteFile(p.BundlePath, bundlePEM, 0600); err != nil {
		return fmt.Errorf("Error Write Bundle %w", err)
	}
	return nil
}

func (p CredentialPaths) Delete() {
	os.Remove(p.CertPath)
	os.Remove(p.KeyPath)
	os.Remove(p.BundlePath)
}
