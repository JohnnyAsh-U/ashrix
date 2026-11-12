
package identity

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

func GenerateCSR(
	key *ecdsa.PrivateKey,
	connectorID string,
) ([]byte, error) {
	if key == nil {
		return nil, errors.New("private key is required")
	}
	if connectorID == "" {
		return nil, errors.New("connector ID is required")
	}

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: connectorID,
		},
	}

	der, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		return nil, fmt.Errorf("create CSR: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: der,
	}), nil
}

func CreateKeyAndCSR(
	keyPath string,
	csrPath string,
	connectorID string,
	outPath string,
) error {
	if keyPath == "" || csrPath == "" {
		return errors.New("key and CSR paths are required")
	}
	if keyPath == csrPath {
		return errors.New("key and CSR paths must differ")
	}

	key, err := GeneratePrivateKey()
	if err != nil {
		return err
	}

	csr, err := GenerateCSR(key, connectorID)
	if err != nil {
		return err
	}

	if err := SavePrivateKey(keyPath, key); err != nil {
		return fmt.Errorf("save private key: %w", err)
	}

	if err := os.WriteFile(outPath, csr, 0600); err != nil {
		_ = os.Remove(keyPath)
		return fmt.Errorf("save CSR: %w", err)
	}

	return nil
}
