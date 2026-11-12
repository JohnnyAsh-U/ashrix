
package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeCSR(t *testing.T) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	der, err := x509.CreateCertificateRequest(
		rand.Reader,
		&x509.CertificateRequest{
			Subject: pkix.Name{CommonName: "attacker-controlled-name"},
		},
		key,
	)
	if err != nil {
		t.Fatal(err)
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: der,
	}), key
}

func TestGenerateAndLoadCA(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")

	ca, err := GenerateCA("Test CA")
	if err != nil {
		t.Fatal(err)
	}

	if err := SaveCA(ca, certPath, keyPath); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadCA(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}

	if !loaded.Certificate.IsCA {
		t.Fatal("expected CA certificate")
	}

	if err := SaveCA(ca, certPath, keyPath); err == nil {
		t.Fatal("expected overwrite to fail")
	}
}

func TestLoadCARejectsMissingKey(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")

	ca, err := GenerateCA("Test CA")
	if err != nil {
		t.Fatal(err)
	}

	if err := SaveCA(ca, certPath, keyPath); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(keyPath); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadCA(certPath, keyPath); err == nil {
		t.Fatal("expected missing key error")
	}
}

func TestIssueAndVerifyConnectorCertificate(t *testing.T) {
	ca, err := GenerateCA("Test CA")
	if err != nil {
		t.Fatal(err)
	}

	csr, _ := makeCSR(t)

	certPEM, cert, err := ca.IssueCertificate(
		"office-01",
		csr,
		30*24*time.Hour,
	)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseCertificatePEM(certPEM)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.SerialNumber.Cmp(cert.SerialNumber) != 0 {
		t.Fatal("serial mismatch")
	}

	err = VerifyConnectorCertificate(
		cert,
		ca.Certificate,
		VerifyOptions{
			ConnectorID:       "office-01",
			ExpectedSerial:    SerialString(cert.SerialNumber),
		},
	)
	if err != nil {
		t.Fatalf("verify certificate: %v", err)
	}

	if cert.Subject.CommonName != "office-01" {
		t.Fatalf("unexpected common name: %s", cert.Subject.CommonName)
	}
}

func TestCSRIdentityIsNotTrusted(t *testing.T) {
	ca, err := GenerateCA("Test CA")
	if err != nil {
		t.Fatal(err)
	}

	csr, _ := makeCSR(t)

	_, cert, err := ca.IssueCertificate(
		"office-01",
		csr,
		30*24*time.Hour,
	)
	if err != nil {
		t.Fatal(err)
	}

	if cert.Subject.CommonName != "office-01" {
		t.Fatal("gateway did not construct connector identity")
	}

	if len(cert.URIs) != 1 {
		t.Fatal("expected exactly one URI SAN")
	}

	if cert.URIs[0].String() !=
		"spiffe://ashrix.example/connectors/office-01" {
		t.Fatalf("unexpected URI SAN: %s", cert.URIs[0])
	}
}

func TestRejectRSAKeyCSR(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	der, err := x509.CreateCertificateRequest(
		rand.Reader,
		&x509.CertificateRequest{},
		key,
	)
	if err != nil {
		t.Fatal(err)
	}

	csr := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: der,
	})

	if _, err := ParseCSR(csr); err == nil {
		t.Fatal("expected RSA CSR rejection")
	}
}

func TestRejectInvalidConnectorID(t *testing.T) {
	ca, err := GenerateCA("Test CA")
	if err != nil {
		t.Fatal(err)
	}

	csr, _ := makeCSR(t)

	if _, _, err := ca.IssueCertificate(
		"../other",
		csr,
		30*24*time.Hour,
	); err == nil {
		t.Fatal("expected invalid connector ID rejection")
	}
}

func TestRejectWrongConnectorIdentity(t *testing.T) {
	ca, err := GenerateCA("Test CA")
	if err != nil {
		t.Fatal(err)
	}

	csr, _ := makeCSR(t)

	_, cert, err := ca.IssueCertificate(
		"office-01",
		csr,
		30*24*time.Hour,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = VerifyConnectorCertificate(
		cert,
		ca.Certificate,
		VerifyOptions{
			ConnectorID:       "office-02",
		},
	)
	if err == nil {
		t.Fatal("expected connector identity mismatch")
	}
}

func TestRejectWrongSerial(t *testing.T) {
	ca, err := GenerateCA("Test CA")
	if err != nil {
		t.Fatal(err)
	}

	csr, _ := makeCSR(t)

	_, cert, err := ca.IssueCertificate(
		"office-01",
		csr,
		30*24*time.Hour,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = VerifyConnectorCertificate(
		cert,
		ca.Certificate,
		VerifyOptions{
			ConnectorID:       "office-01",
			ExpectedSerial:    "123456",
		},
	)
	if err == nil {
		t.Fatal("expected serial mismatch")
	}
}

func TestRejectExpiredCA(t *testing.T) {
	ca, err := GenerateCA("Expired CA")
	if err != nil {
		t.Fatal(err)
	}

	ca.Certificate.NotAfter = time.Now().Add(-time.Hour)

	csr, _ := makeCSR(t)

	if _, _, err := ca.IssueCertificate(
		"office-01",
		csr,
		time.Hour,
	); err == nil {
		t.Fatal("expected expired CA rejection")
	}
}

func TestRejectExcessiveValidity(t *testing.T) {
	ca, err := GenerateCA("Test CA")
	if err != nil {
		t.Fatal(err)
	}

	csr, _ := makeCSR(t)

	if _, _, err := ca.IssueCertificate(
		"office-01",
		csr,
		31*24*time.Hour,
	); err == nil {
		t.Fatal("expected excessive validity rejection")
	}
}
