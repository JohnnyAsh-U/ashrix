package crypto

import (
	"crypto/ed25519"
	"crypto/x509"
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
	SignBundle(payload []byte) ([]byte)
	GetPublicKey() ed25519.PublicKey
	GetPublicKeyString() (string, error)
}


type BundleSigner struct {
	BundleSigningKey *ed25519.PrivateKey
}

var (
	signingKeySecret string = "SIGNINGASHRIX"
	signingKeyContext string = "CPPLATFORM"
)


func BundleSigningKeys(baseDir string, log *slog.Logger) (BundleSigning, error) {
	log.Info("Initializing Bundle Signing Keys...")
	CPPKIDir := filepath.Join(baseDir, "pki", "cp")

	if err := os.MkdirAll(CPPKIDir, 0700); err != nil {
		return nil, fmt.Errorf("creating PKI directory: %w", err)
	}

	keyPath := filepath.Join(CPPKIDir, "bundle-signer.enc")

	//Check if intermediateCAKey and Cert Exists on the filepath
	keyexists, err := filehelper.FileExists(keyPath)
	if err != nil {
		return nil, err
	}

	if !keyexists {
		if keyexists {
			os.Remove(keyPath)
		}
		log.Info("→ Bundle Signing Key not found")
		log.Info("Requesting one...")
		//Generate Bundle Signing Key
		_, err := pki.GenerateEd25519Key(keyPath, signingKeySecret, signingKeyContext)
		if err != nil {
			return nil, err
		}
	}

	//Load the key from storage
	SigningKey, err := pki.LoadEd25519Key(keyPath, signingKeySecret, signingKeyContext)
	if err != nil {
		return nil, err
	}

	log.Info("Control Plane PKI Intializing Done...")

	return &BundleSigner{
		BundleSigningKey: &SigningKey,
	}, nil
}

//Return the Public Key in string
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

//Signing Implementation

func (b *BundleSigner) SignBundle(payload []byte) ([]byte) {
	signature := ed25519.Sign(*b.BundleSigningKey, payload)
	return signature
}

