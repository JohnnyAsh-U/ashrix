package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"

	"net/url"
	"time"
)

func (ca *CA) IssueConnectorCertificate(
	connectorID string,
	csrPEM []byte,
	validity time.Duration,
) ([]byte, *x509.Certificate, error) {
	if ca == nil || ca.Certificate == nil || ca.PrivateKey == nil {
		return nil, nil, fmt.Errorf("CA is not initialized")
	}
	if connectorID == "" {
		return nil, nil, fmt.Errorf("connector ID is required")
	}
	if validity <= 0 {
		return nil, nil, fmt.Errorf("certificate validity must be positive")
	}

	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, nil, fmt.Errorf("invalid CSR PEM")
	}

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CSR: %w", err)
	}

	if err := csr.CheckSignature(); err != nil {
		return nil, nil, fmt.Errorf("invalid CSR signature: %w", err)
	}

	pub, ok := csr.PublicKey.(*ecdsa.PublicKey)
	if !ok || pub.Curve != elliptic.P256() {
		return nil, nil, fmt.Errorf("CSR must use ECDSA P-256")
	}

	// Do not copy the CSR subject, SANs, or extensions.
	// The gateway defines the connector identity.
	identity, err := url.Parse(
		"spiffe://ashrix.example/connectors/" + connectorID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("construct connector identity: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	notAfter := now.Add(validity)
	if notAfter.After(ca.Certificate.NotAfter) {
		notAfter = ca.Certificate.NotAfter
	}
	if !notAfter.After(now) {
		return nil, nil, fmt.Errorf("CA is expired or near expiration")
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkixName(connectorID),
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
		URIs: []*url.URL{identity},
	}

	der, err := x509.CreateCertificate(
		rand.Reader,
		template,
		ca.Certificate,
		pub,
		ca.PrivateKey,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("sign connector certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("parse issued certificate: %w", err)
	}

	result := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	})

	return result, cert, nil
}

func pkixName(connectorID string) pkix.Name {
	return pkix.Name{CommonName: connectorID}
}
