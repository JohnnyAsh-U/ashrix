package identity

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/pki"
)

func InstallCertificate(
	cfg config.Config,
	connectorID string,
	certificatePEM []byte,
) error {

	var certs []*x509.Certificate
	rest := certificatePEM

	for len(bytes.TrimSpace(rest)) > 0 {
		block, next := pem.Decode(rest)
		if block == nil {
			return errors.New("invalid data in certificate bundle")
		}
		rest = next
		if block.Type != "CERTIFICATE" {
			return fmt.Errorf("unexpected PEM block %q in certificate bundle", block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("parse certificate: %w", err)
		}
		certs = append(certs, cert)
	}

	if len(certs) != 2 {
		return fmt.Errorf("bundle must contain exactly a leaf and CA certificate, got %d cert(s)", len(certs))
	}

	leaf, caCert := certs[0], certs[1]

	// 2. Verify ID: leaf CN must == connector.id
	if leaf.Subject.CommonName != cfg.Connector.ID {
		return fmt.Errorf("leaf CN %q != connector.id %q", leaf.Subject.CommonName, cfg.Connector.ID)
	}

	// 3. Verify leaf was signed by this CA
	if err := leaf.CheckSignatureFrom(caCert); err != nil {
		return fmt.Errorf("leaf not signed by CA in bundle: %w", err)
	}

	// 4. Verify validity
	now := time.Now()
	if now.Before(leaf.NotBefore) || now.After(leaf.NotAfter) {
		return fmt.Errorf("leaf not valid now: %s - %s", leaf.NotBefore, leaf.NotAfter)
	}
	if !caCert.IsCA || caCert.KeyUsage&x509.KeyUsageCertSign == 0 {
		return errors.New("bundle issuer is not a signing CA")
	}

	if now.Before(caCert.NotBefore) || now.After(caCert.NotAfter) {
		return fmt.Errorf("CA is not currently valid: %s - %s", caCert.NotBefore, caCert.NotAfter)
	}

	key, err := LoadPrivateKey(cfg.Connector.PrivateKey)
	if err != nil {
		return err
	}

	// 6. Install: split bundle to config paths
	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw})
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})
	if err := verifyCertificate(leaf, caCert, key, connectorID, time.Now()); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Connector.Certificate), 0700); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Connector.CA), 0700); err != nil {
		return err
	}

	if err := atomicWrite(cfg.Connector.CA, caPEM, 0644); err != nil {
		return fmt.Errorf("install CA certificate: %w", err)
	}

	if err := atomicWrite(cfg.Connector.Certificate, leafPEM, 0644); err != nil {
		return fmt.Errorf("install connector certificate: %w", err)
	}

	return nil
}

func verifyCertificate(
	cert *x509.Certificate,
	ca *x509.Certificate,
	key *ecdsa.PrivateKey,
	connectorID string,
	now time.Time,
) error {
	if cert.IsCA {
		return errors.New("connector certificate must not be a CA")
	}

	if cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return errors.New("certificate lacks digital signature usage")
	}

	roots := x509.NewCertPool()
	roots.AddCert(ca)

	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: now,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		return fmt.Errorf("verify certificate chain: %w", err)
	}

	if len(cert.URIs) != 1 || cert.URIs[0].String() != pki.ConnectorIdentityURI(connectorID) {
		return errors.New("certificate URI identity does not match connector ID")
	}

	pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("certificate public key must be ECDSA")
	}

	if pub.X.Cmp(key.PublicKey.X) != 0 ||
		pub.Y.Cmp(key.PublicKey.Y) != 0 {
		return errors.New("certificate does not match local private key")
	}

	return nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".ashrix-*.tmp")
	if err != nil {
		return err
	}

	tmpPath := tmp.Name()
	committed := false

	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

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

	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	committed = true

	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()

	return d.Sync()
}
