package startup

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

func TestNewLocalPKIInitialiserRejectsMissingCredentials(t *testing.T) {
	if _, err := NewLocalPKIInitialiser(nil, nil, "", "gateway.example.test"); err == nil {
		t.Fatal("expected missing credentials error")
	}
}

func TestNewLocalPKIInitialiserRejectsMissingServerName(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{Raw: []byte{1}}
	if _, err := NewLocalPKIInitialiser(cert, key, "", ""); err == nil {
		t.Fatal("expected missing server name error")
	}
}

func TestNewLocalPKIInitialiserBuildsTLS13ClientConfig(t *testing.T) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-ca"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, ca, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	clientCert, err := x509.ParseCertificate(clientDER)
	if err != nil {
		t.Fatal(err)
	}

	initialiser, err := NewLocalPKIInitialiser(clientCert, clientKey, string(ca.Raw), "gateway.example.test")
	if err != nil {
		t.Fatalf("NewLocalPKIInitialiser: %v", err)
	}
	cfg := initialiser.TLSConfig()
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion = %x, want TLS 1.3", cfg.MinVersion)
	}
	if cfg.ServerName != "gateway.example.test" {
		t.Fatalf("ServerName = %q", cfg.ServerName)
	}
	got, err := cfg.GetClientCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Certificate) != 1 || got.Leaf.SerialNumber.Cmp(clientCert.SerialNumber) != 0 {
		t.Fatal("client certificate was not configured")
	}
}

func TestNewLocalPKIInitialiserRejectsMalformedTrustBundle(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{Raw: []byte{1}}
	if _, err := NewLocalPKIInitialiser(cert, key, "not a DER certificate", "gateway.example.test"); err == nil {
		t.Fatal("expected malformed trust bundle error")
	}
}
