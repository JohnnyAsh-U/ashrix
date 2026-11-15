package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/pki"
	"github.com/JohnnyAsh-U/ashrix-api/internal/transport"
)

func gatewayFixture(t *testing.T, serverName string, eku []x509.ExtKeyUsage, notBefore, notAfter time.Time) (*pki.CA, *ecdsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	ca, err := pki.GenerateCA("Ashrix test CA")
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(42),
		Subject:               pkix.Name{CommonName: "gateway-01"},
		DNSNames:              []string{serverName},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           eku,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.Certificate, &key.PublicKey, ca.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return ca, key, cert
}

func writeGatewayFixture(t *testing.T, dir string, cert *x509.Certificate, key *ecdsa.PrivateKey, certSuffix, keySuffix string) (string, string) {
	t.Helper()
	certPath := filepath.Join(dir, certSuffix)
	keyPath := filepath.Join(dir, keySuffix)
	certData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyData := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(certPath, certData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyData, 0600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

func TestNewLocalGatewayPKIAcceptsValidServerCertificate(t *testing.T) {
	now := time.Now()
	ca, key, cert := gatewayFixture(t, "gateway.example.test", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, now.Add(-time.Minute), now.Add(time.Hour))
	certPath, keyPath := writeGatewayFixture(t, t.TempDir(), cert, key, "gateway.crt", "gateway.key")

	gateway, err := NewLocalGatewayPKI(certPath, keyPath, ca.Certificate, "gateway.example.test")
	if err != nil {
		t.Fatalf("NewLocalGatewayPKI: %v", err)
	}
	if gateway.Cert().SerialNumber.Cmp(cert.SerialNumber) != 0 {
		t.Fatal("loaded gateway certificate differs from fixture")
	}
	cfg := gateway.GetTLSConfig()
	if cfg.MinVersion != tls.VersionTLS13 || cfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatal("gateway TLS config must require verified client certificates and TLS 1.3")
	}
	if len(cfg.NextProtos) != 1 || cfg.NextProtos[0] != transport.ALPN {
		t.Fatalf("unexpected ALPN configuration: %v", cfg.NextProtos)
	}
}

func TestNewLocalGatewayPKIRejectsMismatchedPrivateKey(t *testing.T) {
	now := time.Now()
	ca, _, cert := gatewayFixture(t, "gateway.example.test", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, now.Add(-time.Minute), now.Add(time.Hour))
	otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := writeGatewayFixture(t, t.TempDir(), cert, otherKey, "gateway.crt", "gateway.key")
	if _, err := NewLocalGatewayPKI(certPath, keyPath, ca.Certificate, "gateway.example.test"); err == nil {
		t.Fatal("expected mismatched key to be rejected")
	}
}

func TestNewLocalGatewayPKIRejectsWrongServerName(t *testing.T) {
	now := time.Now()
	ca, key, cert := gatewayFixture(t, "other.example.test", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, now.Add(-time.Minute), now.Add(time.Hour))
	certPath, keyPath := writeGatewayFixture(t, t.TempDir(), cert, key, "gateway.crt", "gateway.key")
	if _, err := NewLocalGatewayPKI(certPath, keyPath, ca.Certificate, "gateway.example.test"); err == nil {
		t.Fatal("expected server name mismatch to be rejected")
	}
}

func TestNewLocalGatewayPKIRejectsClientOnlyCertificate(t *testing.T) {
	now := time.Now()
	ca, key, cert := gatewayFixture(t, "gateway.example.test", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, now.Add(-time.Minute), now.Add(time.Hour))
	certPath, keyPath := writeGatewayFixture(t, t.TempDir(), cert, key, "gateway.crt", "gateway.key")
	if _, err := NewLocalGatewayPKI(certPath, keyPath, ca.Certificate, "gateway.example.test"); err == nil {
		t.Fatal("expected client-only certificate to be rejected")
	}
}

func TestNewLocalGatewayPKIRejectsCertificateWithoutExplicitServerUsage(t *testing.T) {
	now := time.Now()
	ca, key, cert := gatewayFixture(t, "gateway.example.test", nil, now.Add(-time.Minute), now.Add(time.Hour))
	certPath, keyPath := writeGatewayFixture(t, t.TempDir(), cert, key, "gateway.crt", "gateway.key")
	if _, err := NewLocalGatewayPKI(certPath, keyPath, ca.Certificate, "gateway.example.test"); err == nil {
		t.Fatal("expected certificate without explicit server usage to be rejected")
	}
}

func TestNewLocalGatewayPKIRejectsExpiredCertificate(t *testing.T) {
	now := time.Now()
	ca, key, cert := gatewayFixture(t, "gateway.example.test", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, now.Add(-2*time.Hour), now.Add(-time.Hour))
	certPath, keyPath := writeGatewayFixture(t, t.TempDir(), cert, key, "gateway.crt", "gateway.key")
	if _, err := NewLocalGatewayPKI(certPath, keyPath, ca.Certificate, "gateway.example.test"); err == nil {
		t.Fatal("expected expired certificate to be rejected")
	}
}

func TestNewLocalGatewayPKIRejectsTrailingPEMData(t *testing.T) {
	now := time.Now()
	ca, key, cert := gatewayFixture(t, "gateway.example.test", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, now.Add(-time.Minute), now.Add(time.Hour))
	certPath, keyPath := writeGatewayFixture(t, t.TempDir(), cert, key, "gateway.crt", "gateway.key")
	data, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, append(data, []byte("\nunexpected-data\n")...), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalGatewayPKI(certPath, keyPath, ca.Certificate, "gateway.example.test"); err == nil {
		t.Fatal("expected trailing certificate data to be rejected")
	}
}

func TestNewLocalGatewayPKIRejectsNonCAIssuer(t *testing.T) {
	now := time.Now()
	_, key, cert := gatewayFixture(t, "gateway.example.test", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, now.Add(-time.Minute), now.Add(time.Hour))
	certPath, keyPath := writeGatewayFixture(t, t.TempDir(), cert, key, "gateway.crt", "gateway.key")
	if _, err := NewLocalGatewayPKI(certPath, keyPath, cert, "gateway.example.test"); err == nil {
		t.Fatal("expected non-CA issuer to be rejected")
	}
}

func TestSaveGatewayCertAndKeyWritesExpectedPermissions(t *testing.T) {
	now := time.Now()
	_, key, cert := gatewayFixture(t, "gateway.example.test", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, now.Add(-time.Minute), now.Add(time.Hour))
	dir := t.TempDir()
	certPath := filepath.Join(dir, "gateway.crt")
	keyPath := filepath.Join(dir, "gateway.key")
	if err := SaveGatwayCertAndKey(certPath, keyPath, cert, key); err != nil {
		t.Fatal(err)
	}
	certInfo, err := os.Stat(certPath)
	if err != nil {
		t.Fatal(err)
	}
	keyInfo, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := certInfo.Mode().Perm(); got != 0644 {
		t.Fatalf("certificate mode = %04o, want 0644", got)
	}
	if got := keyInfo.Mode().Perm(); got != 0600 {
		t.Fatalf("private key mode = %04o, want 0600", got)
	}
}

func TestSaveGatewayCertAndKeyRejectsMismatchedKey(t *testing.T) {
	now := time.Now()
	_, _, cert := gatewayFixture(t, "gateway.example.test", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, now.Add(-time.Minute), now.Add(time.Hour))
	otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath := filepath.Join(dir, "gateway.crt")
	keyPath := filepath.Join(dir, "gateway.key")
	if err := SaveGatwayCertAndKey(certPath, keyPath, cert, otherKey); err == nil {
		t.Fatal("expected mismatched key to be rejected")
	}
	if _, err := os.Stat(certPath); !os.IsNotExist(err) {
		t.Fatalf("certificate should not be written after validation failure, stat error = %v", err)
	}
}
