package identity

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/pki"
)

func TestGeneratePrivateKeyUsesP256(t *testing.T) {
	key, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	if key.Curve != elliptic.P256() {
		t.Fatalf("expected P-256 key, got %s", key.Curve.Params().Name)
	}
}

func TestSaveAndLoadPrivateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connector.key")
	key, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := SavePrivateKey(path, key); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("private key permissions = %04o, want 0600", got)
	}

	loaded, err := LoadPrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.PublicKey.Equal(&key.PublicKey) {
		t.Fatal("loaded key does not match saved key")
	}
}

func TestSavePrivateKeyRejectsNil(t *testing.T) {
	if err := SavePrivateKey(filepath.Join(t.TempDir(), "key"), nil); err == nil {
		t.Fatal("expected nil key to be rejected")
	}
}

func TestLoadPrivateKeyRejectsTrailingData(t *testing.T) {
	key, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "key")
	if err := SavePrivateKey(path, key); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("unexpected")...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPrivateKey(path); err == nil {
		t.Fatal("expected trailing data to be rejected")
	}
}

func TestGenerateCSRContainsConnectorIdentity(t *testing.T) {
	key, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	csrPEM, err := GenerateCSR(key, "edge-01")
	if err != nil {
		t.Fatal(err)
	}
	block, rest := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" || len(strings.TrimSpace(string(rest))) != 0 {
		t.Fatal("expected one valid CSR PEM block")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := csr.CheckSignature(); err != nil {
		t.Fatalf("CSR signature: %v", err)
	}
	if csr.Subject.CommonName != "edge-01" {
		t.Fatalf("CSR common name = %q, want edge-01", csr.Subject.CommonName)
	}
}

func TestGenerateCSRRejectsMissingInputs(t *testing.T) {
	key, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateCSR(nil, "edge-01"); err == nil {
		t.Fatal("expected nil key error")
	}
	if _, err := GenerateCSR(key, ""); err == nil {
		t.Fatal("expected empty connector ID error")
	}
}

func TestCreateKeyAndCSRWritesFiles(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "connector.key")
	csrPath := filepath.Join(dir, "connector.csr")
	if err := CreateKeyAndCSR(keyPath, "edge-01", csrPath); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPrivateKey(keyPath); err != nil {
		t.Fatalf("load generated private key: %v", err)
	}

	csrData, err := os.ReadFile(csrPath)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(csrData)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		t.Fatal("expected generated CSR PEM")
	}
	info, err := os.Stat(csrPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("CSR permissions = %04o, want 0600", got)
	}
}

func TestCreateKeyAndCSRRollsBackKeyWhenCSRWriteFails(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "connector.key")
	// certificatePath := filepath.Join(dir, "connector.crt")
	csrPath := filepath.Join(dir, "existing-directory")
	if err := os.Mkdir(csrPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := CreateKeyAndCSR(keyPath, "edge-01", csrPath); err == nil {
		t.Fatal("expected writing CSR over a directory to fail")
	}
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Fatalf("private key should be rolled back after CSR write failure, stat err = %v", err)
	}
}

func TestCreateKeyAndCSRRejectsCollidingPaths(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "connector.key")
	certificatePath := filepath.Join(dir, "connector.crt")
	csrPath := filepath.Join(dir, "connector.csr")
	for _, paths := range [][2]string{
		{keyPath, keyPath},
		{csrPath, csrPath},
		{certificatePath, certificatePath},
	} {
		if err := CreateKeyAndCSR(paths[0], "edge-01", paths[1]); err == nil {
			t.Fatalf("expected colliding paths to be rejected: %v", paths)
		}
	}
}

func TestLoadCert(t *testing.T) {
	ca, err := pki.GenerateCA("test-ca")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ca.crt")
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Certificate.Raw})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCert(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.SerialNumber.Cmp(ca.Certificate.SerialNumber) != 0 {
		t.Fatal("loaded certificate serial does not match")
	}
}

func TestLoadCertRejectsTrailingData(t *testing.T) {
	ca, err := pki.GenerateCA("test-ca")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ca.crt")
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Certificate.Raw})
	data = append(data, []byte("junk")...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCert(path); err == nil {
		t.Fatal("expected trailing data error")
	}
}

