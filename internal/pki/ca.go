package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	// "path/filepath"
	"time"
)

type CA struct {
	Certificate *x509.Certificate
	PrivateKey  *ecdsa.PrivateKey
}

func LoadCA(certPath, keyPath string) (*CA, error) {
	keyPem, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", keyPath, err)
	}
	block, _ := pem.Decode(keyPem)
	if block == nil {
		return nil, errors.New("failed to load key PEM")
	}
	caKey, err := x509.ParseECPrivateKey(block.Bytes)

	if err != nil {
		return nil, fmt.Errorf("Failed to load cert : %w", err)
	}

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", certPath, err)
	}
	block, _ = pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("invalid cert PEM: %s", keyPath)
	}
	caCert, err := x509.ParseCertificate(block.Bytes)

	if err != nil {
		return nil, fmt.Errorf("parse CA certificate: %w", err)
	}

	if !caCert.IsCA || caCert.KeyUsage&x509.KeyUsageCertSign == 0 {
		return nil, errors.New("configured certificate is not a signing CA")
	}

	// Verify the key and cert
	//Verify public key
	certPub, ok := caCert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("Public Key Invalid")
	}

	//Verify if key and public matches
	if !certPub.Equal(&caKey.PublicKey) {
		return nil, fmt.Errorf("Public Key and Cert Doesn't match")
	}

	//Verify the validity
	now := time.Now()
	if now.Before(caCert.NotBefore) {
		return nil, fmt.Errorf("ca certificate is not valid before %s", caCert.NotBefore)
	}
	if now.After(caCert.NotAfter) {
		return nil, fmt.Errorf("ca certificate expired at %s", caCert.NotAfter)
	}

	return &CA{
		PrivateKey:  caKey,
		Certificate: caCert,
	}, nil
}

func GenerateCA(commonName string) (*CA, error) {
	if commonName == "" {
		return nil, fmt.Errorf("CA common name is required")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	now := time.Now()

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(365 * 24 * time.Hour), //1 year
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign |
			x509.KeyUsageDigitalSignature,
	}

	der, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		&key.PublicKey,
		key,
	)
	if err != nil {
		return nil, fmt.Errorf("create CA certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse generated CA: %w", err)
	}

	return &CA{
		Certificate: cert,
		PrivateKey:  key,
	}, nil
}

func SaveCA(ca *CA, certPath, keyPath string) error {
	if ca == nil || ca.Certificate == nil || ca.PrivateKey == nil {
		return fmt.Errorf("incomplete CA")
	}

	if certPath == keyPath {
		return fmt.Errorf("CA certificate and private key paths must differ")
	}

	//Key
	certData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Certificate.Raw})
	keyDER, err := x509.MarshalECPrivateKey(ca.PrivateKey)
	if err != nil {
		return fmt.Errorf("marshal CA private key: %w", err)
	}
	// tmpPath := tmp.Name()

	keyData := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err := writeExclusive(certPath, certData, 0o644); err != nil {
		return fmt.Errorf("save CA certificate: %w", err)
	}

	if err := writeExclusive(keyPath, keyData, 0o600); err != nil {
		_ = os.Remove(certPath) // roll back the first file if the pair cannot be completed
		return fmt.Errorf("save CA private key: %w", err)
	}
	return nil
}

// writeExclusive writes through a temporary file in the destination directory,
// then links it into place. Unlike Rename, Link never replaces an existing key.
func writeExclusive(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".ashrix-ca-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Link(tmpPath, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		_ = os.Remove(path)
		return err
	}
	defer d.Close()
	return d.Sync()
}

func randomSerial() (*big.Int, error) {
	// X.509 serial numbers should be positive and no longer than 20 octets.
	limit := new(big.Int).Lsh(big.NewInt(1), 159)

	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial: %w", err)
	}

	if serial.Sign() == 0 {
		return nil, fmt.Errorf("generated zero certificate serial")
	}

	return serial, nil
}
