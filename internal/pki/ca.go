package pki


import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type CA struct {
	Certificate *x509.Certificate
	PrivateKey  *ecdsa.PrivateKey
}

func GenerateCA(commonName string, validity time.Duration) (*CA, error) {
	if commonName == "" {
		return nil, fmt.Errorf("CA common name is required")
	}
	if validity <= 0 {
		return nil, fmt.Errorf("CA validity must be positive")
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
		NotAfter:              now.Add(validity),
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

	if err := os.MkdirAll(filepath.Dir(certPath), 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.Certificate.Raw,
	})

	keyDER, err := x509.MarshalPKCS8PrivateKey(ca.PrivateKey)
	if err != nil {
		return fmt.Errorf("marshal CA private key: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyDER,
	})

	// O_EXCL prevents accidental replacement of existing CA material.
	if err := writeExclusive(certPath, certPEM, 0644); err != nil {
		return fmt.Errorf("save CA certificate: %w", err)
	}

	if err := writeExclusive(keyPath, keyPEM, 0600); err != nil {
		// Do not silently leave a half-created CA.
		_ = os.Remove(certPath)
		return fmt.Errorf("save CA private key: %w", err)
	}

	return nil
}

func writeExclusive(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}

	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()

	if writeErr != nil {
		_ = os.Remove(path)
		return writeErr
	}
	if syncErr != nil {
		_ = os.Remove(path)
		return syncErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return closeErr
	}

	return nil
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