func TestLoadTLSCertificate(t *testing.T) {
	ca, err := pki.GenerateCA("test-ca")
	if err != nil {
		t.Fatal(err)
	}
	key, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	csr, err := GenerateCSR(key, "edge-01")
	if err != nil {
		t.Fatal(err)
	}
	certPEM, _, err := ca.IssueCertificate("edge-01", csr, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath := filepath.Join(dir, "connector.crt")
	keyPath := filepath.Join(dir, "connector.key")
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	if err := SavePrivateKey(keyPath, key); err != nil {
		t.Fatal(err)
	}

	pair, err := LoadTLSCertificate(config.Config{Connector: config.Connector{Certificate: certPath, PrivateKey: keyPath}})
	if err != nil {
		t.Fatal(err)
	}
	if pair.Leaf == nil || pair.Leaf.Subject.CommonName != "edge-01" {
		t.Fatal("expected parsed leaf certificate to be populated")
	}
}

func TestLoadTLSCertificateRejectsMismatchedKey(t *testing.T) {
	ca, err := pki.GenerateCA("test-ca")
	if err != nil {
		t.Fatal(err)
	}
	key, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	csr, err := GenerateCSR(key, "edge-01")
	if err != nil {
		t.Fatal(err)
	}
	certPEM, _, err := ca.IssueCertificate("edge-01", csr, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert"), filepath.Join(dir, "key")
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	if err := SavePrivateKey(keyPath, otherKey); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTLSCertificate(config.Config{Connector: config.Connector{Certificate: certPath, PrivateKey: keyPath}}); err == nil {
		t.Fatal("expected mismatched key pair to be rejected")
	}
}

func TestLoadServerRoots(t *testing.T) {
	ca, err := pki.GenerateCA("gateway-ca")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "gateway-ca.crt")
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Certificate.Raw})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	roots, err := LoadServerRoots(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, subject := range roots.Subjects() {
		if bytes.Equal(subject, ca.Certificate.RawSubject) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected custom gateway CA to be trusted")
	}
}

func TestLoadServerRootsRejectsInvalidPEM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad-ca.pem")
	if err := os.WriteFile(path, []byte("not a certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadServerRoots(path); err == nil {
		t.Fatal("expected invalid CA PEM to be rejected")
	}
}

func TestVerifyLocalKeyPair(t *testing.T) {
	key, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{PublicKey: &key.PublicKey}
	if err := VerifyLocalKeyPair(cert, key); err != nil {
		t.Fatalf("matching key pair rejected: %v", err)
	}
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyLocalKeyPair(cert, other); err == nil {
		t.Fatal("expected mismatched key to be rejected")
	}
}

func TestInstallCertificate(t *testing.T) {
	ca, err := pki.GenerateCA("test-ca")
	if err != nil {
		t.Fatal(err)
	}
	key, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	csr, err := GenerateCSR(key, "edge-01")
	if err != nil {
		t.Fatal(err)
	}
	leafPEM, _, err := ca.IssueCertificate("edge-01", csr, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Certificate.Raw})
	bundle := append(append([]byte{}, leafPEM...), caPEM...)

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "connector.key")
	if err := SavePrivateKey(keyPath, key); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Connector: config.Connector{
		ID: "edge-01", PrivateKey: keyPath,
		Certificate: filepath.Join(dir, "certs", "connector.crt"),
		CA:          filepath.Join(dir, "certs", "ca.crt"),
	}}
	if err := InstallCertificate(cfg, "edge-01", bundle); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCert(cfg.Connector.Certificate); err != nil {
		t.Fatalf("installed leaf certificate: %v", err)
	}
	if _, err := LoadCert(cfg.Connector.CA); err != nil {
		t.Fatalf("installed CA certificate: %v", err)
	}
}

func TestInstallCertificateRejectsWrongLocalKey(t *testing.T) {
	ca, err := pki.GenerateCA("test-ca")
	if err != nil {
		t.Fatal(err)
	}
	issuedKey, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	csr, err := GenerateCSR(issuedKey, "edge-01")
	if err != nil {
		t.Fatal(err)
	}
	leafPEM, _, err := ca.IssueCertificate("edge-01", csr, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Certificate.Raw})
	bundle := append(append([]byte{}, leafPEM...), caPEM...)
	wrongKey, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "wrong.key")
	if err := SavePrivateKey(keyPath, wrongKey); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Connector: config.Connector{ID: "edge-01", PrivateKey: keyPath, Certificate: filepath.Join(dir, "cert.pem"), CA: filepath.Join(dir, "ca.pem")}}
	if err := InstallCertificate(cfg, "edge-01", bundle); err == nil {
		t.Fatal("expected certificate/key mismatch to be rejected")
	}
	if _, err := os.Stat(cfg.Connector.Certificate); !os.IsNotExist(err) {
		t.Fatalf("certificate should not be installed, stat err = %v", err)
	}
}

func TestInstallCertificateRejectsWrongConnectorIdentity(t *testing.T) {
	ca, err := pki.GenerateCA("test-ca")
	if err != nil {
		t.Fatal(err)
	}
	key, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	csr, err := GenerateCSR(key, "edge-01")
	if err != nil {
		t.Fatal(err)
	}
	leafPEM, _, err := ca.IssueCertificate("edge-01", csr, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Certificate.Raw})
	bundle := append(append([]byte{}, leafPEM...), caPEM...)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key")
	if err := SavePrivateKey(keyPath, key); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Connector: config.Connector{ID: "edge-02", PrivateKey: keyPath, Certificate: filepath.Join(dir, "cert.pem"), CA: filepath.Join(dir, "ca.pem")}}
	if err := InstallCertificate(cfg, "edge-02", bundle); err == nil {
		t.Fatal("expected connector identity mismatch to be rejected")
	}
}

func TestInstallCertificateRejectsMalformedBundle(t *testing.T) {
	cfg := config.Config{Connector: config.Connector{ID: "edge-01"}}
	for _, input := range [][]byte{nil, []byte("invalid"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("bad")})} {
		if err := InstallCertificate(cfg, "edge-01", input); err == nil {
			t.Fatalf("expected malformed bundle %q to be rejected", input)
		}
	}
}
