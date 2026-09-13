package pki

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/JohnnyAsh-U/ashrix-api/pkg/crypto"
)

func GenerateCertificateSigningKey(privateKeyPath, publicKeyPath, secret, context string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("Generating signing key: %w", err)
	}

	if err := encryptAndWriteKey(privateKeyPath, key, secret, context); err != nil {
		return nil, nil, err
	}

	pubKeyDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling public key: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubKeyDER})
	if err := writeFile(publicKeyPath, pubPEM, 0644); err != nil {
		return nil, nil, fmt.Errorf("writing public key: %w", err)
	}

	fmt.Printf("  ✓ Signing key generated\n")
	return key, &key.PublicKey, nil
}

//Generate Ed25519 Private Key and Save it to the file
func GenerateEd25519Key(privateKeyPath string, secret, context string) (ed25519.PrivateKey, error) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("Generating signing key: %w", err)
	}

	if err := encryptAndWriteEd25519Key(privateKeyPath, privateKey, secret, context); err != nil {
		return nil, err
	}

	fmt.Printf("  ✓ Signing key generated\n")
	return privateKey, nil
}

//Load Ed25519 Private key
func LoadEd25519Key(keyPath, secret, context string) (ed25519.PrivateKey, error) {
	key, err := decryptAndLoadEd25519Key(keyPath, secret, context)
	if err != nil {
		return nil, err
	}
	return key, nil
}


func LoadKeyAndCert(keyPath, certPath, secret, context string) (*ecdsa.PrivateKey, *x509.Certificate, error) {
	key, err := decryptAndLoadKey(keyPath, secret, context)
	if err != nil {
		return nil, nil, err
	}
	cert, err := loadCert(certPath)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, nil
}

func LoadKey(keyPath, secret, context string) (*ecdsa.PrivateKey, error) {
	key, err := decryptAndLoadKey(keyPath, secret, context)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func LoadCert(certPath string) (*x509.Certificate, error) {
	cert, err := loadCert(certPath)
	if err != nil {
		return nil, err
	}
	return cert, nil
}

func ParseCSR(csrPEM []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("failed to decode PEM block containing CSR")
	}
	return x509.ParseCertificateRequest(block.Bytes)
}

func ParsePublicKey(pubKey []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(pubKey)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing public key")
	}
	pubKeyDER, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	return pubKeyDER.(*ecdsa.PublicKey), nil
}

// ParsePublicKeyFromCert extracts an ECDSA public key from a PEM-encoded certificate.
func ParsePublicKeyFromCert(certPEM []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("failed to decode PEM block containing certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	ecdsaPubKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("certificate public key is not of type ECDSA")
	}

	return ecdsaPubKey, nil
}

func MarshalCert(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

func MarshalPubKey(pubKey *ecdsa.PublicKey) []byte {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})
}

func MarshalCsr(csr *x509.CertificateRequest) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csr.Raw,
	})
}

func MarshalKey(key *ecdsa.PrivateKey) []byte {
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDER,
	})
}

func WriteCert(path string, cert *x509.Certificate) error {
	return writeFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), 0644)
}

func WriteKey(path string, key *ecdsa.PrivateKey, secret, context string) error {
	return encryptAndWriteKey(path, key, secret, context)
}

func WriteCSR(path string, csrDER []byte) error {
	return writeFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}), 0644)
}

func RandomSerial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}

func VerifyKeyAndCert(cert *x509.Certificate, key *ecdsa.PrivateKey) error {
	//Verify public key
	certPub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("Public Key Invalid")
	}

	//Verify if key and public matches
	if !certPub.Equal(&key.PublicKey) {
		return errors.New("Key and Cert doesn't match")
	}
	return nil
}

func VerifySignature(pubKey *ecdsa.PublicKey, data []byte, signature []byte) bool {
	return ecdsa.VerifyASN1(pubKey, data, signature)
}

// VerifyPossessionProof is a helper that reconstructs the expected hash
// and verifies the base64-encoded signature.
func VerifyPossessionProof(
	pubKey *ecdsa.PublicKey,
	csrPEM []byte,
	connectorID string,
	timestamp int64,
	sigBytes string,
) bool {

	//Decode the sigbytes
	signature, err := base64.StdEncoding.DecodeString(sigBytes)
	if err != nil {
		return false
	}

	// 1. Reconstruct the exact same byte sequence used during signing
	h := sha256.New()
	h.Write(csrPEM)
	h.Write([]byte(connectorID))

	timeBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBytes, uint64(timestamp))
	h.Write(timeBytes)

	digest := h.Sum(nil)

	// 2. Verify the signature against the reconstructed digest
	return VerifySignature(pubKey, digest, signature)
}

// =============================================================================
// ENCRYPTION — AES-256-GCM with HKDF key derivation (stdlib only)
// =============================================================================

func encryptAndWriteKey(path string, key *ecdsa.PrivateKey, secret, context string) error {
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshaling key: %w", err)
	}
	defer wipeBytes(keyDER)

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	defer wipeBytes(keyPEM)

	encrypted, err := crypto.AesgcmEncrypt(keyPEM, secret, context)
	if err != nil {
		return err
	}
	return writeFile(path, encrypted, 0600)
}


func decryptAndLoadKey(path, secret, context string) (*ecdsa.PrivateKey, error) {
	encrypted, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	keyPEM, err := crypto.AesgcmDecrypt(encrypted, secret, context)
	if err != nil {
		return nil, err
	}
	defer wipeBytes(keyPEM)

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, errors.New("failed to decode decrypted key PEM")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func encryptAndWriteEd25519Key(path string, key ed25519.PrivateKey, secret, context string) error {
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshaling Ed25519 private key: %w", err)
	}
	defer wipeBytes(keyDER)

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyDER,
	})
	defer wipeBytes(keyPEM)

	encrypted, err := crypto.AesgcmEncrypt(keyPEM, secret, context)
	if err != nil {
		return err
	}

	return writeFile(path, encrypted, 0600)
}

func decryptAndLoadEd25519Key(path, secret, context string) (ed25519.PrivateKey, error) {
	encrypted, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	keyPEM, err := crypto.AesgcmDecrypt(encrypted, secret, context)
	if err != nil {
		return nil, err
	}
	defer wipeBytes(keyPEM)

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, errors.New("failed to decode decrypted key PEM")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing Ed25519 private key: %w", err)
	}

	edKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("expected Ed25519 private key, got %T", key)
	}

	return edKey, nil
}
// writeFile writes atomically via temp file + rename.
// No partial writes are ever visible to other processes.
func writeFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename to %s: %w", path, err)
	}
	return nil
}

func loadCert(path string) (*x509.Certificate, error) {
	certPEM, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("invalid cert PEM: %s", path)
	}
	return x509.ParseCertificate(block.Bytes)
}

func wipeBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
