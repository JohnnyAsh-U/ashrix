package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"regexp"
	"strings"

	"net/url"
	"time"
)

var connectorIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`)

const MaxConnectorCertificateValidity = 90 * 24 * time.Hour
const MaxGatewayCertificateValidity = 365 * 24 * time.Hour

// ConnectorIdentityURI returns the canonical identity URI embedded in a
// connector certificate. It is deliberately derived from the gateway-approved ID.
func ConnectorIdentityURI(connectorID string) string {
	return "spiffe://ashrix/connectors/" + connectorID
}

func (ca *CA) IssueCertificate(
	ID string,
	csrPEM []byte,
	validity time.Duration,
) ([]byte, *x509.Certificate, error) {
	if ca == nil || ca.Certificate == nil || ca.PrivateKey == nil {
		return nil, nil, fmt.Errorf("CA is not initialized")
	}
	if !connectorIDPattern.MatchString(ID) {
		return nil, nil, fmt.Errorf("invalid connector ID %q", ID)
	}

	if validity > MaxConnectorCertificateValidity {
		return nil, nil, fmt.Errorf("connector certificate validity cannot exceed %s", MaxConnectorCertificateValidity)
	}
	if !ca.Certificate.IsCA || ca.Certificate.KeyUsage&x509.KeyUsageCertSign == 0 {
		return nil, nil, fmt.Errorf("configured certificate is not a signing CA")
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
	identity, err := url.Parse(ConnectorIdentityURI(ID))
	if err != nil {
		return nil, nil, fmt.Errorf("construct connector identity: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	if now.Before(ca.Certificate.NotBefore) || now.After(ca.Certificate.NotAfter) {
		return nil, nil, fmt.Errorf("CA certificate is not currently valid")
	}
	notAfter := now.Add(validity)
	if notAfter.After(ca.Certificate.NotAfter) {
		notAfter = ca.Certificate.NotAfter
	}
	if !notAfter.After(now) {
		return nil, nil, fmt.Errorf("CA is expired or near expiration")
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkixName(ID),
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
		// DNSNames: []string{"localhost"},
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

// IssueGatewayCertificate issues a server-only certificate for the gateway's
// configured public DNS name or IP address.
func (ca *CA) IssueGatewayCertificate(ID, serverName string, csrPEM []byte, validity time.Duration) ([]byte, *x509.Certificate, error) {
	if ca == nil || ca.Certificate == nil || ca.PrivateKey == nil {
		return nil, nil, fmt.Errorf("CA is not initialized")
	}
	if !connectorIDPattern.MatchString(ID) {
		return nil, nil, fmt.Errorf("invalid gateway ID %q", ID)
	}
	if !validCertificateServerName(serverName) {
		return nil, nil, fmt.Errorf("invalid gateway server name %q", serverName)
	}
	if validity <= 0 || validity > MaxGatewayCertificateValidity {
		return nil, nil, fmt.Errorf("gateway certificate validity must be between 0 and %s", MaxGatewayCertificateValidity)
	}
	if !ca.Certificate.IsCA || ca.Certificate.KeyUsage&x509.KeyUsageCertSign == 0 {
		return nil, nil, fmt.Errorf("configured certificate is not a signing CA")
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
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	if now.Before(ca.Certificate.NotBefore) || now.After(ca.Certificate.NotAfter) {
		return nil, nil, fmt.Errorf("CA certificate is not currently valid")
	}
	notAfter := now.Add(validity)
	if notAfter.After(ca.Certificate.NotAfter) {
		notAfter = ca.Certificate.NotAfter
	}
	if !notAfter.After(now) {
		return nil, nil, fmt.Errorf("CA is expired or near expiration")
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: ID},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(serverName); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{serverName}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.Certificate, pub, ca.PrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("sign gateway certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("parse issued gateway certificate: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), cert, nil
}

func pkixName(ID string) pkix.Name {
	return pkix.Name{CommonName: ID}
}

func validCertificateServerName(name string) bool {
	if name == "" || strings.TrimSpace(name) != name {
		return false
	}
	if net.ParseIP(name) != nil {
		return true
	}
	if len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
				return false
			}
		}
	}
	return true
}
