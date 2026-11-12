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
		return nil, fmt.Errorf("Failed to load key : %w", err)
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
		return nil, fmt.Errorf("ca certificate is not valid after %s", caCert.NotBefore)

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

	//Key
	dir := filepath.Dir(certPath)
	certdata := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Certificate.Raw})
	certmode := os.FileMode(0644)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		fmt.Println("HHHHH", dir)
		return fmt.Errorf("temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(certdata); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing cert: %w", err)
	}
	if err := tmp.Chmod(certmode); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close: %w", err)
	}
	if err := os.Rename(tmpPath, certPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename to %s: %w", certPath, err)
	}

	//Key
	dir = filepath.Dir(keyPath)
	keyDER, err := x509.MarshalECPrivateKey(ca.PrivateKey)
	keydata := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keymode := os.FileMode(0600)
	tmp, err = os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpPath = tmp.Name()

	if _, err := tmp.Write(keydata); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing key: %w", err)
	}
	if err := tmp.Chmod(keymode); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close: %w", err)
	}
	if err := os.Rename(tmpPath, keyPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename to %s: %w", keyPath, err)
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
