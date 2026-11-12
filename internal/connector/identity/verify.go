package identity

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
)

func LoadTLSCertificate(cfg config.Config) (tls.Certificate, error) {
	pair, err := tls.LoadX509KeyPair(
		cfg.Connector.Certificate,
		cfg.Connector.PrivateKey,
	)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load connector certificate: %w", err)
	}

	if len(pair.Certificate) == 0 {
		return tls.Certificate{}, errors.New("connector certificate chain is empty")
	}

	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse connector certificate: %w", err)
	}

	pair.Leaf = leaf
	return pair, nil
}

func LoadServerRoots(path string) (*x509.CertPool, error) {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}

	if path == "" {
		return roots, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read gateway server CA: %w", err)
	}

	if ok := roots.AppendCertsFromPEM(data); !ok {
		return nil, errors.New("no valid gateway server CA certificates found")
	}

	return roots, nil
}

func VerifyLocalKeyPair(
	cert *x509.Certificate,
	key *ecdsa.PrivateKey,
) error {
	pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("certificate public key must be ECDSA")
	}

	if pub.X.Cmp(key.PublicKey.X) != 0 ||
		pub.Y.Cmp(key.PublicKey.Y) != 0 {
		return errors.New("certificate and private key do not match")
	}

	return nil
}
