package crypto

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/JohnnyAsh-U/ashrix-api/pkg/filehelper"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
)

// =============================================================================
// THE INTERFACE — For signing  bundles from cp to gateways
// =============================================================================

type BundleSigning interface {
	SignBundle(payload []byte) []byte
	SignJWT(payload []byte) []byte
	GetPublicKey() ed25519.PublicKey
	GetPublicKeyString() (string, error)
	GetPublicKeyBase64() (string, error)
}

type BundleSigner struct {
	BundleSigningKey *ed25519.PrivateKey
}

const signingKeyContext = "CPPLATFORM"

func BundleSigningKeys(baseDir, secret string, log *slog.Logger) (BundleSigning, error) {
	if secret == "" {
		return nil, fmt.Errorf("bundle signing key encryption secret is required")
	}
	keyPath := filepath.Join(baseDir, "cp", "bundle-signer.enc")
	if _, err := os.Stat(keyPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("bundle signing key is missing; run 'ashrix-cp setup bundle-key': %w", err)
		}
		return nil, fmt.Errorf("checking bundle signing key: %w", err)
	}
	return loadBundleSigningKey(keyPath, secret, log)
}

func ProvisionBundleSigningKeys(baseDir, secret string, log *slog.Logger) (BundleSigning, error) {
	if secret == "" {
		return nil, fmt.Errorf("bundle signing key encryption secret is required")
	}
	log.Info("Initializing Bundle Signing Keys...")
	CPPKIDir := filepath.Join(baseDir, "cp")

	if err := os.MkdirAll(CPPKIDir, 0700); err != nil {
		return nil, fmt.Errorf("creating PKI directory: %w", err)
	}

	keyPath := filepath.Join(CPPKIDir, "bundle-signer.enc")

	// Provisioning creates the key only when it is absent.
	keyexists, err := filehelper.FileExists(keyPath)
	if err != nil {
		return nil, err
	}

	if !keyexists {
		log.Info("Generating bundle signing key")
		_, err := pki.GenerateEd25519Key(keyPath, secret, signingKeyContext)
		if err != nil {
			return nil, err
		}
	}
	return BundleSigningKeys(baseDir, secret, log)
}

func loadBundleSigningKey(keyPath, secret string, log *slog.Logger) (BundleSigning, error) {
	log.Info("Loading Bundle Signing Key...")
	SigningKey, err := pki.LoadEd25519Key(keyPath, secret, signingKeyContext)
	if err != nil {
		return nil, err
	}

	log.Info("Control Plane PKI Intializing Done...")

	return &BundleSigner{
		BundleSigningKey: &SigningKey,
	}, nil
}

// Return the Public Key in string
func (b *BundleSigner) GetPublicKey() ed25519.PublicKey {
	return b.BundleSigningKey.Public().(ed25519.PublicKey)
}

func (b *BundleSigner) GetPublicKeyString() (string, error) {
	pub := b.BundleSigningKey.Public().(ed25519.PublicKey)

	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("marshal public key: %w", err)
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})

	return string(pubPEM), nil
}

func (b *BundleSigner) GetPublicKeyBase64() (string, error) {
	pubDER, err := x509.MarshalPKIXPublicKey(b.GetPublicKey())
	if err != nil {
		return "", fmt.Errorf("marshal public key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(pubDER), nil
}

//Signing Implementation

func (b *BundleSigner) SignBundle(payload []byte) []byte {
	signature := ed25519.Sign(*b.BundleSigningKey, payload)
	return signature
}

func (b *BundleSigner) SignJWT(payload []byte) []byte {
	return ed25519.Sign(*b.BundleSigningKey, payload)
}
