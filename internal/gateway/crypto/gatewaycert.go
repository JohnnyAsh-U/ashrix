package crypto

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/filehelper"
)

// GatewayCrypto handles the gateway's mTLS identity and trust anchors.
type GatewayCrypto interface {
	IsBootstrapped() bool
	GetCert() *x509.Certificate
	GetTLSConfig() *tls.Config          // For talking to CP
	GetConnectorTLSConfig() *tls.Config // For listening to connectors
}

type GatewayPKI struct {
	NodeID      string
	Key         *ecdsa.PrivateKey
	Cert        *x509.Certificate
	TrustBundle *x509.CertPool
	BasePath    string
}

func NewGatewayPKI(nodeID, basePath, secret string) (*GatewayPKI, error) {
	keyPath := filepath.Join(basePath, "node.key.enc")
	certPath := filepath.Join(basePath, "node.crt")
	trustPath := filepath.Join(basePath, "trust-bundle.pem")

	exists, _ := filehelper.FileExists(certPath)
	if !exists {
		return &GatewayPKI{NodeID: nodeID, BasePath: basePath}, nil
	}

	// Load Identity
	key, cert, err := pki_utils.LoadKeyAndCert(keyPath, certPath, secret, "gateway")
	if err != nil {
		return nil, fmt.Errorf("failed to load gateway identity: %w", err)
	}

	// Load Trust Bundle
	trustPEM, err := os.ReadFile(trustPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read trust bundle: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(trustPEM) {
		return nil, fmt.Errorf("failed to parse trust bundle")
	}

	return &GatewayPKI{
		NodeID:      nodeID,
		Key:         key,
		Cert:        cert,
		TrustBundle: pool,
		BasePath:    basePath,
	}, nil
}

func (g *GatewayPKI) IsBootstrapped() bool {
	return g.Cert != nil
}

func (g *GatewayPKI) GetCert() *x509.Certificate {
	return g.Cert
}

// GetTLSConfig returns a config for the gateway to connect to the Control Plane.
func (g *GatewayPKI) GetTLSConfig() *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{g.Cert.Raw},
			PrivateKey:  g.Key,
			Leaf:        g.Cert,
		}},
		RootCAs:    g.TrustBundle,
		MinVersion: tls.VersionTLS13,
	}
}

// GetConnectorTLSConfig returns a config for the gateway to accept connector streams.
func (g *GatewayPKI) GetConnectorTLSConfig() *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{g.Cert.Raw},
			PrivateKey:  g.Key,
			Leaf:        g.Cert,
		}},
		ClientCAs:  g.TrustBundle,
		ClientAuth: tls.RequireAndVerifyClientCert,
		MinVersion: tls.VersionTLS13,
	}
}

// SaveIdentity persists the credentials received during bootstrap.
// func (g *GatewayPKI) SaveIdentity(certPEM []byte, key *ecdsa.PrivateKey, trustPEM []byte, secret string) error {
// 	keyPath := filepath.Join(g.BasePath, "node.key.enc")
// 	certPath := filepath.Join(g.BasePath, "node.crt")
// 	trustPath := filepath.Join(g.BasePath, "trust-bundle.pem")

// 	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
// 		return err
// 	}

// 	// In real implementation: encrypt key using pki_utils and write to keyPath
// 	// Write certPEM and trustPEM to respective paths
// 	return nil 
// }
