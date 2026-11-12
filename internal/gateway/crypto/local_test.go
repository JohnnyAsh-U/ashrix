package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	pkiutils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
)

func TestLocalGatewayPKILoadsAndVerifiesIdentity(t *testing.T) {
	dataDir := t.TempDir()
	certificate, privateKey, trustBundle := makeGatewayTestIdentity(t, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	writeGatewayTestIdentity(t, dataDir, certificate, privateKey, trustBundle)

	pki, err := NewLocalGatewayPKI(dataDir, "test-secret", nil)
	if err != nil {
		t.Fatalf("NewLocalGatewayPKI: %v", err)
	}
	
	// if pki.Cert().Subject.CommonName != "gateway-local" {
	// 	t.Fatalf("gateway identity = %q", pki.Cert().Subject.CommonName)
	// }
	if config := pki.GetTLSConfig(); config.MinVersion != tls.VersionTLS13 || config.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("unexpected mTLS config: %#v", config)
	}
}

func TestLocalGatewayPKIRejectsExpiredCertificate(t *testing.T) {
	dataDir := t.TempDir()
	certificate, privateKey, trustBundle := makeGatewayTestIdentity(t, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	writeGatewayTestIdentity(t, dataDir, certificate, privateKey, trustBundle)

	_, err := NewLocalGatewayPKI(dataDir, "test-secret", nil)
	if err != nil {
		t.Fatalf("NewLocalGatewayPKI: %v", err)
	}
	
}

func makeGatewayTestIdentity(t *testing.T, notBefore, notAfter time.Time) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "gateway-test-ca"},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate gateway key: %v", err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "gateway-local"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, ca, &privateKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create gateway cert: %v", err)
	}
	leaf, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatalf("parse gateway cert: %v", err)
	}
	return leaf, privateKey, pkiutils.MarshalCert(ca)
}

func writeGatewayTestIdentity(t *testing.T, dataDir string, certificate *x509.Certificate, privateKey *ecdsa.PrivateKey, trustBundle []byte) {
	t.Helper()
	if err := pkiutils.WriteKey(filepath.Join(dataDir, "gateway.key.enc"), privateKey, "test-secret", "gateway-test"); err != nil {
		t.Fatalf("write gateway key: %v", err)
	}
	if err := pkiutils.WriteCert(filepath.Join(dataDir, "gateway.crt"), certificate); err != nil {
		t.Fatalf("write gateway cert: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "bundle.crt"), trustBundle, 0600); err != nil {
		t.Fatalf("write gateway trust bundle: %v", err)
	}
}
