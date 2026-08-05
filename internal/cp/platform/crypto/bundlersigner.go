package crypto

import (
	// "crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/pkg/filehelper"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
)

// =============================================================================
// THE INTERFACE — For signing  bundles from cp to gateways
// =============================================================================

type BundleSigning interface {
	SignBundle(version string, payload []byte) (*SignedBundle, error)
	VerifyBundle(sb *SignedBundle) error
	GetPublicKey() ed25519.PublicKey
	GetPublicKeyString() (string, error)
}


type Bundle struct {
	Version  string    `json:"version"`
	Payload  []byte    `json:"payload"`
	IssuedAt time.Time `json:"issued_at"`
	Nonce    string    `json:"nonce"`
}

type SignedBundle struct {
	Bundle    Bundle `json:"bundle"`
	Signature string `json:"signature"`
}

type BundleSigner struct {
	BundleSigningKey *ed25519.PrivateKey
}

var (
	signingKeySecret string = "SIGNINGASHRIX"
	signingKeyContext string = "CPPLATFORM"
)


func BundleSigningKeys(baseDir string) (BundleSigning, error) {
	fmt.Println("Initializing Bundle Signing Keys...")
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
		fmt.Println("→ Bundle Signing Key not found")
		fmt.Println("Requesting one...")
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

	fmt.Println("Control Plane PKI Intializing Done...")

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

func (b *BundleSigner) SignBundle(version string, payload []byte) (*SignedBundle, error) {
	if b.BundleSigningKey == nil {
		return nil, fmt.Errorf("bundle signing key not initialized")
	}

	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	bundle := Bundle{
		Version:  version,
		Payload:  payload,
		IssuedAt: time.Now().UTC(),
		Nonce:    base64.StdEncoding.EncodeToString(nonce),
	}

	data, err := json.Marshal(bundle)
	if err != nil {
		return nil, fmt.Errorf("marshal bundle: %w", err)
	}

	signature := ed25519.Sign(*b.BundleSigningKey, data)

	return &SignedBundle{
		Bundle:    bundle,
		Signature: base64.StdEncoding.EncodeToString(signature),
	}, nil
}


func (b *BundleSigner) VerifyBundle(sb *SignedBundle) error {
	if b.BundleSigningKey == nil {
		return fmt.Errorf("bundle signing key not initialized")
	}

	pub := b.BundleSigningKey.Public().(ed25519.PublicKey)

	data, err := json.Marshal(sb.Bundle)
	if err != nil {
		return fmt.Errorf("marshal bundle: %w", err)
	}

	sig, err := base64.StdEncoding.DecodeString(sb.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	if !ed25519.Verify(pub, data, sig) {
		return fmt.Errorf("bundle signature invalid")
	}

	return nil
}

