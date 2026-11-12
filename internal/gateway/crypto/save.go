package crypto

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

func SaveGatwayCertAndKey(certPath, keyPath string, cert *x509.Certificate, key *ecdsa.PrivateKey) error {
	if cert == nil || key== nil {
		return fmt.Errorf("incomplete CA")
	}

	//Key
	dir := filepath.Dir(certPath)
	certdata := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	certmode := os.FileMode(0644)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
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
	keyDER, err := x509.MarshalECPrivateKey(key)
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