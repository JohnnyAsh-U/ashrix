package pki

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	KeyringService = "ashrix-connector"
	KeyringUser    = "private_key"
)

func SaveKeyToKeyring(key *ecdsa.PrivateKey) error {
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return keyring.Set(KeyringService, KeyringUser, string(keyPEM))
}

func LoadKeyFromKeyring() (*ecdsa.PrivateKey, error) {
	keyPEMStr, err := keyring.Get(KeyringService, KeyringUser)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode([]byte(keyPEMStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode key PEM from keyring")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func DeleteKeyFromKeyring() error {
	return keyring.Delete(KeyringService, KeyringUser)
}

func LoadKeyAndCertFromKeyring(certPath string) (*ecdsa.PrivateKey, *x509.Certificate, error) {
	key, err := LoadKeyFromKeyring()
	if err != nil {
		return nil, nil, err
	}
	cert, err := LoadCert(certPath)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, nil
}
